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
	"bytes"
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	yaml2 "k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"text/template"
	"time"

	interviewcomv1alpha1 "github.com/AgentNemo00/operator/api/v1alpha1"
)

type Naming func(req ctrl.Request) string

type SubPods struct {
	Template string
	Naming   Naming
}

// DummyReconciler reconciles a Dummy object
type DummyReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	PodTemplates map[string]SubPods
}

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
	dummy, err := r.resource(ctx, req.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.processDeletion(ctx, req)
		}
		return ctrl.Result{}, err
	}
	logger.Info(fmt.Sprintf("Message: %s", dummy.Spec.Message))
	err = r.init(ctx, dummy)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info(fmt.Sprintf("Dummy in state %s", dummy.Status.PodStatus))
	switch dummy.Status.PodStatus {
	case v1.PodPending:
		return r.processPending(ctx, req, dummy)
	case v1.PodRunning:
		return r.processRunning(ctx, req, dummy)
	}
	return ctrl.Result{}, nil
}

// processDeletion - process for the cleanup after the resource is deleted
func (r *DummyReconciler) processDeletion(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var retErr error
	for _, pods := range r.PodTemplates {
		podName := pods.Naming(req)
		err := r.deletePod(ctx, podName, req.Namespace)
		if err != nil {
			retErr = err
		}
	}
	return ctrl.Result{}, retErr
}

// processRunning - process for the running state
func (r *DummyReconciler) processRunning(ctx context.Context, req ctrl.Request, dummy *interviewcomv1alpha1.Dummy) (ctrl.Result, error) {
	nginxTemplate := r.PodTemplates["nginx"]
	nginxName := nginxTemplate.Naming(req)
	// check if pod exits
	ok, err := r.hasPod(ctx, nginxName, req.Namespace)
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
		return ctrl.Result{Requeue: true}, nil
	}
	pod, err := r.getPod(ctx, nginxName, req.Namespace)
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
		return ctrl.Result{RequeueAfter: time.Second * 3}, nil
	}
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// processPending - process for the pending state
func (r *DummyReconciler) processPending(ctx context.Context, req ctrl.Request, dummy *interviewcomv1alpha1.Dummy) (ctrl.Result, error) {
	nginxTemplate := r.PodTemplates["nginx"]
	nginxName := nginxTemplate.Naming(req)
	// check if pod exits
	ok, err := r.hasPod(ctx, nginxName, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !ok {
		// if not create it, and check again
		return ctrl.Result{RequeueAfter: time.Second * 5}, r.createPod(ctx, nginxName, req.Namespace, nginxTemplate.Template)
	}
	pod, err := r.getPod(ctx, nginxName, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	// check if pod is ready
	if pod.Status.Phase == v1.PodRunning {
		dummy.Status.PodStatus = v1.PodRunning
		err = r.Status().Update(ctx, dummy)
		if err != nil {
			return ctrl.Result{}, err
		}
		// requeue to process running state
		return ctrl.Result{Requeue: true}, nil
	}
	// if not wait and requeue to check the state again
	return ctrl.Result{RequeueAfter: time.Second * 3}, nil
}

// init - Initializes the operator fields
func (r *DummyReconciler) init(ctx context.Context, obj *interviewcomv1alpha1.Dummy) error {
	if obj.Status.PodStatus != "" {
		return nil
	}
	obj.Status.SpecEcho = obj.Spec.Message
	obj.Status.PodStatus = v1.PodPending
	return r.Status().Update(ctx, obj)
}

// resource - Returns the underlying operator structure
func (r *DummyReconciler) resource(ctx context.Context, name types.NamespacedName) (*interviewcomv1alpha1.Dummy, error) {
	dummy := &interviewcomv1alpha1.Dummy{}
	return dummy, r.Get(ctx, name, dummy)
}

// getPod - Returns the pod with the given name from the namespace
func (r *DummyReconciler) getPod(ctx context.Context, name, namespace string) (*v1.Pod, error) {
	p := &v1.Pod{}
	return p, r.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}, p)
}

// hasPod - check if a pod is there
func (r *DummyReconciler) hasPod(ctx context.Context, name, namespace string) (bool, error) {
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

// createPod - creates a pod
func (r *DummyReconciler) createPod(ctx context.Context, name, namespace, templatePath string) error {
	config := &struct {
		Name      string
		Namespace string
	}{
		Name:      name,
		Namespace: namespace,
	}
	podTemplate, err := template.ParseFiles(templatePath)
	if err != nil {
		return err
	}
	buffer := &bytes.Buffer{}
	err = podTemplate.Execute(buffer, config)
	if err != nil {
		return err
	}
	nginx := &v1.Pod{}
	decoder := yaml2.NewYAMLOrJSONDecoder(buffer, buffer.Len())
	err = decoder.Decode(nginx)
	if err != nil {
		return err
	}
	return r.Create(ctx, nginx)
}

// deletePod - deletes the pod with the given name and namespace
func (r *DummyReconciler) deletePod(ctx context.Context, name, namespace string) error {
	ok, err := r.hasPod(ctx, name, namespace)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	pod, err := r.getPod(ctx, name, namespace)
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
