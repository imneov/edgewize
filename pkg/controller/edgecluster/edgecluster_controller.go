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
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/util/retry"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/helm"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chartutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
	ksclusterv1alpha1 "kubesphere.io/api/cluster/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	CurrentNamespace             = "edgewize-system"
	controllerName               = "edgecluster-controller"
	DefaultDistro                = "k3s"
	ComponentEdgeWize            = "edgewize"
	ComponentCloudCore           = "cloudcore"
	EdgeWizeValuesConfigName     = "edgewize-values-config"
	WhizardGatewayServiceName    = "gateway-whizard-operated"
	MonitorNamespace             = "kubesphere-monitoring-system"
	EdgeWizeServers              = "edgewize-servers.yaml"
	EdgeDeploySecret             = "edge-deploy-secret"
	MonitorPromServiceName       = "prometheus-k8s"
	WhizardEdgeGatewayConfigName = "whizard-edge-gateway-configmap"
)

var DefaultComponents = "edgewize,whizard-edge-agent,cloudcore,fluent-operator,ks-core,edge-ota-server"

func init() {
	if dc := os.Getenv("DEFAULT_COMPONENTS"); dc != "" {
		DefaultComponents = dc
	}
}

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
	err := os.MkdirAll(filepath.Join(homedir.HomeDir(), ".kube", "member"), 0644)
	if err != nil {
		klog.Error("create .kube directory error", err)
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
// +kubebuilder:rbac:groups=cluster.kubesphere.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.kubesphere.io,resources=clusters/status,verbs=get

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
			if err := r.UpdateEdgeCluster(rootCtx, req.NamespacedName, func(_instance *infrav1alpha1.EdgeCluster) error {
				_instance.ObjectMeta.Finalizers = append(_instance.ObjectMeta.Finalizers, finalizer)
				return r.Update(rootCtx, _instance)
			}); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if sliceutil.HasString(instance.ObjectMeta.Finalizers, finalizer) {
			if _, err := r.undoReconcile(ctx, instance); err != nil {
				logger.Error(err, "undoReconcile failed", "instance", instance.Name)
			}
			if err := r.UpdateEdgeCluster(rootCtx, req.NamespacedName, func(_instance *infrav1alpha1.EdgeCluster) error {
				// remove our finalizer from the list and update it.
				_instance.ObjectMeta.Finalizers = sliceutil.RemoveString(_instance.ObjectMeta.Finalizers, func(item string) bool {
					return item == finalizer
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

func (r *Reconciler) undoReconcile(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "func", "undoReconcile", "instance", instance.Name)
	logger.V(4).Info("delete edge cluster", "instance", instance)
	kubeconfig := instance.Status.ConfigFile
	switch instance.Status.Status {
	case infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		status, err := helm.Status(instance.Spec.Distro, instance.Name, instance.Spec.Namespace, kubeconfig)
		if err != nil {
			return ctrl.Result{}, err
		}
		switch status {
		case "deployed", "superseded", "failed", "pending-install", "pending-upgrade", "pending-rollback":
			logger.V(4).Info("begin uninstall edge cluster ", "status", status)
			instance.Status.Status = infrav1alpha1.UninstallingStatus
			err = helm.Uninstall(instance.Name, instance.Spec.Namespace, kubeconfig)
			if err != nil {
				logger.Error(err, "uninstall edge cluster error")
			} else {
				logger.V(4).Info("uninstall edge cluster success", "name", instance.Name)
			}
		}
	}
	member := &infrav1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
	}
	if err := r.Delete(ctx, member); err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "delete edge cluster error")
	}
	ksMember := &ksclusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
	}
	if err := r.Delete(ctx, ksMember); err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "delete kubesphere member cluster error")
	}

	err := r.CleanEdgeClusterResources(instance.Name, instance.Spec.Namespace, kubeconfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.Status().Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	err = r.UnregisterWhizardEdgeGatewayRouters(ctx, instance)
	if err != nil {
		logger.Error(err, "remove edge cluster edge gateway route cfg failed", "name", instance.Name)
	}
	return ctrl.Result{}, nil
}

