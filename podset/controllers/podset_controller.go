/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
	batchv1 "tutorial.operatorsdk.io/podset/api/v1"
)

var log = logf.Log.WithName("controller_podset")

// PodSetReconciler reconciles a PodSet object
type PodSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.app.example.com,resources=podsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.app.example.com,resources=podsets/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.app.example.com,resources=podsets/finalizers,verbs=update

// Reconcile reads that state of the cluster for a PodSet object and makes changes based on the state read
// and what is in the PodSet.Spec
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.

func (r *PodSetReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PodSet")

	// 获取当前 PodSet
	podSet := batchv1.PodSet{}
	err := r.Get(ctx, request.NamespacedName, &podSet)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// 获取当前 Pod 状态，并和实预期的状态做对比
	existingPods := &corev1.PodList{}
	podLabels := labels.Set{
		"app":     podSet.Name,
		"version": "v1.0",
	}
	err = r.List(ctx, existingPods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(podLabels),
		Namespace:     podSet.Namespace,
	})
	if err != nil {
		reqLogger.Error(err, "error get pods : ",
			"request.Namespace", request.Namespace, "matchingFields", podLabels)

		return reconcile.Result{}, err
	}
	var existingPodNames []string
	for _, pod := range existingPods.Items {
		if pod.GetObjectMeta().GetDeletionTimestamp() != nil {
			continue
		}
		if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodRunning {
			existingPodNames = append(existingPodNames, pod.Name)
		}
	}
	status := batchv1.PodSetStatus{
		Replicas: int32(len(existingPodNames)),
		PodNames: existingPodNames,
	}
	if !reflect.DeepEqual(podSet.Status, status) {
		podSet.Status = status
		err := r.Client.Status().Update(context.TODO(), &podSet)
		if err != nil {
			reqLogger.Error(err, "failed to update the podSet")
			return reconcile.Result{}, err
		}
	}

	// 缩容
	if status.Replicas > podSet.Spec.Replicas {
		reqLogger.Info("delete a pod in the podset", "expected replicas", podSet.Spec.Replicas, "Pod.Names", existingPodNames)
		pod := existingPods.Items[0]
		err = r.Delete(ctx, &pod)
		if err != nil {
			reqLogger.Error(err, "error delete pods!")
			return reconcile.Result{}, err
		}
	}

	// 扩容
	if status.Replicas < podSet.Spec.Replicas {
		reqLogger.Info("Adding a pod in the podset", "expected replicas", podSet.Spec.Replicas, "Pod.Names", existingPodNames)
		pod := newPodForCR(&podSet)
		if err := controllerutil.SetControllerReference(&podSet, pod, r.Scheme); err != nil {
			reqLogger.Error(err, "unable to set owner reference on new pod")
			return reconcile.Result{}, err
		}
		err = r.Create(ctx, pod)
		if err != nil {
			reqLogger.Error(err, "error create pods!")
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{Requeue: true}, err
}

func newPodForCR(ps *batchv1.PodSet) *corev1.Pod {
	podLabels := map[string]string{
		"app":     ps.Name,
		"version": "v1.0",
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:      ps.Name + "-pod",
			Namespace:         ps.Namespace,
			CreationTimestamp: metav1.NewTime(time.Now()),
			Labels:            podLabels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "busybox",
					Image: "busybox",
					Command: []string{
						"sleep", "3600",
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.PodSet{}).
		Owns(&corev1.Pod{}).
		Complete(r)
}
