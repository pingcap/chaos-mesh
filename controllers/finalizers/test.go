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

package finalizers

import (
	"context"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

func TestAddDefaultFinalizer(k8sClient client.Client) {
	key := types.NamespacedName{
		Name:      "foo3",
		Namespace: "default",
	}
	duration := "1000s"
	chaos := &v1alpha1.TimeChaos{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo3",
			Namespace: "default",
		},
		Spec: v1alpha1.TimeChaosSpec{
			TimeOffset: "100ms",
			ClockIds:   []string{"CLOCK_REALTIME"},
			Duration:   &duration,
			ContainerSelector: v1alpha1.ContainerSelector{
				PodSelector: v1alpha1.PodSelector{
					Mode: v1alpha1.OnePodMode,
				},
			},
		},
	}

	ginkgo.By("creating a chaos")
	{
		gomega.Expect(k8sClient.Create(context.TODO(), chaos)).To(gomega.Succeed())
	}

	ginkgo.By("Adding finalizers")
	{
		err := wait.Poll(time.Second*1, time.Second*10, func() (ok bool, err error) {
			err = k8sClient.Get(context.TODO(), key, chaos)
			if err != nil {
				return false, err
			}
			return len(chaos.GetObjectMeta().GetFinalizers()) > 0 && chaos.GetObjectMeta().GetFinalizers()[0] == RecordFinalizer, nil
		})
		gomega.Expect(err).ToNot(gomega.HaveOccurred())
	}

	ginkgo.By("deleting the created object")
	{
		gomega.Expect(k8sClient.Delete(context.TODO(), chaos)).To(gomega.Succeed())
	}
}
