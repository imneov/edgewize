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
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	controllerutils "github.com/edgewize-io/edgewize/pkg/controller/utils/controller"
	"github.com/edgewize-io/edgewize/pkg/helm"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/homedir"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	controllerName     = "edgecluster-controller"
	DefaultComponents  = "edgewize,cloudcore,-fluent-operator"
	ComponentEdgeWize  = "edgewize"
	ComponentCloudCore = "cloudcore"
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
	_ = os.MkdirAll(filepath.Join(homedir.HomeDir(), ".kube"), 0644)
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
			logger.V(4).Info("edge cluster is created, add finalizer and update", "req", req, "finalizer", finalizer)
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, finalizer)
			if err := r.Update(rootCtx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if sliceutil.HasString(instance.ObjectMeta.Finalizers, finalizer) {
			if _, err := r.undoReconcile(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
			// remove our finalizer from the list and update it.
			instance.ObjectMeta.Finalizers = sliceutil.RemoveString(instance.ObjectMeta.Finalizers, func(item string) bool {
				return item == finalizer
			})
			logger.V(4).Info("update edge cluster")
			if err := r.Update(rootCtx, instance); err != nil {
				logger.Error(err, "update edge cluster failed")
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
	logger.V(4).Info("delete edge cluster", "instance", instance)
	switch instance.Status.Status {
	case infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		status, err := helm.Status(instance.Spec.Distro, instance.Name, instance.Spec.Namespace, "")
		if err != nil {
			return ctrl.Result{}, err
		}
		switch status {
		case "deployed", "superseded", "failed", "pending-install", "pending-upgrade", "pending-rollback":
			logger.V(4).Info("begin uninstall edge cluster ", "status", status)
			instance.Status.Status = infrav1alpha1.UninstallingStatus
			err = helm.Uninstall(instance.Spec.Distro, instance.Name, instance.Spec.Namespace, "")
			if err != nil {
				logger.Error(err, "uninstall edge cluster error")
				return ctrl.Result{}, err
			}
			logger.V(4).Info("uninstall edge cluster success", "name", instance.Name)
		}
	}
	member := &infrav1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
	}
	if err := r.Delete(ctx, member); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	// remove pvc and namespace
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("data-%s-0", instance.Name),
			Namespace: instance.Spec.Namespace,
		},
	}
	if err := r.Delete(ctx, pvc); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Spec.Namespace,
		},
	}
	if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) doReconcile(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "doReconcile", instance.Name)
	if instance.Name == "" || instance.Spec.Namespace == "" {
		return ctrl.Result{}, errors.New("cluster name and namespace cannot be empty")
	}
	logger.V(3).Info("origin value", "distro", instance.Spec.Distro, "components", instance.Spec.Components, "advertiseaddress", instance.Spec.AdvertiseAddress)
	if instance.Spec.Distro == "" {
		instance.Spec.Distro = "k3s" // TODO constants
	}
	if instance.Spec.Components == "" {
		instance.Spec.Components = DefaultComponents
	} else if !strings.Contains(instance.Spec.Components, ComponentEdgeWize) {
		instance.Spec.Components = fmt.Sprintf("%s,%s", ComponentEdgeWize, instance.Spec.Components)
	}
	if instance.Spec.AdvertiseAddress == nil {
		instance.Spec.AdvertiseAddress = []string{}
	}
	logger.V(3).Info("set default value", "distro", instance.Spec.Distro, "components", instance.Spec.Components, "advertiseaddress", instance.Spec.AdvertiseAddress)
	if err := r.Update(ctx, instance); err != nil {
		logger.Error(err, "update edge cluster error")
		return ctrl.Result{}, err
	}

	member := &infrav1alpha1.Cluster{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name}, member)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(4).Info("cluster not found, create new cluster", "name", instance.Name)
			member = &infrav1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: instance.Name,
					Labels: map[string]string{
						infrav1alpha1.MemberClusterRole: "",
						infrav1alpha1.ClusterAlias:      instance.Spec.Alias,
					},
				},
				Spec: infrav1alpha1.ClusterSpec{
					HostCluster: instance.Spec.HostCluster,
					Provider:    "edgewize",
					Connection: infrav1alpha1.Connection{
						Type:       infrav1alpha1.ConnectionTypeDirect,
						KubeConfig: []byte(instance.Status.KubeConfig),
					},
				},
			}
			if instance.Spec.Location != "" {
				member.Labels[infrav1alpha1.ClusterLocation] = instance.Spec.Location
			}
			err := r.Create(ctx, member)
			if err != nil {
				logger.Error(err, "create cluster error", "name", instance.Name)
				return ctrl.Result{}, err
			}
		}
	}

	switch instance.Status.Status {
	case "", infrav1alpha1.InstallingStatus:
		logger.V(2).Info("install edge cluster")
		status, err := InstallChart(instance.Spec.Distro, instance.Name, instance.Spec.Namespace, "", nil)
		if err != nil {
			logger.Error(err, "install edge cluster error")
			return ctrl.Result{}, err
		}
		instance.Status.Status = status
	case infrav1alpha1.RunningStatus:
		if instance.Status.KubeConfig == "" {
			config, err := r.GetKubeConfig(instance)
			if err != nil {
				logger.Error(err, "get edge cluster kube config error")
				return ctrl.Result{}, err
			}
			data, err := clientcmd.Write(*config)
			if err != nil {
				logger.Error(err, "encode edge cluster kube config error")
				return ctrl.Result{}, err
			}
			instance.Status.KubeConfig = string(data)
		}
		err := CheckKubeConfig(instance.Name, []byte(instance.Status.KubeConfig))
		if err != nil {
			logger.Error(err, "write edge cluster kube config to file error")
			return ctrl.Result{}, err
		}
		components := strings.Split(instance.Spec.Components, ",")
		for _, component := range components {
			if len(component) > 0 && component[0] != '-' {
				switch component {
				case "edgewize":
					err = r.ReconcileEdgeWize(ctx, instance, member)
					if err != nil {
						logger.Error(err, "install edgewize agent error")
						return ctrl.Result{}, err
					}
				case "cloudcore":
					err = r.ReconcileCloudCore(ctx, instance)
					if err != nil {
						logger.Error(err, "install cloudcore error")
						return ctrl.Result{}, err
					}
				case "fluent-operator":
					err = r.ReconcileFluentOperator(ctx, instance)
					if err != nil {
						logger.Error(err, "install cloudcore error")
						return ctrl.Result{}, err
					}
				default:
					logger.Info(fmt.Sprintf("unknown component %s", component))
				}
			}
		}

	case infrav1alpha1.ErrorStatus:
		logger.Info("edge cluster install error", "name", instance.Name)
	}
	if err := r.Status().Update(ctx, instance); err != nil {
		logger.Error(err, "install edge cluster status error")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) GetKubeConfig(instance *infrav1alpha1.EdgeCluster) (*clientcmdapi.Config, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: instance.Spec.Namespace,
		Name:      fmt.Sprintf("vc-%s", instance.Name),
	}
	err := r.Get(context.Background(), key, secret)
	if err != nil {
		return nil, err
	}
	data, ok := secret.Data["config"]
	if !ok {
		return nil, errors.New("kubeconfig does not exist")
	}
	config, err := clientcmd.Load(data)
	if err != nil {
		return nil, err
	}
	// my-vcluster is the default name of cluster kubeconfig
	config.Clusters["my-vcluster"].Server = fmt.Sprintf("https://%s.%s:443", instance.Name, instance.Spec.Namespace)
	return config, nil
}