func (r *Reconciler) doReconcile(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "doReconcile", instance.Name)
	if instance.Name == "" || instance.Spec.Namespace == "" {
		return ctrl.Result{}, errors.New("cluster name and namespace cannot be empty")
	}
	defer func() {
		if err := r.Status().Update(ctx, instance); err != nil {
			logger.Error(err, "update edge cluster status error")
		}
	}()
	if err := r.UpdateEdgeCluster(ctx, nn, func(_instance *infrav1alpha1.EdgeCluster) error {
		logger.V(3).Info("origin value", "distro", _instance.Spec.Distro, "components", _instance.Spec.Components, "advertiseaddress", _instance.Spec.AdvertiseAddress)
		if _instance.Spec.Distro == "" {
			_instance.Spec.Distro = DefaultDistro
		}
		if _instance.Spec.Type == "" {
			_instance.Spec.Type = infrav1alpha1.InstallTypeAuto
		}
		if _instance.Spec.Components == "" {
			_instance.Spec.Components = DefaultComponents
		} else if !strings.Contains(_instance.Spec.Components, ComponentEdgeWize) {
			_instance.Spec.Components = fmt.Sprintf("%s,%s", ComponentEdgeWize, _instance.Spec.Components)
		}
		if _instance.Spec.AdvertiseAddress == nil {
			_instance.Spec.AdvertiseAddress = []string{}
		}
		logger.V(3).Info("set default value", "distro", _instance.Spec.Distro, "components", _instance.Spec.Components, "advertiseaddress", _instance.Spec.AdvertiseAddress)
		_err := r.Update(ctx, _instance)
		if _err != nil {
			logger.V(2).Info("retry to update EdgeCluster", "err", _err.Error())
		}
		return _err
	}); err != nil {
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

	if err := r.Get(ctx, nn, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	switch instance.Spec.Type {
	case infrav1alpha1.InstallTypeAuto:
		if instance.Status.Status != infrav1alpha1.RunningStatus {
			err := r.InstallEdgeCluster(ctx, instance)
			if err != nil {
				logger.Error(err, "install cluster error", "name", instance.Name, "distro", instance.Spec.Distro)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
	case infrav1alpha1.InstallTypeManual:
		if instance.Status.Status != infrav1alpha1.RunningStatus {
			instance.Status.Status = infrav1alpha1.RunningStatus
			instance.Status.KubeConfig = string(instance.Spec.KubeConfig)
		}
	}
	if instance.Status.KubeConfig == "" {
		config, err := r.GetKubeConfig(instance)
		if err != nil {
			if apierrors.IsNotFound(err) {
				logger.Info("edge cluster kube config not found, retry after 3s")
				return ctrl.Result{RequeueAfter: time.Second * 3}, nil
			}
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
	err = SaveToLocal(instance.Name, []byte(instance.Status.KubeConfig))
	if err != nil {
		logger.Error(err, "write edge cluster kube config to file error")
		return ctrl.Result{}, err
	}
	err = r.InstallEdgeClusterComponents(ctx, instance)
	if err != nil {
		logger.Error(err, "install edge cluster components error")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) InstallEdgeCluster(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "InstallEdgeCluster", instance.Name)
	needCreateNS := true
	kubeconfig, err := r.LoadExternalKubeConfig(ctx, instance.Spec.Namespace)
	if err != nil {
		return err
	}
	// 如果配置了外部Namespace，则不需要创建命名空间
	if kubeconfig != "" {
		needCreateNS = false
	}
	// 获取 member 集群 kubeconfig
	if kubeconfig == "" && instance.Spec.HostCluster != "host" {
		kubeconfig, err = r.LoadMemberKubeConfig(ctx, instance.Spec.HostCluster)
		if err != nil {
			return err
		}
	}
	logger.V(1).Info("install edge cluster", "name", instance.Name)
	values, err := r.GetValuesFromConfigMap(ctx, "vcluster")
	if err != nil {
		logger.Error(err, "get vcluster values error, use default")
		values = map[string]interface{}{}
	}
	nsExisted := r.IsNamespaceExisted(ctx, kubeconfig, instance.Spec.Namespace)
	createNamespace := needCreateNS && !nsExisted
	if instance.Annotations == nil {
		instance.Annotations = make(map[string]string)
	}
	status, err := InstallChart(instance.Spec.Distro, instance.Name, instance.Spec.Namespace, kubeconfig, createNamespace, values)
	if err != nil {
		logger.Error(err, "install edge cluster error")
		return err
	}
	instance.Status.Status = status
	instance.Status.ConfigFile = kubeconfig
	return nil
}

func (r *Reconciler) InstallEdgeClusterComponents(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "InstallEdgeClusterComponents", instance.Name)
	components := strings.Split(instance.Spec.Components, ",")
	var err error
	for _, component := range components {
		if len(component) > 0 && component[0] != '-' {
			switch component {
			case "ks-core":
				err = r.ReconcileKSCore(ctx, instance)
				if err != nil {
					logger.Error(err, "install ks-core error")
					return err
				}
			case "edgewize":
				err = r.ReconcileEdgeWize(ctx, instance)
				if err != nil {
					logger.Error(err, "install edgewize agent error")
					return err
				}
			case "whizard-edge-agent":
				err = r.ReconcileWhizardEdgeAgent(ctx, instance)
				if err != nil {
					logger.Error(err, "install whizard-edge-agent error")
					return err
				}
			case "cloudcore":
				err = r.ReconcileCloudCore(ctx, instance)
				if err != nil {
					logger.Error(err, "install cloudcore error")
					return err
				}
			case "fluent-operator":
				err = r.ReconcileFluentOperator(ctx, instance)
				if err != nil {
					logger.Error(err, "install cloudcore error")
					return err
				}
			case "edge-ota-server":
				err = r.ReconcileEdgeOtaServer(ctx, instance)
				if err != nil {
					logger.Error(err, "edge-ota-server error")
					return err
				}
			default:
				logger.Info(fmt.Sprintf("unknown component %s", component))
			}
		}
	}
	return nil
}

func (r *Reconciler) UpdateEdgeCluster(ctx context.Context, nn types.NamespacedName, updateFunc func(*infrav1alpha1.EdgeCluster) error) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		instance := &infrav1alpha1.EdgeCluster{}
		if err := r.Get(ctx, nn, instance); err != nil {
			return client.IgnoreNotFound(err)
		}
		return updateFunc(instance)
	})
}

func (r *Reconciler) GetKubeConfig(instance *infrav1alpha1.EdgeCluster) (*clientcmdapi.Config, error) {
	// 读取外部 kubeconfig 创建 k8s client
	file := filepath.Join(homedir.HomeDir(), ".kube", "external", instance.Spec.Namespace)
	secret := &corev1.Secret{}
	service := &corev1.Service{}
	// 如果配置了外部 kubeconfig 则使用外部 kubeconfig，否则使用当前集群 kubeconfig
	if _, err := os.Stat(file); err == nil {
		config, err := clientcmd.BuildConfigFromFlags("", file)
		if err != nil {
			return nil, err
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}

		secret, err = clientset.CoreV1().Secrets(instance.Spec.Namespace).Get(context.Background(), fmt.Sprintf("vc-%s", instance.Name), metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		service, err = clientset.CoreV1().Services(instance.Spec.Namespace).Get(context.Background(), instance.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		key := types.NamespacedName{
			Namespace: instance.Spec.Namespace,
			Name:      fmt.Sprintf("vc-%s", instance.Name),
		}
		err = r.Get(context.Background(), key, secret)
		if err != nil {
			return nil, err
		}
		key.Name = instance.Name
		err = r.Get(context.Background(), key, service)
		if err != nil {
			return nil, err
		}
	}
	data, ok := secret.Data["config"]
	if !ok {
		return nil, errors.New("kubeconfig does not exist")
	}
	config, err := clientcmd.Load(data)
	if err != nil {
		return nil, err
	}
	cluster := config.Contexts[config.CurrentContext].Cluster
	if service.Spec.Ports != nil && len(service.Spec.Ports) > 0 {
		config.Clusters[cluster].Server = fmt.Sprintf("https://%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port)
	} else {
		return config, errors.New("edge cluster service port does not exist")
	}
	return config, nil
}

func (r *Reconciler) ReconcileWhizardEdgeAgent(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "ReconcileWhizardEdgeAgent", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install whizard-edge-agent")
		return nil
	}

	switch instance.Status.EdgeWize {
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		err := r.InitImagePullSecret(ctx, instance, instance.Name, MonitorNamespace)
		if err != nil {
			return err
		}
		values, err := r.GetValuesFromConfigMap(ctx, "whizard-edge-agent")
		if err != nil {
			logger.Error(err, "get vcluster values error, use default")
			values = map[string]interface{}{}
		}
		err = r.SetMonitorComponent(ctx, values, instance)
		if err != nil {
			logger.Error(err, "get gateway svc ip error, need to configure manually")
		}
		klog.V(3).Infof("ReconcileWhizardEdgeAgent: %v", values)
		status, err := InstallChart("whizard-edge-agent", "whizard-edge-agent", MonitorNamespace, instance.Name, true, values)
		if err != nil {
			logger.Error(err, "install whizard-edge-agent error")
			instance.Status.EdgewizeMonitor = infrav1alpha1.ErrorStatus
			return err
		}
		err = r.RegisterWhizardEdgeGatewayRouters(ctx, instance.Name, instance)
		if err != nil {
			logger.Error(err, "update whizard edge gateway config failed")
			instance.Status.EdgewizeMonitor = infrav1alpha1.ErrorStatus
		} else {
			instance.Status.EdgewizeMonitor = status
		}
		return nil
	}
	return nil
}
func (r *Reconciler) ReconcileEdgeOtaServer(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "ReconcileEdgeOtaServer", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install edge-ota-server")
		return nil
	}

	switch instance.Status.EdgeWize {
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		namespace := "kubeedge"
		err := r.InitImagePullSecret(ctx, instance, instance.Name, namespace)
		if err != nil {
			return err
		}
		values, err := r.GetValuesFromConfigMap(ctx, "edge-ota-server")
		if err != nil {
			logger.Error(err, "get vcluster values error, use default")
			values = map[string]interface{}{}
		}
		klog.V(3).Infof("ReconcileEdgeOtaServer: %v", values)
		status, err := InstallChart("edge-ota-server", "edge-ota-server", namespace, instance.Name, true, values)
		if err != nil {
			logger.Error(err, "install edge-ota-server error")
			instance.Status.EdgeOtaServer = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.EdgeOtaServer = status
		return nil
	}
	return nil
}

func (r *Reconciler) ReconcileKSCore(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "ReconcileKSCore", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install ks-core")
		return nil
	}
	switch instance.Status.KSCore {
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		namespace := "kubesphere-system"
		err := r.InitImagePullSecret(ctx, instance, instance.Name, namespace)
		if err != nil {
			logger.Error(err, "init image pull secret error, use default")
			return err
		}
		values, err := r.GetValuesFromConfigMap(ctx, "ks-core")
		if err != nil {
			logger.Error(err, "get ks-core values error, use default")
			values = map[string]interface{}{}
		}
		err = r.SetKSCoreValues(ctx, values)
		if err != nil {
			logger.Error(err, "set ks-core values error, skip")
		}
		klog.V(3).Infof("ReconcileKSCore: %v", values)
		status, err := InstallChart("ks-core", "ks-core", namespace, instance.Name, true, values)
		if err != nil {
			logger.Error(err, "install ks-core error")
			instance.Status.KSCore = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.KSCore = status
		if instance.Status.KSCore == infrav1alpha1.RunningStatus {
			member := &ksclusterv1alpha1.Cluster{}
			err = r.Get(ctx, types.NamespacedName{Name: instance.Name}, member)
			if err != nil {
				if apierrors.IsNotFound(err) {
					member = &ksclusterv1alpha1.Cluster{
						ObjectMeta: metav1.ObjectMeta{
							Name: instance.Name,
							Annotations: map[string]string{
								"kubesphere.io/creator": "admin",
							},
							Labels: map[string]string{
								"cluster-role.kubesphere.io/edge": "",
							},
						},
						Spec: ksclusterv1alpha1.ClusterSpec{
							JoinFederation: true,
							Connection: ksclusterv1alpha1.Connection{
								Type:       ksclusterv1alpha1.ConnectionTypeDirect,
								KubeConfig: []byte(instance.Status.KubeConfig),
							},
						},
					}
					err := r.Create(ctx, member)
					if err != nil {
						logger.Error(err, "create kubesphere member cluster error")
						return err
					}
					klog.V(4).Infof("crete kubesphere member cluster success, name: %s", instance.Name)
				} else {
					logger.Error(err, "get kubesphere member cluster error")
					return err
				}
			}
		}
		return nil
	}
	return nil
}

func (r *Reconciler) ReconcileEdgeWize(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "ReconcileEdgeWize", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install edgewize agent")
		return nil
	}
	switch instance.Status.EdgeWize {
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		namespace := CurrentNamespace
		err := r.InitImagePullSecret(ctx, instance, instance.Name, namespace)
		if err != nil {
			return err
		}
		values, err := r.GetValuesFromConfigMap(ctx, "edgewize")
		if err != nil {
			logger.Error(err, "get vcluster values error, use default")
			values = map[string]interface{}{}
		}
		values["role"] = "member"
		klog.V(3).Infof("ReconcileEdgeWize: %v", values)
		status, err := InstallChart("edgewize", "edgewize", namespace, instance.Name, true, values)
		if err != nil {
			logger.Error(err, "install edgewize error")
			instance.Status.EdgeWize = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.EdgeWize = status
		if instance.Status.EdgeWize == infrav1alpha1.RunningStatus {
			// 等待 edgewize running 后再更新 kubeconfig，否则前端边缘集群显示 running，进入集群页面会报错
			err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				edge := &infrav1alpha1.Cluster{}
				key := types.NamespacedName{Name: instance.Name}
				err := r.Get(ctx, key, edge)
				if err != nil {
					logger.Error(err, "get edge cluster error")
					return err
				}
				edge.Spec.Connection.KubeConfig = []byte(instance.Status.KubeConfig)
				err = r.Update(ctx, edge)
				if err != nil {
					logger.Error(err, "update edge cluster error")
					return err
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
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
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		namespace := "kubeedge"
		err := r.InitImagePullSecret(ctx, instance, instance.Name, namespace)
		if err != nil {
			return err
		}
		err = r.InitCloudCoreCert(ctx, instance, instance.Name, namespace)
		if err != nil {
			klog.Warning("init cloudhub certs error, use default", err)
		}
		values, err := r.GetValuesFromConfigMap(ctx, "cloudcore")
		if err != nil {
			klog.Warning("get vcluster values error, use default", err)
			values = map[string]interface{}{}
		}
		err = SetCloudCoreValues(values, instance)
		if err != nil {
			klog.Warningf("set cloudcore values error, err: %v", err)
		}
		klog.V(3).Infof("ReconcileCloudCore: %v", values)
		status, err := InstallChart("cloudcore", "cloudcore", namespace, instance.Name, true, values)
		if err != nil {
			klog.Warning("install cloudcore error, will try again at the next Reconcile.", "error", err)
			instance.Status.CloudCore = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.CloudCore = status
		if status == infrav1alpha1.RunningStatus {
			err := r.UpdateCloudCoreService(ctx, instance.Name, "kubeedge", instance)
			if err != nil {
				logger.Info("update edgewize-cloudcore-service error", "error", err)
			}
		}
		return nil
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
	case "", infrav1alpha1.InstallingStatus, infrav1alpha1.RunningStatus, infrav1alpha1.ErrorStatus:
		namespace := "fluent"
		err := r.InitImagePullSecret(ctx, instance, instance.Name, namespace)
		if err != nil {
			return err
		}
		values, err := r.GetValuesFromConfigMap(ctx, "fluent-operator")
		if err != nil {
			logger.Error(err, "get vcluster values error, use default")
			values = map[string]interface{}{}
		}
		klog.V(3).Infof("ReconcileFluentOperator: %v", values)
		err = r.SetClusterOutput(values, instance)
		if err != nil {
			logger.Error(err, "configure ClusterOutput failed, skip install fluent-operator")
			return nil
		}
		status, err := InstallChart("fluent-operator", "fluent-operator", namespace, instance.Name, true, values)
		if err != nil {
			logger.Error(err, "install fluent-operator error")
			instance.Status.FluentOperator = infrav1alpha1.ErrorStatus
			return err
		}
		instance.Status.FluentOperator = status
		return nil
	}
	return nil
}

func (r *Reconciler) SetKSCoreValues(ctx context.Context, values chartutil.Values) error {
	ksConfig := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Name:      "kubesphere-config",
		Namespace: "kubesphere-system",
	}
	err := r.Get(ctx, key, ksConfig)
	if err != nil {
		return err
	}
	if data, ok := ksConfig.Data["kubesphere.yaml"]; ok {
		value, err := chartutil.ReadValues([]byte(data))
		if err != nil {
			klog.Errorf("parse kubesphere.yaml error, err: %v", err)
			return err
		}
		jwt, err := value.PathValue("authentication.jwtSecret")
		if err != nil {
			klog.Errorf("get jwtSecret error, err: %v", err)
			return err
		}
		configInterface := values["config"]
		config := make(map[string]interface{})
		if configInterface != nil {
			config = configInterface.(map[string]interface{})
		}
		config["jwtSecret"] = jwt
	}
	return nil
}

func SetCloudCoreValues(values chartutil.Values, instance *infrav1alpha1.EdgeCluster) error {
	if len(instance.Spec.AdvertiseAddress) > 0 {
		cloudcore := values["cloudCore"].(map[string]interface{})
		modules := cloudcore["modules"].(map[string]interface{})
		cloudHub := modules["cloudHub"].(map[string]interface{})
		cloudHub["advertiseAddress"] = instance.Spec.AdvertiseAddress
		values["edgeClusterName"] = instance.Name
	}
	return nil
}

// edge node send data to whizard edge gateway
func (r *Reconciler) SetClusterOutput(values chartutil.Values, instance *infrav1alpha1.EdgeCluster) (err error) {
	port, err := values.PathValue("fluentbit.kubeedge.prometheusRemoteWrite.port")
	if err != nil {
		return err
	}

	host, err := values.PathValue("fluentbit.kubeedge.prometheusRemoteWrite.host")
	if err != nil {
		return err
	}

	enabled, err := values.PathValue("fluentbit.kubeedge.enable")
	if err != nil {
		return err
	}

	fluentbitConf, err := values.Table("fluentbit")
	if err != nil {
		return err
	}

	fluentbitConfMap := fluentbitConf.AsMap()
	fluentbitConfMap["kubeedge"] = map[string]interface{}{
		"enable": enabled,
		"prometheusRemoteWrite": map[string]interface{}{
			"host":      host,
			"port":      port,
			"routerKey": instance.Name,
		},
	}
	return nil
}

func (r *Reconciler) SetMonitorComponent(ctx context.Context, values chartutil.Values, instance *infrav1alpha1.EdgeCluster) (err error) {
	gatewayService := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: MonitorNamespace,
		Name:      WhizardGatewayServiceName,
	}
	err = r.Get(ctx, key, gatewayService)
	if err != nil {
		return
	}

	whizardAgentConf := map[string]interface{}{"tenant": instance.Name}
	if gatewayService.Spec.Ports != nil && len(gatewayService.Spec.Ports) > 0 {
		gatewayIP := gatewayService.Spec.ClusterIP
		gatewayPort := gatewayService.Spec.Ports[0].Port
		whizardAgentConf["gateway_address"] = fmt.Sprintf("http://%s:%d", gatewayIP, gatewayPort)
	}
	values["whizard_agent_proxy"] = whizardAgentConf
	return
}

func (r *Reconciler) RegisterWhizardEdgeGatewayRouters(ctx context.Context, kubeconfig string, instance *infrav1alpha1.EdgeCluster) (err error) {
	file := filepath.Join(homedir.HomeDir(), ".kube", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		klog.Error("create rest config from kubeconfig string error", err.Error())
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("create k8s client from restconfig error", err.Error())
		return
	}

	promService, err := clientset.CoreV1().
		Services(MonitorNamespace).
		Get(ctx, MonitorPromServiceName, metav1.GetOptions{})
	if err != nil {
		return
	}

	var routePath string
	if promService.Spec.ClusterIP != "" {
		for _, item := range promService.Spec.Ports {
			if item.Name == "web" {
				routePath = fmt.Sprintf("http://%s:%d/api/v1/write", promService.Spec.ClusterIP, item.Port)
				break
			}
		}
	}

	if routePath == "" {
		err = errors.New("create routePath for whizard edge gateway failed")
		return
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		edgeGatewayConfigMap := &corev1.ConfigMap{}
		key := types.NamespacedName{
			Namespace: MonitorNamespace,
			Name:      WhizardEdgeGatewayConfigName,
		}
		err = r.Get(ctx, key, edgeGatewayConfigMap)
		if err != nil {
			return
		}

		cmFile, ok := edgeGatewayConfigMap.Data["config.yaml"]
		if !ok {
			err = errors.New("whizard edge configmap is empty")
			return
		}

		cmData := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(cmFile), cmData)
		if err != nil {
			return err
		}

		routersMap := make(map[string]interface{})
		value, ok := cmData["routers"]
		if ok {
			routersMap = value.(map[string]interface{})
		}

		routersMap[instance.Name] = routePath
		cmData["routers"] = routersMap

		newCfgFile, err := yaml.Marshal(cmData)
		if err != nil {
			return
		}

		edgeGatewayConfigMap.Data["config.yaml"] = string(newCfgFile)
		err = r.Update(ctx, edgeGatewayConfigMap)
		return
	})
	return
}

func (r *Reconciler) UnregisterWhizardEdgeGatewayRouters(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (err error) {
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		edgeGatewayConfigMap := &corev1.ConfigMap{}
		key := types.NamespacedName{
			Namespace: MonitorNamespace,
			Name:      WhizardEdgeGatewayConfigName,
		}
		err = r.Get(ctx, key, edgeGatewayConfigMap)
		if err != nil {
			return
		}

		EdgeGatewayCM := "config.yaml"
		cmFile, ok := edgeGatewayConfigMap.Data[EdgeGatewayCM]
		if !ok {
			err = errors.New("whizard edge configmap is empty")
			return
		}

		cmData := make(map[string]interface{})
		err = yaml.Unmarshal([]byte(cmFile), cmData)
		if err != nil {
			return err
		}

		routersMap := make(map[string]interface{})
		value, ok := cmData["routers"]
		if ok {
			routersMap = value.(map[string]interface{})
		}

		delete(routersMap, instance.Name)
		cmData["routers"] = routersMap

		newCfgFile, err := yaml.Marshal(cmData)
		if err != nil {
			return
		}

		edgeGatewayConfigMap.Data[EdgeGatewayCM] = string(newCfgFile)
		err = r.Update(ctx, edgeGatewayConfigMap)
		return
	})
	return
}

// LoadExternalKubeConfig 从 ConfigMap 加载外部的 kubeconfig 保存到本地目录
func (r *Reconciler) LoadExternalKubeConfig(ctx context.Context, name string) (string, error) {
	path := filepath.Join("external", name)
	file := filepath.Join(homedir.HomeDir(), ".kube", path)
	if _, err := os.Stat(file); err == nil {
		return path, nil
	}
	return "", nil
}

// LoadMemberKubeConfig 获取 member 集群的 kubeconfig 并保存到本地目录
func (r *Reconciler) LoadMemberKubeConfig(ctx context.Context, name string) (string, error) {
	path := filepath.Join("member", name)
	file := filepath.Join(homedir.HomeDir(), ".kube", path)
	if _, err := os.Stat(file); err == nil {
		return path, err
	}
	cluster := &ksclusterv1alpha1.Cluster{}
	key := types.NamespacedName{
		Name: name,
	}
	err := r.Get(ctx, key, cluster)
	if err != nil {
		return "", client.IgnoreNotFound(err)
	}
	err = SaveToLocal(path, cluster.Spec.Connection.KubeConfig)
	if err != nil {
		return "", err
	}
	return path, nil
}

func (r *Reconciler) CleanEdgeClusterResources(name, namespace, kubeconfig string) error {
	file := filepath.Join(homedir.HomeDir(), ".kube", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		klog.Error("create rest config from kubeconfig string error", err.Error())
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("create clientset from config error", err.Error())
		return err
	}

	err = clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), fmt.Sprintf("data-%s-0", name), metav1.DeleteOptions{})
	if err != nil {
		klog.Error("delete edgecluster pvc error", fmt.Sprintf("data-%s-0", name), err.Error())
		return err
	}
	err = clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), fmt.Sprintf("data-%s-etcd-0", name), metav1.DeleteOptions{})
	if err != nil {
		klog.Error("delete edgecluster pvc error", fmt.Sprintf("data-%s-etcd-0", name), err.Error())
		return err
	}
	return nil
}

