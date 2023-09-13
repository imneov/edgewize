/*
Copyright 2019 The KubeSphere Authors.

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

package apptemplate

import (
	"context"

	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
)

const (
	controllerName = "apptemplate-controller"
	Finalizer      = "apptemplate.finalizer.edge.edgewize.io"
)

// Reconciler reconciles a Workspace object
type Reconciler struct {
	client.Client
	Logger                  logr.Logger
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Logger.GetSink() == nil {
		r.Logger = ctrl.Log.WithName("controllers").WithName(controllerName)
	}
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(controllerName)
	}
	if r.MaxConcurrentReconciles <= 0 {
		r.MaxConcurrentReconciles = 1
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		For(&appsv1alpha1.AppTemplate{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=edge.edgewize.io,resources=apptemplate,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edge.edgewize.io,resources=apptemplateversion,verbs=get;list;watch;delete
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.WithValues("AppTemplate", req.NamespacedName)
	logger.V(7).Info("receive request", "req", req)
	rootCtx := context.Background()
	instance := &appsv1alpha1.AppTemplate{}
	if err := r.Get(rootCtx, req.NamespacedName, instance); err != nil {
		logger.V(7).Error(err, "get apptemplate error")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !sliceutil.HasString(instance.ObjectMeta.Finalizers, Finalizer) {
			logger.V(4).Info("apptemplate is created, add finalizer and update", "req", req, "finalizer", Finalizer)
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *appsv1alpha1.AppTemplate) error {
				_instance.ObjectMeta.Finalizers = append(_instance.ObjectMeta.Finalizers, Finalizer)
				return r.Update(rootCtx, _instance)
			}); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if sliceutil.HasString(instance.ObjectMeta.Finalizers, Finalizer) {
			if _, err := r.undoReconcile(ctx, instance); err != nil {
				logger.Error(err, "undoReconcile failed", "instance", instance.Name)
			}
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *appsv1alpha1.AppTemplate) error {
				// remove our finalizer from the list and update it.
				_instance.ObjectMeta.Finalizers = sliceutil.RemoveString(_instance.ObjectMeta.Finalizers, func(item string) bool {
					return item == Finalizer
				})
				logger.V(4).Info("update apptemplate")
				return r.Update(rootCtx, _instance)
			}); err != nil {
				logger.Error(err, "update apptemplate failed")
				return ctrl.Result{}, err
			}
		}
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) UpdateInstance(ctx context.Context, nn types.NamespacedName, updateFunc func(deployer *appsv1alpha1.AppTemplate) error) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		instance := &appsv1alpha1.AppTemplate{}
		if err := r.Get(ctx, nn, instance); err != nil {
			return client.IgnoreNotFound(err)
		}
		return updateFunc(instance)
	})
}

// When deleting an 'AppTemplate,' also delete the corresponding 'AppTemplateVersion'.
func (r *Reconciler) undoReconcile(ctx context.Context, instance *appsv1alpha1.AppTemplate) (ctrl.Result, error) {
	rootCtx := context.Background()
	var err error
	selector := labels.SelectorFromSet(map[string]string{apisappsv1alpha1.LabelAppTemplate: instance.Name})
	apptemplateVersions := appsv1alpha1.AppTemplateVersionList{}
	if err = r.List(rootCtx, &apptemplateVersions, &client.ListOptions{LabelSelector: selector}); err == nil {
		for _, item := range apptemplateVersions.Items {
			err = r.Delete(rootCtx, &item)
		}
	}
	return ctrl.Result{}, err
}
