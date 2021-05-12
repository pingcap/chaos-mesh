// Copyright 2021 Chaos Mesh Authors.
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

package pause

import (
	"context"
	"reflect"
	"strconv"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/chaos-mesh/chaos-mesh/controllers/schedule/utils"
	"github.com/chaos-mesh/chaos-mesh/controllers/types"
	"github.com/go-logr/logr"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	k8sTypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Reconciler struct {
	client.Client
	Log          logr.Logger
	ActiveLister *utils.ActiveLister

	Recorder record.EventRecorder
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()

	schedule := &v1alpha1.Schedule{}
	err := r.Get(ctx, req.NamespacedName, schedule)
	if err != nil {
		if !k8sError.IsNotFound(err) {
			r.Log.Error(err, "unable to get chaos")
		}
		return ctrl.Result{}, nil
	}

	list, err := r.ActiveLister.ListActiveJobs(ctx, schedule)
	if err != nil {
		r.Recorder.Eventf(schedule, "Warning", "Failed", "Failed to list active jobs: %s", err.Error())
		return ctrl.Result{}, nil
	}

	items := reflect.ValueOf(list).Elem().FieldByName("Items")
	for i := 0; i < items.Len(); i++ {
		item := items.Index(i).Addr().Interface().(v1alpha1.InnerObject)
		if item.IsPaused() != schedule.IsPaused() {
			key := k8sTypes.NamespacedName{
				Namespace: item.GetObjectMeta().GetNamespace(),
				Name:      item.GetObjectMeta().GetName(),
			}
			pause := strconv.FormatBool(schedule.IsPaused())

			updateError := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				r.Log.Info("updating object", "pause", schedule.IsPaused())

				if err := r.Client.Get(ctx, key, item); err != nil {
					r.Log.Error(err, "unable to get schedule")
					return err
				}
				if item.GetObjectMeta().Annotations == nil {
					item.GetObjectMeta().Annotations = make(map[string]string)
				}
				item.GetObjectMeta().Annotations[v1alpha1.PauseAnnotationKey] = pause

				return r.Client.Update(ctx, item)
			})
			if updateError != nil {
				r.Log.Error(updateError, "fail to update")
				r.Recorder.Eventf(schedule, "Warning", "Failed", "Failed to set pause to %s for %s", pause, key)
				return ctrl.Result{}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}

func NewController(mgr ctrl.Manager, client client.Client, log logr.Logger, lister *utils.ActiveLister) (types.Controller, error) {
	ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Schedule{}).
		Named("schedule-pause").
		Complete(&Reconciler{
			client,
			log.WithName("schedule-pause"),
			lister,
			mgr.GetEventRecorderFor("schedule-pause"),
		})
	return "schedule-pause", nil
}