func (r *Reconciler) GetValuesFromConfigMap(ctx context.Context, component string) (chartutil.Values, error) {
	configmap := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
		Name:      EdgeWizeValuesConfigName,
	}
	err := r.Get(ctx, key, configmap)
	if err != nil {
		return nil, err
	}
	values := make(map[string]interface{})
	if content := configmap.Data[component]; content != "" {
		strings.Replace(content, "$name", component, 1) // TODO
		values, err = chartutil.ReadValues([]byte(content))
		if err != nil {
			return nil, err
		}
		return values, nil
	}
	return values, nil
}

func (r *Reconciler) IsNamespaceExisted(ctx context.Context, kubeconfig, namespace string) bool {
	file := filepath.Join(homedir.HomeDir(), ".kube", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		klog.Error("create rest config from kubeconfig string error", err.Error())
		return true
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("create k8s client from restconfig error", err.Error())
		return true
	}
	_, err = clientset.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false
		}
		klog.Error("get namespace error", err.Error())
		return true
	}
	return true
}

type ServiceMap map[string]corev1.ServiceSpec

func (r *Reconciler) UpdateCloudCoreService(ctx context.Context, kubeconfig, namespace string, instance *infrav1alpha1.EdgeCluster) error {
	file := filepath.Join(homedir.HomeDir(), ".kube", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		klog.Error("create rest config from kubeconfig string error", err.Error())
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("create clientset from config error", err.Error())
		return err
	}

	// TODO kubeedge -> instance.Spec.Namespace
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, "cloudcore", metav1.GetOptions{})
	if err != nil {
		klog.Error("get service cloudcore error ", err.Error())
		return err
	}
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: "edgewize-system",
		Name:      "edgewize-cloudcore-service",
	}
	err = r.Get(ctx, key, cm)
	if err != nil {
		klog.Error("get configmap edgewize-cloudcore-service error ", err.Error())
		return err
	}
	svcMap := ServiceMap{}
	if data, ok := cm.Data[EdgeWizeServers]; ok {
		err = yaml.Unmarshal([]byte(data), &svcMap)
		if err != nil {
			klog.Errorf("invalid %s, err:%v", EdgeWizeServers, err)
			cm.Data = make(map[string]string) // TODO
		}
	}
	svcMap[instance.Name] = svc.Spec
	data, err := yaml.Marshal(svcMap)
	if err != nil {
		klog.Error("Marshal svc.Spec error", err.Error())
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[EdgeWizeServers] = string(data)
	err = r.Update(ctx, cm)
	if err != nil {
		klog.Error("update edgewize-cloudcore-service configmap error ", err.Error())
		return err
	}
	return nil
}

