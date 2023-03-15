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

package edgecluster

import (
	"context"
	"errors"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	controllerutils "github.com/edgewize-io/edgewize/pkg/controller/utils/controller"
	"github.com/edgewize-io/edgewize/pkg/helm"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	controllerName = "edgecluster-controller"
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
	if r.Logger == nil {
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
		For(&infrav1alpha1.EdgeCluster{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=infra.edgewize.io,resources=edgeclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infra.edgewize.io,resources=edgeclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infra.edgewize.io,resources=edgeclusters/finalizers,verbs=update
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.WithValues("edgecluster", req.NamespacedName)
	logger.V(4).Info("receive request", "req", req)
	rootCtx := context.Background()
	instance := &infrav1alpha1.EdgeCluster{}
	if err := r.Get(rootCtx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// name of your custom finalizer
	finalizer := "edgeclusterfinalizer.infra.edgewize.io"

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !sliceutil.HasString(instance.ObjectMeta.Finalizers, finalizer) {
			logger.V(4).Info("edgecluster is created, add finalizer and update", "req", req, "finalizer", finalizer)
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizer)
			if err := r.Update(rootCtx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if sliceutil.HasString(instance.ObjectMeta.Finalizers, finalizer) {
			// TODO undoReconcile
			// remove our finalizer from the list and update it.
			instance.ObjectMeta.Finalizers = sliceutil.RemoveString(instance.ObjectMeta.Finalizers, func(item string) bool {
				return item == finalizer
			})
			logger.V(4).Info("update edgecluster")
			if err := r.Update(rootCtx, instance); err != nil {
				logger.Error(err, "update edgecluster failed")
				return ctrl.Result{}, err
			}
		}
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}
	if _, err := r.doReconcile(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	r.Recorder.Event(instance, corev1.EventTypeNormal, controllerutils.SuccessSynced, controllerutils.MessageResourceSynced)
	return ctrl.Result{}, nil
}

func (r *Reconciler) undoReconcile(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "func", "undoReconcile", "instance", instance.Name)
	logger.V(4).Info("delete edgecluster", "instance", instance)
	switch instance.Status.Status {
	case infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		status, err := helm.Status(instance.Spec.Distro, instance.Spec.Name, instance.Spec.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		switch status {
		case "deployed", "superseded", "failed", "pending-install", "pending-upgrade", "pending-rollback":
			logger.V(4).Info("begin uninstall edgecluster ", "status", status)
			instance.Status.Status = infrav1alpha1.UninstallingStatus
			err = helm.Uninstall(instance.Spec.Distro, instance.Spec.Name, instance.Spec.Namespace)
			if err != nil {
				logger.Error(err, "uninstall edge cluster error")
				return ctrl.Result{}, err
			}
			logger.V(4).Info("uninstall edgecluster success", "name", instance.Name)
		}
	}
	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) doReconcile(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "doReconcile", instance.Name)
	if instance.Spec.Name == "" || instance.Spec.Namespace == "" {
		return ctrl.Result{}, errors.New("cluster name and namespace cannot be empty")
	}
	if instance.Spec.Distro == "" {
		instance.Spec.Distro = "k3s"
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	logger.V(4).Info("doReconcile ", "edgecluster", instance)

	switch instance.Status.Status {
	case "", infrav1alpha1.InstallingStatus:
		status, err := helm.Status(instance.Spec.Distro, instance.Spec.Name, instance.Spec.Namespace)
		if err != nil {
			return ctrl.Result{}, err
		}
		logger.V(4).Info("doReconcile ", "helm release status", status)
		switch status {
		case release.StatusUnknown:
			logger.Info("install edge cluster", "name", instance.Spec.Name)
			instance.Status.Status = infrav1alpha1.InstallingStatus
			err = helm.Install(instance.Spec.Distro, instance.Spec.Name, instance.Spec.Namespace)
			if err != nil {
				logger.Error(err, "install edge cluster error")
				return ctrl.Result{}, err
			}
		case release.StatusUninstalling:
			instance.Status.Status = infrav1alpha1.UninstallingStatus
		case release.StatusUninstalled:
			instance.Status.Status = infrav1alpha1.UninstalledStatus
		case release.StatusDeployed:
			instance.Status.Status = infrav1alpha1.RunningStatus
		case release.StatusFailed:
			instance.Status.Status = infrav1alpha1.ErrorStatus
		}
	case infrav1alpha1.RunningStatus:

	case infrav1alpha1.ErrorStatus:
		logger.Info("edge cluster install error", "name", instance.Spec.Name)
	case infrav1alpha1.UninstalledStatus:
		logger.Info("edge cluster uninstalled", "name", instance.Spec.Name)
	}
	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}
