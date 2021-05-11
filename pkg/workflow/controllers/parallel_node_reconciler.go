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

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
)

// ParallelNodeReconciler watches on nodes which type is Parallel
type ParallelNodeReconciler struct {
	*ChildrenNodesFetcher
	kubeClient    client.Client
	eventRecorder record.EventRecorder
	logger        logr.Logger
}

func NewParallelNodeReconciler(kubeClient client.Client, eventRecorder record.EventRecorder, logger logr.Logger) *ParallelNodeReconciler {
	return &ParallelNodeReconciler{
		ChildrenNodesFetcher: NewChildrenNodesFetcher(kubeClient, logger),
		kubeClient:           kubeClient,
		eventRecorder:        eventRecorder,
		logger:               logger,
	}
}

// Reconcile is extremely like the one in SerialNodeReconciler, only allows the parallel schedule
func (it *ParallelNodeReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	defer func() {
		it.logger.V(4).Info("Finished syncing for parallel node",
			"node", request.NamespacedName,
			"duration", time.Since(startTime),
		)
	}()

	ctx := context.TODO()

	node := v1alpha1.WorkflowNode{}
	err := it.kubeClient.Get(ctx, request.NamespacedName, &node)
	if err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// only resolve parallel nodes
	if node.Spec.Type != v1alpha1.TypeParallel {
		return reconcile.Result{}, nil
	}

	it.logger.V(4).Info("resolve parallel node", "node", request)

	// make effects, create/remove children nodes
	err = it.syncChildrenNodes(ctx, node)
	if err != nil {
		return reconcile.Result{}, err
	}

	// update status
	updateError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		nodeNeedUpdate := v1alpha1.WorkflowNode{}
		err := it.kubeClient.Get(ctx, request.NamespacedName, &nodeNeedUpdate)
		if err != nil {
			return err
		}

		activeChildren, finishedChildren, err := it.fetchChildrenNodes(ctx, nodeNeedUpdate)
		if err != nil {
			return err
		}

		nodeNeedUpdate.Status.FinishedChildren = nil
		for _, finishedChild := range finishedChildren {
			nodeNeedUpdate.Status.FinishedChildren = append(nodeNeedUpdate.Status.FinishedChildren,
				corev1.LocalObjectReference{
					Name: finishedChild.Name,
				})
		}

		nodeNeedUpdate.Status.ActiveChildren = nil
		for _, activeChild := range activeChildren {
			nodeNeedUpdate.Status.ActiveChildren = append(nodeNeedUpdate.Status.ActiveChildren,
				corev1.LocalObjectReference{
					Name: activeChild.Name,
				})
		}

		// TODO: also check the consistent between spec in task and the spec in child node
		if len(finishedChildren) == len(nodeNeedUpdate.Spec.Tasks) {
			SetCondition(&nodeNeedUpdate.Status, v1alpha1.WorkflowNodeCondition{
				Type:   v1alpha1.ConditionAccomplished,
				Status: corev1.ConditionTrue,
				Reason: "",
			})
		} else {
			SetCondition(&nodeNeedUpdate.Status, v1alpha1.WorkflowNodeCondition{
				Type:   v1alpha1.ConditionAccomplished,
				Status: corev1.ConditionFalse,
				Reason: "",
			})
		}

		return it.kubeClient.Status().Update(ctx, &nodeNeedUpdate)
	})

	if updateError != nil {
		it.logger.Error(err, "failed to update the status of node", "node", request)
		return reconcile.Result{}, updateError
	}

	return reconcile.Result{}, nil
}

func (it *ParallelNodeReconciler) syncChildrenNodes(ctx context.Context, node v1alpha1.WorkflowNode) error {

	// empty parallel node
	if len(node.Spec.Tasks) == 0 {
		it.logger.V(4).Info("empty parallel node, NOOP",
			"node", fmt.Sprintf("%s/%s", node.Namespace, node.Name),
		)
		return nil
	}

	activeChildrenNodes, finishedChildrenNodes, err := it.fetchChildrenNodes(ctx, node)
	if err != nil {
		return err
	}
	existsChildrenNodes := append(activeChildrenNodes, finishedChildrenNodes...)
	var prefixNamesOfNodes []string

	for _, childNode := range existsChildrenNodes {
		prefixNamesOfNodes = append(prefixNamesOfNodes, getTaskNameFromGeneratedName(childNode.GetName()))
	}

	var tasksToStartup []string

	if len(relativeComplementSet(prefixNamesOfNodes, node.Spec.Tasks)) > 0 {
		// TODO: check the specific of task and workflow nodes
		// the definition of Spec.Tasks changed, remove all the existed nodes
		tasksToStartup = node.Spec.Tasks
		for _, childNode := range existsChildrenNodes {
			err := it.kubeClient.Delete(ctx, &childNode)
			if err != nil {
				it.logger.Error(err, "failed to delete outdated child node",
					"node", fmt.Sprintf("%s/%s", node.Namespace, node.Name),
					"child node", fmt.Sprintf("%s/%s", childNode.Namespace, childNode.Name),
				)
			}
		}
	} else {
		tasksToStartup = relativeComplementSet(node.Spec.Tasks, prefixNamesOfNodes)
	}

	if len(tasksToStartup) == 0 {
		it.logger.Info("no need to spawn new child node", "node", fmt.Sprintf("%s/%s", node.Namespace, node.Name))
		return nil
	}

	parentWorkflow := v1alpha1.Workflow{}
	err = it.kubeClient.Get(ctx, types.NamespacedName{
		Namespace: node.Namespace,
		Name:      node.Spec.WorkflowName,
	}, &parentWorkflow)
	if err != nil {
		it.logger.Error(err, "failed to fetch parent workflow",
			"node", fmt.Sprintf("%s/%s", node.Namespace, node.Name),
			"workflow name", node.Spec.WorkflowName)
		return err
	}
	// TODO: using ordered id instead of random suffix is better, like StatefulSet, also related to the sorting
	childrenNodes, err := renderNodesByTemplates(&parentWorkflow, &node, tasksToStartup...)
	if err != nil {
		it.logger.Error(err, "failed to render children childrenNodes",
			"node", fmt.Sprintf("%s/%s", node.Namespace, node.Name))
		return err
	}

	// TODO: emit event
	var childrenNames []string
	for _, childNode := range childrenNodes {
		err := it.kubeClient.Create(ctx, childNode)
		if err != nil {
			it.logger.Error(err, "failed to create child node",
				"node", fmt.Sprintf("%s/%s", node.Namespace, node.Name),
				"child node", childNode)
			return err
		}
		childrenNames = append(childrenNames, childNode.Name)
	}
	it.logger.Info("serial node spawn new child node",
		"node", fmt.Sprintf("%s/%s", node.Namespace, node.Name),
		"child node", childrenNames)

	return nil
}

func getTaskNameFromGeneratedName(generatedNodeName string) string {
	index := strings.LastIndex(generatedNodeName, "-")
	if index < 0 {
		return generatedNodeName
	}
	return generatedNodeName[:index]
}

// relativeComplementSet return the set of elements which contained in former but not in latter
func relativeComplementSet(former []string, latter []string) []string {
	var result []string
	formerSet := make(map[string]struct{})
	latterSet := make(map[string]struct{})

	for _, item := range former {
		formerSet[item] = struct{}{}
	}
	for _, item := range latter {
		latterSet[item] = struct{}{}
	}
	for k := range formerSet {
		if _, ok := latterSet[k]; !ok {
			result = append(result, k)
		}
	}
	return result
}