func (r *Reconciler) DeleteCloudCoreService(ctx context.Context, kubeconfig, namespace string, instance *infrav1alpha1.EdgeCluster) error {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: "edgewize-system",
		Name:      "edgewize-cloudcore-service",
	}
	err := r.Get(ctx, key, cm)
	if err != nil {
		klog.Error("get configmap edgewize-cloudcore-service error ", err.Error())
		return err
	}
	svcMap := ServiceMap{}
	if data, ok := cm.Data[EdgeWizeServers]; ok {
		err = yaml.Unmarshal([]byte(data), &svcMap)
		if err != nil {
			klog.Errorf("invalid %s, err:%v", EdgeWizeServers, err)
			cm.Data = make(map[string]string) // TODO
		}
	}
	klog.V(3).Infof("delete cloudcore service: %s", instance.Name)
	delete(svcMap, instance.Name)
	data, err := yaml.Marshal(svcMap)
	if err != nil {
		klog.Error("Marshal svc.Spec error", err.Error())
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[EdgeWizeServers] = string(data)
	err = r.Update(ctx, cm)
	if err != nil {
		klog.Error("update edgewize-cloudcore-service configmap error ", err.Error())
		return err
	}
	return nil
}

func (r *Reconciler) InitCloudCoreCert(ctx context.Context, instance *infrav1alpha1.EdgeCluster, kubeconfig string, namespace string) error {
	file := filepath.Join(homedir.HomeDir(), ".kube", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		klog.Error("create rest config from kubeconfig string error", err.Error())
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("create clientset from config error", err.Error())
		return err
	}

	cloudhub, err := clientset.CoreV1().Secrets(namespace).Get(ctx, "cloudhub", metav1.GetOptions{})
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			klog.Error("get secret cloudhub error", err.Error())
			return err
		}
	} else {
		klog.Infof("secret cloudhub exists, skip. cloudhub: %v", cloudhub.String())
		return nil
	}

	rootca := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      "edgewize-root-ca",
		Namespace: CurrentNamespace,
	}
	err = r.Get(ctx, key, rootca)
	if err != nil {
		klog.Warningf("get secret edgewize-root-ca error: %v", err)
		return err
	}
	klog.V(3).Infof("rootca: %v", rootca)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			klog.Errorf("create namespace %s error: %v", namespace, err)
			return err
		}
	}
	var caCrt []byte
	var caKey []byte

	if rootca.Data == nil {
		klog.Errorf("secret rootca is invalided")
		return fmt.Errorf("invalid rootca secret")
	}
	// name is empty means not found
	var ok bool
	caCrtPem, ok := rootca.Data["cacrt"]
	if !ok {
		klog.Errorf("secret root ca cert is invalided")
		return fmt.Errorf("invalid rootca secret")
	}
	if caCrtBlock, _ := pem.Decode(caCrtPem); caCrtBlock == nil {
		klog.Errorf("pem decode root ca cert error")
		return fmt.Errorf("invalid rootca secret")
	} else {
		caCrt = caCrtBlock.Bytes
	}

	caKeyPem, ok := rootca.Data["cakey"]
	if !ok {
		klog.Errorf("secret root ca key is invalided")
		return fmt.Errorf("invalid rootca secret")
	}
	if caKeyBlock, _ := pem.Decode(caKeyPem); caKeyBlock == nil {
		klog.Errorf("pem decode root ca key error")
		return fmt.Errorf("invalid rootca secret")
	} else {
		caKey = caKeyBlock.Bytes
	}

	klog.V(5).Infof("root CA content, crt: %s, key: %s", base64.StdEncoding.EncodeToString(caCrt), base64.StdEncoding.EncodeToString(caKey))
	serverCrt, serverKey, err := SignCloudCoreCert(caCrt, caKey)
	if err != nil {
		klog.Error("create cloudhub cert error", err)
		return err
	}
	cloudhub = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudhub",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"rootCA.crt": pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: caCrt}),
			"rootCA.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: caKey}),
			"server.crt": pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: serverCrt}),
			"server.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: serverKey}),
		},
	}
	klog.V(3).Infof("secret cloudhub content: %s", cloudhub.String())
	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, cloudhub, metav1.CreateOptions{})
	if err != nil {
		klog.Error("create secret cloudhub error", err)
		return err
	}
	return nil
}

