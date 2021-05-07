// Copyright 2019 Chaos Mesh Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	authorizationv1 "k8s.io/client-go/kubernetes/typed/authorization/v1"

	"github.com/go-logr/logr"
	"go.uber.org/fx"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/chaos-mesh/chaos-mesh/cmd/chaos-controller-manager/provider"
	"github.com/chaos-mesh/chaos-mesh/controllers"
	"github.com/chaos-mesh/chaos-mesh/controllers/types"
	"github.com/chaos-mesh/chaos-mesh/pkg/selector"

	"golang.org/x/time/rate"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/chaos-mesh/chaos-mesh/controllers/metrics"
	"github.com/chaos-mesh/chaos-mesh/pkg/webhook/config"
	"github.com/chaos-mesh/chaos-mesh/pkg/webhook/config/watcher"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	ccfg "github.com/chaos-mesh/chaos-mesh/controllers/config"
	"github.com/chaos-mesh/chaos-mesh/controllers/podiochaos"
	grpcUtils "github.com/chaos-mesh/chaos-mesh/pkg/grpc"
	"github.com/chaos-mesh/chaos-mesh/pkg/version"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	controllermetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	apiWebhook "github.com/chaos-mesh/chaos-mesh/api/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	printVersion bool
	setupLog     = ctrl.Log.WithName("setup")
)

func parseFlags() {
	flag.BoolVar(&printVersion, "version", false, "print version information and exit")
	flag.Parse()
}

func main() {
	parseFlags()
	version.PrintVersionInfo("Controller manager")
	if printVersion {
		os.Exit(0)
	}

	// set RPCTimeout config
	grpcUtils.RPCTimeout = ccfg.ControllerCfg.RPCTimeout
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	app := fx.New(
		fx.Options(
			fx.Provide(
				provider.NewOption,
				provider.NewClient,
				provider.NewManager,
				provider.NewReader,
				provider.NewLogger,
				provider.NewAuthCli,
				provider.NewScheme,
			),
			controllers.Module,
			selector.Module,
			types.ChaosObjects,
		),
		fx.Invoke(Run),
	)

	app.Run()
}

type RunParams struct {
	fx.In

	Mgr     ctrl.Manager
	Client  client.Client
	Reader  client.Reader
	Logger  logr.Logger
	AuthCli *authorizationv1.AuthorizationV1Client

	Controllers []types.Controller `group:"controller"`
	Objs        []types.Object     `group:"objs"`
}

func Run(params RunParams) error {
	mgr := params.Mgr
	logger := params.Logger
	client := params.Client
	authCli := params.AuthCli

	var err error
	for _, obj := range params.Objs {
		err = ctrl.NewWebhookManagedBy(mgr).
			For(obj.Object).
			Complete()
		if err != nil {
			return err
		}
	}

	// We only setup webhook for podiochaos, and the logic of applying chaos are in the mutation
	// webhook, because we need to get the running result synchronously in io chaos reconciler
	v1alpha1.RegisterPodIoHandler(&podiochaos.Handler{
		Client: client,
		Log:    logger.WithName("handler").WithName("PodIOChaos"),
	})
	err = (&v1alpha1.PodIoChaos{}).SetupWebhookWithManager(mgr)
	if err != nil {
		return err
	}

	// Init metrics collector
	metricsCollector := metrics.NewChaosCollector(mgr.GetCache(), controllermetrics.Registry)

	setupLog.Info("Setting up webhook server")
	hookServer := mgr.GetWebhookServer()
	hookServer.CertDir = ccfg.ControllerCfg.CertsDir
	conf := config.NewConfigWatcherConf()

	stopCh := ctrl.SetupSignalHandler()

	if ccfg.ControllerCfg.PprofAddr != "0" {
		go func() {
			if err := http.ListenAndServe(ccfg.ControllerCfg.PprofAddr, nil); err != nil {
				setupLog.Error(err, "unable to start pprof server")
				os.Exit(1)
			}
		}()
	}

	if err = ccfg.ControllerCfg.WatcherConfig.Verify(); err != nil {
		setupLog.Error(err, "invalid environment configuration")
		os.Exit(1)
	}
	configWatcher, err := watcher.New(*ccfg.ControllerCfg.WatcherConfig, metricsCollector)
	if err != nil {
		setupLog.Error(err, "unable to create config watcher")
		os.Exit(1)
	}

	go watchConfig(configWatcher, conf, stopCh)
	hookServer.Register("/inject-v1-pod", &webhook.Admission{
		Handler: &apiWebhook.PodInjector{
			Config:        conf,
			ControllerCfg: ccfg.ControllerCfg,
			Metrics:       metricsCollector,
		}},
	)
	hookServer.Register("/validate-auth", &webhook.Admission{
		Handler: apiWebhook.NewAuthValidator(ccfg.ControllerCfg.SecurityMode, mgr.GetClient(), mgr.GetAPIReader(), authCli,
			ccfg.ControllerCfg.ClusterScoped, ccfg.ControllerCfg.TargetNamespace, ccfg.ControllerCfg.EnableFilterNamespace),
	},
	)

	setupLog.Info("Starting manager")
	if err := mgr.Start(stopCh); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	return nil
}

func setupWatchQueue(stopCh <-chan struct{}, configWatcher *watcher.K8sConfigMapWatcher) workqueue.Interface {
	// watch for reconciliation signals, and grab configmaps, then update the running configuration
	// for the server
	sigChan := make(chan interface{}, 10)

	queue := workqueue.NewRateLimitingQueue(&workqueue.BucketRateLimiter{Limiter: rate.NewLimiter(rate.Limit(0.5), 1)})

	go func() {
		for {
			select {
			case <-stopCh:
				queue.ShutDown()
				return
			case <-sigChan:
				queue.AddRateLimited(struct{}{})
			}
		}
	}()

	go func() {
		for {
			setupLog.Info("Launching watcher for ConfigMaps")
			if err := configWatcher.Watch(sigChan, stopCh); err != nil {
				switch err {
				case watcher.ErrWatchChannelClosed:
					// known issue: https://github.com/kubernetes/client-go/issues/334
					setupLog.Info("watcher channel has closed, restart watcher")
				default:
					setupLog.Error(err, "unable to watch new ConfigMaps")
					os.Exit(1)
				}
			}

			select {
			case <-stopCh:
				close(sigChan)
				return
			default:
				// sleep 2 seconds to prevent excessive log due to infinite restart
				time.Sleep(2 * time.Second)
			}
		}
	}()

	return queue
}

func watchConfig(configWatcher *watcher.K8sConfigMapWatcher, cfg *config.Config, stopCh <-chan struct{}) {
	queue := setupWatchQueue(stopCh, configWatcher)

	for {
		item, shutdown := queue.Get()
		if shutdown {
			break
		}
		func() {
			defer queue.Done(item)

			setupLog.Info("Triggering ConfigMap reconciliation")
			updatedInjectionConfigs, err := configWatcher.GetInjectionConfigs()
			if err != nil {
				setupLog.Error(err, "unable to get ConfigMaps")
				return
			}

			setupLog.Info("Updating server with newly loaded configurations",
				"original configs count", len(cfg.Injections), "updated configs count", len(updatedInjectionConfigs))
			cfg.ReplaceInjectionConfigs(updatedInjectionConfigs)
			setupLog.Info("Configuration replaced")
		}()
	}
}