func (r *Reconciler) ReconcileEdgeWize(ctx context.Context, instance *infrav1alpha1.EdgeCluster, member *infrav1alpha1.Cluster) error {
	logger := log.FromContext(ctx, "ReconcileEdgeWize", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install edgewize agent")
		return nil
	}
	switch instance.Status.EdgeWize {
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus:
		values := chartutil.Values{}
		values["role"] = "member"
		status, err := InstallChart("edgewize", "edgewize", "edgewize-system", instance.Name, values)
		if err != nil {
			instance.Status.EdgeWize = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.EdgeWize = status
		if instance.Status.EdgeWize == infrav1alpha1.RunningStatus {
			member.Spec.Connection.KubeConfig = []byte(instance.Status.KubeConfig)
			err = r.Update(ctx, member)
			if err != nil {
				return err
			}
		}
		return nil
	case infrav1alpha1.ErrorStatus:
		logger.Info("edgewize install error")
	}
	return nil
}

func (r *Reconciler) ReconcileCloudCore(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "ReconcileCloudCore", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install cloudcore")
		return nil
	}
	switch instance.Status.CloudCore {
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus:
		values := chartutil.Values{}
		//cloudCore.modules.cloudHub.advertiseAddress=xxx
		values["cloudCore"] = map[string]interface{}{
			"modules": map[string]interface{}{
				"cloudHub": map[string]interface{}{
					"advertiseAddress": instance.Spec.AdvertiseAddress,
				},
			},
		}
		status, err := InstallChart("cloudcore", "cloudcore", "kubeedge", instance.Name, values)
		if err != nil {
			instance.Status.CloudCore = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.CloudCore = status
		return nil
	case infrav1alpha1.ErrorStatus:
		logger.Info("edgewize install error")
	}
	return nil
}

func (r *Reconciler) ReconcileFluentOperator(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "ReconcileCloudCore", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.Info("kubeconfig is null, skip install fluent-operator")
		return nil
	}
	switch instance.Status.FluentOperator {
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus:
		values := chartutil.Values{}
		status, err := InstallChart("fluent-operator", "fluent-operator", "fluent", instance.Name, values)
		if err != nil {
			instance.Status.CloudCore = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.CloudCore = status
		return nil
	case infrav1alpha1.ErrorStatus:
		logger.Info("fluent-operator install error")
	}
	return nil
}

func InstallChart(file, name, namespace, kubeconfig string, values chartutil.Values) (infrav1alpha1.Status, error) {
	chartStatus, err := helm.Status(file, name, namespace, kubeconfig)
	if err != nil {
		return "", err
	}
	switch chartStatus {
	case release.StatusUnknown, release.StatusUninstalled:
		err = helm.Install(file, name, namespace, kubeconfig, values)
		if err != nil {
			return "", err
		}
		return infrav1alpha1.InstallingStatus, nil
	case release.StatusUninstalling:
		return infrav1alpha1.UninstallingStatus, nil
	case release.StatusDeployed:
		return infrav1alpha1.RunningStatus, nil
	case release.StatusFailed:
		return infrav1alpha1.ErrorStatus, nil
	case release.StatusPendingInstall, release.StatusPendingUpgrade, release.StatusPendingRollback:
		return infrav1alpha1.InstallingStatus, nil
	}
	return infrav1alpha1.ErrorStatus, nil
}

func SaveToLocal(name string, config []byte) error {
	path := filepath.Join(homedir.HomeDir(), ".kube", name)
	return os.WriteFile(path, config, 0644)
}

func CheckKubeConfig(name string, config []byte) error {
	path := filepath.Join(homedir.HomeDir(), ".kube", name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = SaveToLocal(name, config)
		if err != nil {
			return err
		}
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if bytes.Compare(data, config) != 0 {
		err = SaveToLocal(name, config)
		if err != nil {
			return err
		}
	}
	return nil
}
