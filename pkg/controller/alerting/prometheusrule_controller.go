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

package alerting

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/go-logr/logr"
	promresourcesv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"

	clusterutils "github.com/edgewize-io/edgewize/pkg/controller/cluster/utils"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
)

// GlobalPrometheusRuleReconcilers syncs prometheusrules across cluster
type GlobalPrometheusRuleReconcilers struct {
	Client client.Client
	Logger logr.Logger

	reconcilerMap    map[string]reconcilerData
	reconcilerMapMux sync.Mutex
}

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/finalizers,verbs=update
// +kubebuilder:rbac:groups=infra.edgewize.io,resources=clusters,verbs=get
func (r *GlobalPrometheusRuleReconcilers) createReconcileFunc(clusterName string, mgr ctrl.Manager) reconcile.Func {
	sourceClient := mgr.GetClient() // source cluster client
	log := mgr.GetLogger()
	scheme := mgr.GetScheme()
	return func(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
		logger := log.WithValues("source prometheusrule", req.NamespacedName)
		logger.Info("sync prometheusrule")

		cluster := &infrav1alpha1.Cluster{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: clusterName}, cluster)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("ignore prometheusrule in the source cluster which is not found")
				return reconcile.Result{}, nil
			}
			return reconcile.Result{}, err
		}

		var targetRule = &promresourcesv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: PrometheusRuleNamespace,
				Name:      fmt.Sprintf("%s-%s", clusterName, req.Name),
			},
		}

		logger = logger.WithValues("target prometheusrule", targetRule.Namespace+"/"+targetRule.Name)

		var sourceRule = &promresourcesv1.PrometheusRule{}
		err = sourceClient.Get(ctx, req.NamespacedName, sourceRule)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.V(3).Info("source prometheusrule is not found and delete its target prometheusrule")
				err = r.Client.Delete(ctx, targetRule)
				return ctrl.Result{}, client.IgnoreNotFound(err)
			}
			return ctrl.Result{}, err
		}

		finalizer := "finalizers.alerting.kubesphere.io/prometheusrules"

		if sourceRule.GetDeletionTimestamp().IsZero() {
			// The object is not being deleted, so if it does not have our finalizer,
			// then lets add the finalizer and update the object.
			if !sliceutil.HasString(sourceRule.GetFinalizers(), finalizer) {
				sourceRule.SetFinalizers(append(sourceRule.GetFinalizers(), finalizer))
				return ctrl.Result{}, sourceClient.Update(ctx, sourceRule)
			}
		} else {
			// The object is being deleted
			if sliceutil.HasString(sourceRule.GetFinalizers(), finalizer) {
				logger.V(3).Info("source prometheusrule is being deleted, so delete its target prometheusrule")
				err = r.Client.Delete(ctx, targetRule)
				if err != nil && !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				// remove our finalizer from the list and update it.
				sourceRule.SetFinalizers(sliceutil.RemoveString(sourceRule.GetFinalizers(), func(item string) bool {
					return item == finalizer
				}))
				err = sourceClient.Update(ctx, sourceRule)
				if err != nil {
					return ctrl.Result{}, err
				}
			}
			// Our finalizer has finished, so the reconciler can do nothing.
			return ctrl.Result{}, nil
		}

		err = syncPrometheusRule(clusterName, sourceRule, targetRule)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = ctrl.SetControllerReference(cluster, targetRule, scheme)
		if err != nil {
			return ctrl.Result{}, err
		}

		var current promresourcesv1.PrometheusRule
		err = r.Client.Get(ctx, types.NamespacedName{
			Namespace: targetRule.Namespace,
			Name:      targetRule.Name,
		}, &current)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			// If the target rule does not exist, create it
			logger.V(3).Info("target prometheusrule not exists, so create it")
			err = r.Client.Create(ctx, targetRule)
			return ctrl.Result{}, err
		}
		// If the target rule exists, update it
		if !reflect.DeepEqual(targetRule.Spec, current.Spec) ||
			!reflect.DeepEqual(targetRule.Labels, current.Labels) ||
			!reflect.DeepEqual(targetRule.OwnerReferences, current.OwnerReferences) {
			targetRule.SetResourceVersion(current.GetResourceVersion())
			logger.V(3).Info("update target prometheusrule")
			err = r.Client.Update(ctx, targetRule)
			if err != nil && apierrors.IsConflict(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

}

func (r *GlobalPrometheusRuleReconcilers) setupReconciler(ctx context.Context,
	cluster *infrav1alpha1.Cluster, requeueCallback RequeueCallbackFunc) error {

	clusterName := cluster.Name
	kubeconfig := cluster.Spec.Connection.KubeConfig

	if reconciler, ok := r.reconcilerMap[clusterName]; ok {
		if bytes.Equal(reconciler.kubeconfig, kubeconfig) {
			return nil
		}
		// If kubeconfig changed, remove the reconciler and then setup it again
		r.removeReconciler(clusterName)
	}

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeconfig)
	if err != nil {
		return fmt.Errorf("unable to create client config from kubeconfig bytes, %#v", err)
	}
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get client config, %#v", err)
	}

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		MetricsBindAddress: "0", // disable the metrics serving
		Logger:             r.Logger.WithValues("source cluster", clusterName),
	})
	if err != nil {
		return err
	}

	prometheusRuleFilter := func(obj client.Object) bool {
		if obj.GetNamespace() != PrometheusRuleNamespace || obj.GetLabels() == nil {
			return false
		}
		_, hasOwnerCluster := obj.GetLabels()[PrometheusRuleResourceLabelKeyOwnerCluster]
		if hasOwnerCluster {
			return false
		}
		level, hasLevel := obj.GetLabels()[PrometheusRuleResourceLabelKeyRuleLevel]
		if !hasLevel {
			return false
		}
		switch level {
		case string(RuleLevelNamesapce):
			return true
		case string(RuleLevelCluster):
			return true
		}
		return false
	}

	err = ctrl.NewControllerManagedBy(mgr).
		For(&promresourcesv1.PrometheusRule{},
			builder.WithPredicates(predicate.NewPredicateFuncs(prometheusRuleFilter))).
		Complete(r.createReconcileFunc(clusterName, mgr))
	if err != nil {
		return err
	}

	childCtx, cancel := context.WithCancel(ctx)
	r.reconcilerMapMux.Lock()
	r.reconcilerMap[clusterName] = reconcilerData{
		cancel:     cancel,
		kubeconfig: kubeconfig,
	}
	r.reconcilerMapMux.Unlock()

	go func() {
		err := mgr.Start(childCtx)
		if err != nil {
			// if failed to start, clean and retry it
			r.removeReconciler(clusterName)
			if requeueCallback != nil {
				requeueCallback()
			}
		}
	}()

	return nil
}

