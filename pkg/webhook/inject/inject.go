// Copyright 2019 PingCAP, Inc.
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

package inject

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pingcap/chaos-operator/pkg/utils"
	"github.com/pingcap/chaos-operator/pkg/webhook/config"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var log = ctrl.Log.WithName("inject-webhook")

var ignoredNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
}

const (
	// StatusInjected is the annotation value for /status that indicates an injection was already performed on this pod
	StatusInjected = "injected"
)

func Inject(res *v1beta1.AdmissionRequest, cli client.Client, cfg *config.Config) *v1beta1.AdmissionResponse {
	var pod corev1.Pod
	if err := json.Unmarshal(res.Object.Raw, &pod); err != nil {
		log.Error(err, "Could not unmarshal raw object")
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	// Deal with potential empty fields, e.g., when the pod is created by a deployment
	podName := potentialPodName(&pod.ObjectMeta)
	if pod.ObjectMeta.Namespace == "" {
		pod.ObjectMeta.Namespace = res.Namespace
	}

	log.Info("AdmissionReview for",
		"Kind", res.Kind, "Namespace", res.Namespace, "Name", res.Name, "podName", podName, "UID", res.UID, "patchOperation", res.Operation, "UserInfo", res.UserInfo)
	log.V(4).Info("Object", "Object", string(res.Object.Raw))
	log.V(4).Info("OldObject", "OldObject", string(res.OldObject.Raw))
	log.V(4).Info("Pod", "Pod", pod)

	requiredKey, ok := injectRequired(&pod.ObjectMeta, cli, cfg)
	if !ok {
		log.Info("Skipping injection due to policy check", "namespace", pod.ObjectMeta.Namespace, "name", podName)
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	injectionConfig, err := cfg.GetRequestedConfig(requiredKey)
	if err != nil {
		log.Error(err, "Error getting injection config, permitting launch of pod with no sidecar injected", "injectionConfig",
			injectionConfig)
		// dont prevent pods from launching! just return allowed
		return &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	annotations := map[string]string{cfg.StatusAnnotationKey(): StatusInjected}

	patchBytes, err := createPatch(&pod, injectionConfig, annotations)
	if err != nil {
		return &v1beta1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	log.Info("AdmissionResponse: patch", "patchBytes", string(patchBytes))
	return &v1beta1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *v1beta1.PatchType {
			pt := v1beta1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}

// Check whether the target resource need to be injected and return the required config name
func injectRequired(metadata *metav1.ObjectMeta, cli client.Client, cfg *config.Config) (string, bool) {
	// skip special kubernetes system namespaces
	for _, namespace := range ignoredNamespaces {
		if metadata.Namespace == namespace {
			log.Info("Skip mutation for it' in special namespace", "name", metadata.Name, "namespace", metadata.Namespace)
			return "", false
		}
	}

	log.V(4).Info("meta", "meta", metadata)

	if checkInjectStatus(metadata, cfg) {
		log.Info("Pod annotation indicates injection already satisfied, skipping",
			"namespace", metadata.Namespace, "name", metadata.Name,
			"annotationKey", cfg.StatusAnnotationKey(), "value", StatusInjected)
		return "", false
	}

	requiredConfig, ok := injectByPodRequired(metadata, cfg)
	if ok {
		log.Info("Pod annotation requesting sidecar config",
			"namespace", metadata.Namespace, "name", metadata.Name,
			"annotation", cfg.RequestAnnotationKey(), "requiredConfig", requiredConfig)
		return requiredConfig, true
	}

	requiredConfig, ok = injectByNamespaceRequired(metadata, cli, cfg)
	if ok {
		log.Info("Pod annotation requesting sidecar config",
			"namespace", metadata.Namespace, "name", metadata.Name,
			"annotation", cfg.RequestAnnotationKey(), "requiredConfig", requiredConfig)
		return requiredConfig, true
	}

	return "", false
}

func checkInjectStatus(metadata *metav1.ObjectMeta, cfg *config.Config) bool {
	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	status, ok := annotations[cfg.StatusAnnotationKey()]
	if ok && strings.ToLower(status) == StatusInjected {
		return true
	}

	return false
}

func injectByNamespaceRequired(metadata *metav1.ObjectMeta, cli client.Client, cfg *config.Config) (string, bool) {
	var ns corev1.Namespace
	if err := cli.Get(context.Background(), types.NamespacedName{Name: metadata.Namespace}, &ns); err != nil {
		log.Error(err, "failed to get namespace", "namespace", metadata.Namespace)
		return "", false
	}

	annotations := ns.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	required, ok := annotations[utils.GenAnnotationKeyForWebhook(cfg.RequestAnnotationKey(), metadata.Name)]
	if ok {
		log.Info("Get sidecar config from namespace annotations",
			"namespace", metadata.Namespace, "pod", metadata.Name, "config", required)
		return strings.ToLower(required), true
	}

	return "", false
}

func injectByPodRequired(metadata *metav1.ObjectMeta, cfg *config.Config) (string, bool) {
	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	required, ok := annotations[cfg.RequestAnnotationKey()]
	if !ok {
		log.Info("Pod annotation is missing, skipping injection",
			"namespace", metadata.Namespace, "name", metadata.Name, "annotation", cfg.RequestAnnotationKey())
		return "", false
	}

	log.Info("Get sidecar config from pod annotations",
		"namespace", metadata.Namespace, "pod", metadata.Name, "config", required)
	return strings.ToLower(required), true
}

// create mutation patch for resource
func createPatch(pod *corev1.Pod, inj *config.InjectionConfig, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation

	// make sure any injected containers in our config get the EnvVars and VolumeMounts injected
	// this mutates inj.Containers with our environment vars
	mutatedInjectedContainers := mergeEnvVars(inj.Environment, inj.Containers)
	mutatedInjectedContainers = mergeVolumeMounts(inj.VolumeMounts, mutatedInjectedContainers)

	// make sure any injected init containers in our config get the EnvVars and VolumeMounts injected
	// this mutates inj.InitContainers with our environment vars
	mutatedInjectedInitContainers := mergeEnvVars(inj.Environment, inj.InitContainers)
	mutatedInjectedInitContainers = mergeVolumeMounts(inj.VolumeMounts, mutatedInjectedInitContainers)

	// patch containers with our injected containers
	patch = append(patch, addContainers(pod.Spec.Containers, mutatedInjectedContainers, "/spec/containers")...)

	// patch all existing containers with the env vars and volume mounts
	patch = append(patch, setEnvironment(pod.Spec.Containers, inj.Environment)...)
	patch = append(patch, addVolumeMounts(pod.Spec.Containers, inj.VolumeMounts)...)

	// add initContainers, hostAliases and volumes
	patch = append(patch, addContainers(pod.Spec.InitContainers, mutatedInjectedInitContainers, "/spec/initContainers")...)
	patch = append(patch, addHostAliases(pod.Spec.HostAliases, inj.HostAliases, "/spec/hostAliases")...)
	patch = append(patch, addVolumes(pod.Spec.Volumes, inj.Volumes, "/spec/volumes")...)

	// set commands and args
	patch = append(patch, setCommands(pod.Spec.Containers, inj.PostStart)...)

	// set annotations
	patch = append(patch, updateAnnotations(pod.Annotations, annotations)...)

	// set shareProcessNamespace
	patch = append(patch, updateShareProcessNamespace(inj.ShareProcessNamespace)...)

	return json.Marshal(patch)
}

func setCommands(target []corev1.Container, postStart map[string]config.ExecAction) (patch []patchOperation) {
	if postStart == nil {
		return
	}

	for containerIndex, container := range target {
		execCmd, ok := postStart[container.Name]
		if !ok {
			continue
		}

		path := fmt.Sprintf("/spec/containers/%d/command", containerIndex)
		var value interface{}
		value = []string{"/bin/sh"}
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  path,
			Value: value,
		})

		var argsValue interface{}
		argsPath := fmt.Sprintf("/spec/containers/%d/args", containerIndex)
		args := execCmd.Command

		args = append(args, container.Command...)
		args = append(args, container.Args...)
		argsValue = args
		patch = append(patch, patchOperation{
			Op:    "replace",
			Path:  argsPath,
			Value: argsValue,
		})
	}

	return patch
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func setEnvironment(target []corev1.Container, addedEnv []corev1.EnvVar) (patch []patchOperation) {
	var value interface{}
	for containerIndex, container := range target {
		// for each container in the spec, determine if we want to patch with any env vars
		first := len(container.Env) == 0
		for _, add := range addedEnv {
			path := fmt.Sprintf("/spec/containers/%d/env", containerIndex)
			hasKey := false
			// make sure we dont override any existing env vars; we only add, dont replace
			for _, origEnv := range container.Env {
				if origEnv.Name == add.Name {
					hasKey = true
					break
				}
			}
			if !hasKey {
				// make a patch
				value = add
				if first {
					first = false
					value = []corev1.EnvVar{add}
				} else {
					path = path + "/-"
				}
				patch = append(patch, patchOperation{
					Op:    "add",
					Path:  path,
					Value: value,
				})
			}
		}
	}

	return patch
}

func addContainers(target, added []corev1.Container, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		log.V(6).Info("add container", "add", add)
		path := basePath
		if first {
			first = false
			value = []corev1.Container{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addVolumes(target, added []corev1.Volume, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.Volume{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

func addVolumeMounts(target []corev1.Container, addedVolumeMounts []corev1.VolumeMount) (patch []patchOperation) {
	var value interface{}
	for containerIndex, container := range target {
		// for each container in the spec, determine if we want to patch with any volume mounts
		first := len(container.VolumeMounts) == 0
		for _, add := range addedVolumeMounts {
			path := fmt.Sprintf("/spec/containers/%d/volumeMounts", containerIndex)
			hasKey := false
			for _, origVolumeMount := range container.VolumeMounts {
				if origVolumeMount.Name == add.Name {
					hasKey = true
					break
				}
			}
			value = add
			if first {
				first = false
				value = []corev1.VolumeMount{add}
			} else {
				path = path + "/-"
			}

			if hasKey {
				patch = append(patch, patchOperation{
					Op:    "replace",
					Path:  path,
					Value: value,
				})
				continue
			}

			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  path,
				Value: value,
			})
		}
	}
	return patch
}

func addHostAliases(target, added []corev1.HostAlias, basePath string) (patch []patchOperation) {
	first := len(target) == 0
	var value interface{}
	for _, add := range added {
		value = add
		path := basePath
		if first {
			first = false
			value = []corev1.HostAlias{add}
		} else {
			path = path + "/-"
		}
		patch = append(patch, patchOperation{
			Op:    "add",
			Path:  path,
			Value: value,
		})
	}
	return patch
}

// for containers, add any env vars that are not already defined in the Env list.
// this does _not_ return patches; this is intended to be used only on containers defined
// in the injection config, so the resources do not exist yet in the k8s api (thus no patch needed)
func mergeEnvVars(envs []corev1.EnvVar, containers []corev1.Container) []corev1.Container {
	mutatedContainers := []corev1.Container{}
	for _, c := range containers {
		for _, newEnv := range envs {
			// check each container for each env var by name.
			// if the container has a matching name, dont override!
			skip := false
			for _, origEnv := range c.Env {
				if origEnv.Name == newEnv.Name {
					skip = true
					break
				}
			}
			if !skip {
				c.Env = append(c.Env, newEnv)
			}
		}
		mutatedContainers = append(mutatedContainers, c)
	}
	return mutatedContainers
}

func mergeVolumeMounts(volumeMounts []corev1.VolumeMount, containers []corev1.Container) []corev1.Container {
	mutatedContainers := []corev1.Container{}
	for _, c := range containers {
		for _, newVolumeMount := range volumeMounts {
			// check each container for each volume mount by name.
			// if the container has a matching name, dont override!
			skip := false
			for _, origVolumeMount := range c.VolumeMounts {
				if origVolumeMount.Name == newVolumeMount.Name {
					skip = true
					break
				}
			}
			if !skip {
				c.VolumeMounts = append(c.VolumeMounts, newVolumeMount)
			}
		}
		mutatedContainers = append(mutatedContainers, c)
	}
	return mutatedContainers
}

func updateAnnotations(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		keyEscaped := strings.Replace(key, "/", "~1", -1)

		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:    "add",
				Path:  "/metadata/annotations/" + keyEscaped,
				Value: value,
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + keyEscaped,
				Value: value,
			})
		}
	}
	return patch
}

func updateShareProcessNamespace(value bool) (patch []patchOperation) {
	op := "add"
	patch = append(patch, patchOperation{
		Op:    op,
		Path:  "/spec/shareProcessNamespace",
		Value: value,
	})
	return patch
}

func potentialPodName(metadata *metav1.ObjectMeta) string {
	if metadata.Name != "" {
		return metadata.Name
	}
	if metadata.GenerateName != "" {
		return metadata.GenerateName + "***** (actual name not yet known)"
	}
	return ""
}
