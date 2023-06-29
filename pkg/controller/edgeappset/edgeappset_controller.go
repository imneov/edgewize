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

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
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
			if _, err := r.undoReconcile(ctx, req.NamespacedName, instance); err != nil {
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
	return r.doReconcile(ctx, req.NamespacedName, instance)
}

func (r *Reconciler) undoReconcile(ctx context.Context, nn types.NamespacedName, instance *appsv1alpha1.EdgeAppSet) (ctrl.Result, error) {
	deployList := &appsv1.DeploymentList{}
	err := r.List(ctx, deployList, client.InNamespace(""), client.MatchingLabels{
		appsv1alpha1.LabelEdgeAppSet: instance.Name,
	})
	if err != nil {
		klog.Error(err, "list deployments failed")
		return ctrl.Result{}, err
	}
	for _, deploy := range deployList.Items {
		if err := r.Delete(ctx, &deploy); err != nil {
			klog.Error(err, "delete deployment failed", "deployment", deploy.Name)
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) doReconcile(ctx context.Context, nn types.NamespacedName, instance *appsv1alpha1.EdgeAppSet) (ctrl.Result, error) {
	logger := r.Logger.WithName("doReconcile")
	logger.V(3).Info("start reconcile", "instance", instance.Name)

	// 获取Deployment的数量和状态
	deployments, err := r.getDeployments(ctx, instance)
	if err != nil {
		// 处理错误
		return ctrl.Result{}, err
	}
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Get(ctx, nn, instance); err != nil {
			return err
		}
		instance.Status.WorkloadCount = len(deployments)
		instance.Status.Deployments = deployments
		return r.Status().Update(ctx, instance)
	})
	if err != nil {
		logger.Error(err, "update status failed", "instance", instance.Name)
		return ctrl.Result{}, err
	}

	// 如果状态中工作负载总数为0，则为每个 nodeselector 创建一个 deployment
	klog.V(5).Infof("origin deploy: %+v", instance.Spec.DeploymentTemplate)

	if instance.Status.WorkloadCount < len(instance.Spec.NodeSelectors) {
		for i, selector := range instance.Spec.NodeSelectors {
			var name string
			if selector.NodeGroup != "" {
				name = fmt.Sprintf("%s-%s-%d", instance.Name, selector.NodeGroup, i)
			} else {
				name = fmt.Sprintf("%s-%d", instance.Name, i)
			}

			if _, ok := deployments[name]; ok {
				continue
			}

			deploy := &appsv1.Deployment{}
			deploy.Name = name
			deploy.Labels = map[string]string{
				"app": deploy.Name,
			}
			deploy.Labels[appsv1alpha1.LabelEdgeAppSet] = instance.Name
			deploy.Labels[appsv1alpha1.LabelNodeGroup] = selector.NodeGroup
			deploy.OwnerReferences = []metav1.OwnerReference{
				*metav1.NewControllerRef(instance, instance.GetObjectKind().GroupVersionKind()),
			}

			deploy.Namespace = instance.Namespace
			deploy.Spec = instance.Spec.DeploymentTemplate.Spec
			deploy.Spec.Template.Labels = map[string]string{
				"app": deploy.Name,
			}
			deploy.Spec.Selector.MatchLabels = map[string]string{
				"app": deploy.Name,
			}
			// 部署到指定节点
			if selector.NodeName != "" {
				deploy.Spec.Template.Spec.NodeName = selector.NodeName
				deploy.Labels[appsv1alpha1.LabelNode] = selector.NodeGroup
			}
			klog.V(5).Infof("create deploy: %+v", deploy)
			err := r.Create(ctx, deploy)
			if err != nil {
				logger.Error(err, "create deployment error", "name", deploy.Name, "namespace", deploy.Namespace)
			}
		}
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

func (r *Reconciler) getDeployments(ctx context.Context, edgeAppSet *appsv1alpha1.EdgeAppSet) (map[string]appsv1alpha1.DeploymentStatus, error) {
	deployments := make(map[string]appsv1alpha1.DeploymentStatus)

	// 使用Kubernetes客户端库获取Deployment列表
	deploymentList := &appsv1.DeploymentList{}
	err := r.List(ctx, deploymentList, &client.ListOptions{
		Namespace:     edgeAppSet.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{appsv1alpha1.LabelEdgeAppSet: edgeAppSet.Name}),
	})
	if err != nil {
		return nil, err
	}

	// 遍历Deployment列表，获取每个Deployment的状态
	for _, deployment := range deploymentList.Items {
		status := getDeploymentStatus(&deployment)
		deployments[deployment.Name] = appsv1alpha1.DeploymentStatus{
			Status: status,
		}
	}

	return deployments, nil
}

// 获取deployment的状态
func getDeploymentStatus(deployment *appsv1.Deployment) string {
	// 判断是否失败
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentReplicaFailure && condition.Status == corev1.ConditionTrue {
			return appsv1alpha1.Failed
		} else if condition.Type == appsv1.DeploymentAvailable && condition.Status == corev1.ConditionTrue {
			return appsv1alpha1.Succeeded
		}
	}
	if deployment.Status.Replicas == deployment.Status.ReadyReplicas {
		return appsv1alpha1.Succeeded
	}
	return appsv1alpha1.Progressing
}