func (r *GlobalPrometheusRuleReconcilers) removeReconciler(clusterName string) {
	r.reconcilerMapMux.Lock()
	defer r.reconcilerMapMux.Unlock()
	if reconciler, ok := r.reconcilerMap[clusterName]; ok {
		if reconciler.cancel != nil {
			reconciler.cancel()
		}
		delete(r.reconcilerMap, clusterName)
	}
}

func (r *GlobalPrometheusRuleReconcilers) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	if r.Logger.GetSink() == nil {
		r.Logger = mgr.GetLogger()
	}

	r.reconcilerMap = map[string]reconcilerData{}

	return (&clusterReconciler{
		prometheusRuleReconcilers: r,
		Client:                    r.Client,
	}).SetupWithMananger(mgr)

}

type reconcilerData struct {
	cancel     context.CancelFunc
	kubeconfig []byte
}

type RequeueCallbackFunc func()

type clusterReconciler struct {
	client.Client

	Log logr.Logger

	prometheusRuleReconcilers *GlobalPrometheusRuleReconcilers
	enqueueRequestForObject   *enqueueRequestForObject
}

// +kubebuilder:rbac:groups=infra.edgewize.io,resources=clusters,verbs=get;list;watch
func (r *clusterReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := r.Log.WithValues("cluster", req.NamespacedName)

	cluster := &infrav1alpha1.Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.prometheusRuleReconcilers.removeReconciler(cluster.Name)
			logger.Info("remove cluster from ClusterReconciler", "cluster", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if !clusterutils.IsClusterReady(cluster) {
		logger.Info("cluster is not ready")
		r.prometheusRuleReconcilers.removeReconciler(cluster.Name)
		return reconcile.Result{}, nil
	}

	// setup the prometheusrule reconciler for the cluster
	// if it failed to start, call the requeue callback func to requeue the cluster.
	err = r.prometheusRuleReconcilers.setupReconciler(ctx, cluster, func() {
		r.enqueueRequestForObject.requeue(req)
	})
	return reconcile.Result{}, err
}

func (r *clusterReconciler) SetupWithMananger(mgr ctrl.Manager) error {
	if r.Log.GetSink() == nil {
		r.Log = mgr.GetLogger()
	}
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}
	ctr, err := controller.New("cluster", mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return err
	}
	r.enqueueRequestForObject = &enqueueRequestForObject{}
	err = ctr.Watch(
		&source.Kind{Type: &infrav1alpha1.Cluster{}},
		r.enqueueRequestForObject)
	return err
}

type enqueueRequestForObject struct {
	handler.EnqueueRequestForObject
	queue workqueue.RateLimitingInterface
	once  sync.Once
}

func (e *enqueueRequestForObject) initQueue(q workqueue.RateLimitingInterface) {
	e.once.Do(func() {
		if e.queue == nil {
			e.queue = q
		}
	})
}

// requeue adds the request to queue again,
// and should be called for asynchronous reconcile error
func (e *enqueueRequestForObject) requeue(req reconcile.Request) {
	e.queue.AddRateLimited(req)
}

func (e *enqueueRequestForObject) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.initQueue(q)
	e.EnqueueRequestForObject.Create(evt, q)
}

func (e *enqueueRequestForObject) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.initQueue(q)
	e.EnqueueRequestForObject.Update(evt, q)
}

func (e *enqueueRequestForObject) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.initQueue(q)
	e.EnqueueRequestForObject.Delete(evt, q)
}

func (e *enqueueRequestForObject) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.initQueue(q)
	e.EnqueueRequestForObject.Generic(evt, q)
}
