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

package edgeappset

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
)

const (
	controllerName = "edgeappset-controller"
	Finalizer      = "edgeappset.finalizer.edge.edgewize.io"
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
		For(&appsv1alpha1.EdgeAppSet{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=edge.edgewize.io,resources=edgeappsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=edge.edgewize.io,resources=edgeappsets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=edge.edgewize.io,resources=edgeappsets/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=,resources=deployments/status,verbs=get

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.WithValues("EdgeAppSet", req.NamespacedName)
	logger.V(7).Info("receive request", "req", req)
	rootCtx := context.Background()
	instance := &appsv1alpha1.EdgeAppSet{}
	if err := r.Get(rootCtx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !sliceutil.HasString(instance.ObjectMeta.Finalizers, Finalizer) {
			logger.V(4).Info("edge cluster is created, add finalizer and update", "req", req, "finalizer", Finalizer)
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *appsv1alpha1.EdgeAppSet) error {
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
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *appsv1alpha1.EdgeAppSet) error {
				// remove our finalizer from the list and update it.
				_instance.ObjectMeta.Finalizers = sliceutil.RemoveString(_instance.ObjectMeta.Finalizers, func(item string) bool {
					return item == Finalizer
				})
				logger.V(4).Info("update edge cluster")
				return r.Update(rootCtx, _instance)
			}); err != nil {
				logger.Error(err, "update edge cluster failed")
				return ctrl.Result{}, err
			}
		}
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}
	r.verifyEdgeAppSet(ctx, instance)
	return r.doReconcile(ctx, instance)
}

func (r *Reconciler) undoReconcile(ctx context.Context, instance *appsv1alpha1.EdgeAppSet) (ctrl.Result, error) {
	// do nothing in current version
	return ctrl.Result{}, nil
}

// TODO delete
func (r *Reconciler) verifyEdgeAppSet(ctx context.Context, instance *appsv1alpha1.EdgeAppSet) {
	uniqueNodeSelector := make(map[string]appsv1alpha1.NodeSelector)
	for _, nodeSelector := range instance.Spec.NodeSelectors {
		uniqueName := fmt.Sprintf("%s-%s-%s", nodeSelector.Project, nodeSelector.NodeGroup, nodeSelector.NodeName)
		uniqueNodeSelector[uniqueName] = nodeSelector
	}
	if len(uniqueNodeSelector) == len(instance.Spec.NodeSelectors) {
		return
	} else {
		instance.Spec.NodeSelectors = make([]appsv1alpha1.NodeSelector, 0, len(uniqueNodeSelector))
		for _, nodeSelector := range uniqueNodeSelector {
			instance.Spec.NodeSelectors = append(instance.Spec.NodeSelectors, nodeSelector)
		}
	}
}

func (r *Reconciler) doReconcile(ctx context.Context, instance *appsv1alpha1.EdgeAppSet) (ctrl.Result, error) {
	logger := r.Logger.WithName("doReconcile")
	logger.V(3).Info("start reconcile", "instance", instance.Name)

	if instance.Status.WorkloadCount != len(instance.Spec.NodeSelectors) {
		instance.Status.WorkloadCount = len(instance.Spec.NodeSelectors)
	}

	//if instance.Status.UpdatedWorkloadCount != instance.Status.WorkloadCount {
	if instance.Status.UpdatedWorkloadCount != instance.Status.WorkloadCount {
		count, err := r.updateDeployments(ctx, instance)
		if err != nil {
			logger.Error(err, "update deployments failed", "instance", instance.Name)
			return ctrl.Result{RequeueAfter: 3 * time.Second}, err
		}
		instance.Status.UpdatedWorkloadCount = count
	}

	allDeployments, err := r.getAllDeployments(ctx, instance)
	if err != nil {
		logger.Error(err, "get all deployments failed", "instance", instance.Name)
		return ctrl.Result{}, err
	}
	instance.Status.ReadyWorkloadCount = readyWorkloadCounts(allDeployments)
	instance.Status.UnavailableWorkloadCount = unavailableWorkloadCounts(allDeployments)

	err = r.Status().Update(ctx, instance)
	if err != nil {
		logger.Error(err, "update status failed", "instance", instance.Name)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) UpdateInstance(ctx context.Context, nn types.NamespacedName, updateFunc func(deployer *appsv1alpha1.EdgeAppSet) error) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		instance := &appsv1alpha1.EdgeAppSet{}
		if err := r.Get(ctx, nn, instance); err != nil {
			return client.IgnoreNotFound(err)
		}
		return updateFunc(instance)
	})
}

func (r *Reconciler) getAllDeployments(ctx context.Context, edgeAppSet *appsv1alpha1.EdgeAppSet) ([]appsv1.Deployment, error) {
	deploymentList := &appsv1.DeploymentList{}
	err := r.List(ctx, deploymentList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{appsv1alpha1.LabelEdgeAppSet: edgeAppSet.Name}),
	})
	if err != nil {
		return nil, err
	}

	return deploymentList.Items, nil
}

func (r *Reconciler) updateDeployments(ctx context.Context, instance *appsv1alpha1.EdgeAppSet) (int, error) {
	logger := r.Logger.WithName("updateDeployments")
	allDeployments, err := r.getAllDeployments(ctx, instance)
	if err != nil {
		logger.Error(err, "get all deployments failed", "instance", instance.Name)
		return 0, err
	}

	oldDeployments := make(map[string]appsv1.Deployment, 0)
	for _, deployment := range allDeployments {
		uniqueName := fmt.Sprintf("%s-%s-%s", deployment.Namespace, deployment.Labels[appsv1alpha1.LabelNodeGroup], deployment.Labels[appsv1alpha1.LabelNode])
		oldDeployments[uniqueName] = deployment
	}

	// createDeployments is which will be created
	createDeployments := make(map[string]*appsv1.Deployment)
	// deleteDeployments is which will be deleted
	deleteDeployments := make(map[string]*appsv1.Deployment)
	for _, selector := range instance.Spec.NodeSelectors {
		uniqueName := fmt.Sprintf("%s-%s-%s", selector.Project, selector.NodeGroup, selector.NodeName)
		if _, ok := oldDeployments[uniqueName]; ok {
			delete(oldDeployments, uniqueName)
		} else {
			target := buildDeployment(instance, selector)
			createDeployments[uniqueName] = target
		}
	}
	for _, deployment := range oldDeployments {
		uniqueName := fmt.Sprintf("%s-%s-%s", deployment.Namespace, deployment.Labels[appsv1alpha1.LabelNodeGroup], deployment.Labels[appsv1alpha1.LabelNode])
		deleteDeployments[uniqueName] = &deployment
	}

	count := len(oldDeployments)
	for _, deployment := range createDeployments {
		err = r.Create(ctx, deployment)
		if err != nil {
			logger.Error(err, "create deployment failed", "deployment", deployment.Name)
			continue
		}
		count += 1
	}
	for _, deployment := range deleteDeployments {
		err = r.Delete(ctx, deployment)
		if err != nil {
			logger.Error(err, "delete deployment failed", "deployment", deployment.Name)
			continue
		}
		count -= 1
	}
	return count, nil
}