func (r *Reconciler) InitImagePullSecret(ctx context.Context, instance *infrav1alpha1.EdgeCluster, kubeconfig string, namespace string) error {
	file := filepath.Join(homedir.HomeDir(), ".kube", kubeconfig)
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		klog.Error("create rest config from kubeconfig string error", err.Error())
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("create clientset from config error", err.Error())
		return err
	}

	imagePullSecret := &corev1.Secret{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
		Name:      EdgeDeploySecret,
	}
	err = r.Get(ctx, key, imagePullSecret)
	if err != nil {
		klog.Error("get image-pull-secret error", err.Error())
		return client.IgnoreNotFound(err)
	}

	edgeDeploySecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, EdgeDeploySecret, metav1.GetOptions{})
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			klog.Error("get secret zpk-deploy-secret error", err.Error())
			return err
		}
	} else {
		klog.V(3).Infof("secret edge-deploy-secret exists, skip. edge-deploy-secret: %v", edgeDeploySecret.String())
		return nil
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}
	// try to create namespace
	_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			klog.Errorf("create namespace %s error: %s", namespace, err.Error())
			return err
		}
	}
	imagePullSecret.ObjectMeta.Namespace = namespace
	imagePullSecret.ObjectMeta.UID = ""
	imagePullSecret.ObjectMeta.ResourceVersion = ""
	imagePullSecret.ObjectMeta.CreationTimestamp = metav1.NewTime(time.Time{})
	klog.V(3).Infof("secret edge-deploy-secret content: %s", imagePullSecret.String())
	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, imagePullSecret, metav1.CreateOptions{})
	if err != nil {
		klog.Error("create secret edge-deploy-secret error", err)
		return err
	}
	return nil
}
