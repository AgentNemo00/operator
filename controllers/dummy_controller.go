/*
Copyright 2023.

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
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	interviewcomv1alpha1 "github.com/AgentNemo00/operator/api/v1alpha1"
)

// DummyReconciler reconciles a Dummy object
type DummyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// TODO: write unit tests and publish to dockerhub

//+kubebuilder:rbac:groups=interview.com,resources=dummies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=interview.com,resources=dummies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=interview.com,resources=dummies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// Modify the Reconcile function to compare the state specified by
// the Dummy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *DummyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("Starting reconciling for dummy resource at %s", req.String()))
	defer logger.Info(fmt.Sprintf("Ending reconciling for dummy resource at %s", req.String()))
	nginxName := fmt.Sprintf("nginx-%s", req.Name)
	dummy, err := r.Resource(ctx, req.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, r.DeletePod(ctx, nginxName, req.Namespace)
		}
		return ctrl.Result{}, err
	}
	logger.Info(fmt.Sprintf("Message: %s", dummy.Spec.Message))
	err = r.Init(ctx, dummy)
	if err != nil {
		return ctrl.Result{}, err
	}
	// TODO: clean up and comments
	switch dummy.Status.PodStatus {
	case v1.PodPending:
		// check if pod exits
		ok, err := r.HasPod(ctx, nginxName, req.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !ok {
			// if not create it
			return ctrl.Result{}, r.CreatePod(ctx, nginxName, req.Namespace)
		}
		pod, err := r.GetPod(ctx, nginxName, req.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		// check if pod if ready
		if pod.Status.Phase == v1.PodRunning {
			dummy.Status.PodStatus = v1.PodRunning
			err = r.Status().Update(ctx, dummy)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		// if not requeue
		return ctrl.Result{RequeueAfter: time.Second * 3}, nil
	case v1.PodRunning:
		// check if pod exits
		ok, err := r.HasPod(ctx, nginxName, req.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !ok {
			// if not, set status back to pending
			dummy.Status.PodStatus = v1.PodPending
			err = r.Status().Update(ctx, dummy)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		pod, err := r.GetPod(ctx, nginxName, req.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		// check if pod is ready and change state
		if pod.Status.Phase == v1.PodPending {
			dummy.Status.PodStatus = v1.PodPending
			err = r.Status().Update(ctx, dummy)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, nil
}

// Init - Initializes the operator fields
func (r *DummyReconciler) Init(ctx context.Context, obj *interviewcomv1alpha1.Dummy) error {
	if obj.Status.PodStatus != "" {
		return nil
	}
	obj.Status.SpecEcho = obj.Spec.Message
	obj.Status.PodStatus = v1.PodPending
	return r.Status().Update(ctx, obj)
}

// Resource - Returns the underlying operator structure
func (r *DummyReconciler) Resource(ctx context.Context, name types.NamespacedName) (*interviewcomv1alpha1.Dummy, error) {
	dummy := &interviewcomv1alpha1.Dummy{}
	return dummy, r.Get(ctx, name, dummy)
}

// GetPod - Returns the pod with the given name from the namespace
func (r *DummyReconciler) GetPod(ctx context.Context, name, namespace string) (*v1.Pod, error) {
	p := &v1.Pod{}
	return p, r.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, p)
}

// HasPod - check if an pod is there
func (r *DummyReconciler) HasPod(ctx context.Context, name, namespace string) (bool, error) {
	p := &v1.Pod{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, p)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreatePod - creates a pod
func (r *DummyReconciler) CreatePod(ctx context.Context, name, namespace string) error {
	nginx := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  "nginx-sample",
				Image: "nginx",
			}},
		},
	}
	return r.Create(ctx, nginx)
}

// DeletePod - deletes the pod with the given name and namespace
func (r *DummyReconciler) DeletePod(ctx context.Context, name, namespace string) error {
	ok, err := r.HasPod(ctx, name, namespace)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	pod, err := r.GetPod(ctx, name, namespace)
	if err != nil {
		return err
	}
	return r.Delete(ctx, pod)
}

// SetupWithManager sets up the controller with the Manager.
func (r *DummyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&interviewcomv1alpha1.Dummy{}).
		Complete(r)
}
