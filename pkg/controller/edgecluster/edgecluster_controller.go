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
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/util/retry"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/helm"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	ksclusterv1alpha1 "kubesphere.io/api/cluster/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	NotFoundError            = errors.New("not found")
	InstallEdgeClusterError  = errors.New("Install edge cluster error")
	ErrEdgeClusterNil        = errors.New("EdgeCluster is nil")
	ErrEdgeClusterK8sAddress = errors.New("Get EdgeCluster k8s apiserver error")
)

const (
	K8S_GATEWAY_PORT = "K8S_GATEWAY_PORT"
)

type updateEdgeClusterFunc func(*infrav1alpha1.EdgeCluster) (err error)

// EdgeClusterOperator Define for unit test
type EdgeClusterOperator interface {
	//Update(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error
	UpdateEdgeCluster(ctx context.Context, nn types.NamespacedName, updateFunc updateEdgeClusterFunc) error
	UpdateEdgeClusterStatus(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error
	verifyEdgeCluster(ctx context.Context, name types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error
	createKSClusterCR(ctx context.Context, name types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error
	installEdgeCluster(ctx context.Context, name types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error
	prepareEdgeCluster(ctx context.Context, name types.NamespacedName, instance *infrav1alpha1.EdgeCluster) (kubeconfig []byte, err error)
	initEdgeCluster(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (err error)
	installComponents(ctx context.Context, name types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error
	updateServices(ctx context.Context, name types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error
	needUpgrade(instance *infrav1alpha1.EdgeCluster) bool
	prepareNamespace(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error
	LoadExternalKubeConfigAbs(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (string, error)
}

var ClusterComponentName = "vcluster"

type NameNamespace struct {
	name      string
	namespace string
}

var SystemNamespaces = []string{metav1.NamespaceSystem, metav1.NamespacePublic, corev1.NamespaceNodeLease}
var ExtendedNamespaces = []string{"kubesphere-controls-system"}

var ComponentsNamespaces = []NameNamespace{
	{"edgewize", "edgewize-system"},
	{"whizard-edge-agent", "kubesphere-monitoring-system"},
	{"cloudcore", "kubeedge"},
	{"fluent-operator", "fluent"},
	{"ks-core", "kubesphere-system"},
	{"edge-ota-server", "edgewize-system"},
	{"kubefed", "kube-federation-system"},
	{"eventbus", "edgewize-system"},
	{"router-manager", "edgewize-system"},
	{"modelmesh", "edgewize-system"},
	{"hami-device-plugin", "hami-device"},
	{"hami-scheduler", "hami"},
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	klog.V(0).Info("SetupWithManager1")
	klog.V(3).Info("SetupWithManager13")
	klog.V(5).Info("SetupWithManager15")
	klog.V(8).Info("SetupWithManager18")
	ctrl.Log.WithName("controllers").WithName(controllerName).V(0).Info("SetupWithManager2", "controllerName", controllerName)
	fmt.Println(ctrl.Log)
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
		r.MaxConcurrentReconciles = 3
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
	logger := r.Logger.WithValues("clusterName", req.NamespacedName.Name)
	logger.V(4).Info("receive request", "req", req)
	rootCtx := context.Background()
	instance := &infrav1alpha1.EdgeCluster{}
	if err := r.Get(rootCtx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// make different edge cluster reconcile concurrently
	if r.IsLock(instance.Name) {
		logger.V(3).Info("[+]edge cluster is reconciling, retry after 5s", "cluster", instance.Name)
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	r.Lock(instance.Name)
	defer r.Unlock(instance.Name)

	// name of your custom finalizer
	finalizer := "edgeclusterfinalizer.infra.edgewize.io"

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !sliceutil.HasString(instance.ObjectMeta.Finalizers, finalizer) {
			logger.V(4).Info("edge cluster is created, add finalizer and update", "req", req, "finalizer", finalizer)
			if err := r.UpdateEdgeCluster(rootCtx, req.NamespacedName, func(_instance *infrav1alpha1.EdgeCluster) (err error) {
				_instance.ObjectMeta.Finalizers = append(_instance.ObjectMeta.Finalizers, finalizer)
				return nil
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
			if err := r.UpdateEdgeCluster(rootCtx, req.NamespacedName, func(_instance *infrav1alpha1.EdgeCluster) (err error) {
				// remove our finalizer from the list and update it.
				_instance.ObjectMeta.Finalizers = sliceutil.RemoveString(_instance.ObjectMeta.Finalizers, func(item string) bool {
					return item == finalizer
				})
				return nil
			}); err != nil {
				logger.Error(err, "update edge cluster failed")
				return ctrl.Result{}, err
			}
		}
		// Our finalizer has finished, so the reconciler can do nothing.
		return ctrl.Result{}, nil
	}

	return doReconcile(ctx, r, r.Logger, req.NamespacedName, instance)
}

func (r *Reconciler) IsLock(cluster string) bool {
	_, ok := r.Instances.Load(cluster)
	return ok
}

func (r *Reconciler) Lock(cluster string) {
	r.Instances.Store(cluster, struct{}{})
}

func (r *Reconciler) Unlock(cluster string) {
	r.Instances.Delete(cluster)
}

func (r *Reconciler) undoReconcileEdgeCluster(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := r.Logger.WithValues("edgecluster", instance.Name)
	extKubeConfig, err := r.LoadExternalKubeConfig(ctx, instance)
	status, err := helm.Status(instance.Spec.Distro, instance.Name, instance.Spec.Namespace, extKubeConfig)
	if err != nil {
		logger.Error(err, "get edge cluster status error", "name", instance.Name,
			"Namespace", instance.Spec.Namespace, "extKubeConfig", extKubeConfig)
		return err
	}
	switch status {
	case "deployed", "superseded", "failed", "pending-install", "pending-upgrade", "pending-rollback":
		logger.V(4).Info("begin uninstall edge cluster ", "status", status)
		err = helm.Uninstall(instance.Name, instance.Spec.Namespace, extKubeConfig)
		if err != nil {
			logger.Error(err, "get edge cluster status error", "name", instance.Name,
				"Namespace", instance.Spec.Namespace, "extKubeConfig", extKubeConfig)
			return err
		} else {
			logger.V(3).Info("uninstall edge cluster success", "name", instance.Name)
		}
	}

	//remove edgecluster pvc
	err = r.CleanEdgeClusterResources(instance.Name, instance.Spec.Namespace, extKubeConfig)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) undoReconcile(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx, "func", "undoReconcile", "instance", instance.Name)
	logger.V(4).Info("delete edge cluster", "instance", instance)

	//kubeconfig := instance.Status.ConfigFile 此处逻辑应该是判断 InstallType 是否为 auto，如果是 auto 则需要从MetaCluster中卸载
	switch instance.Spec.Type {
	case infrav1alpha1.InstallTypeManual:
		logger.V(4).Info("manual cluster, skip uninstall cluster ", "instance", instance.Name)
	case infrav1alpha1.InstallTypeAuto:
		if err := r.undoReconcileEdgeCluster(ctx, instance); err != nil {
			logger.Error(err, "undoReconcileEdgeCluster failed", "instance", instance.Name)
		}
	}

	ksMember := &ksclusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
	}
	if err := r.Delete(ctx, ksMember); err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "delete kubesphere member cluster error")
	}

	// 这个逻辑不是所有安装类型都需要，移到 undoReconcileEdgeCluster
	//err := r.CleanEdgeClusterResources(instance.Name, instance.Spec.Namespace, kubeconfig)
	//if err != nil {
	//	return ctrl.Result{}, err
	//}

	//if err := r.Status().Update(ctx, instance); err != nil {  此时无需更新状态，因为已经删除了
	//	return ctrl.Result{}, err
	//}

	// delete infra cluster when all de resources are deleted
	edge := &infrav1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: instance.Name,
		},
	}
	if err := r.Delete(ctx, edge); err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "delete edge cluster error")
	}

	StopWatchEdgeClusterResource(instance.Name)
	StopWatchExternClusterResource(instance.Name)
	return ctrl.Result{}, nil
}

func (r *Reconciler) verifyEdgeCluster(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) (err error) {
	logger := log.FromContext(ctx, "verifyEdgeCluster", instance.Name)
	logger.V(3).Info("origin value", "distro", instance.Spec.Distro, "components", instance.Spec.Components, "advertiseaddress", instance.Spec.AdvertiseAddress)

	err = r.UpdateEdgeCluster(ctx, nn, func(cluster *infrav1alpha1.EdgeCluster) (err error) {
		updated := false
		if cluster.Spec.Components == nil || len(cluster.Spec.Components) == 0 {
			components, err := r.GetDefaultComponents(ctx)
			if err != nil {
				logger.Error(err, "get default components error")
				return err
			}
			cluster.Spec.Components = components
			updated = true
		}
		if cluster.Spec.Distro == "" {
			cluster.Spec.Distro = DefaultDistro
			updated = true
		}
		if cluster.Spec.Type == "" {
			cluster.Spec.Type = infrav1alpha1.InstallTypeAuto
			updated = true
		}
		if cluster.Spec.AdvertiseAddress == nil {
			cluster.Spec.AdvertiseAddress = []string{}
			updated = true
		}
		if updated {
			logger.V(3).Info("update edge cluster", "distro", cluster.Spec.Distro,
				"components", cluster.Spec.Components, "advertiseaddress", cluster.Spec.AdvertiseAddress)
		}
		return nil
	})
	if err != nil {
		logger.Error(err, "update edge cluster error")
	}
	return nil
}

func (r *Reconciler) createKSClusterCR(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error {
	logger := log.FromContext(ctx, "cluster", instance.Name)
	member := &ksclusterv1alpha1.Cluster{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name}, member)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(4).Info("cluster not found, create new cluster", "name", instance.Name)
			member = &ksclusterv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: instance.Name,
					Annotations: map[string]string{
						"kubesphere.io/creator":     "admin",
						"kubesphere.io/alias-name":  instance.Annotations["kubesphere.io/alias-name"],
						"kubesphere.io/description": instance.Annotations["kubesphere.io/description"],
					},
					Labels: map[string]string{
						infrav1alpha1.EdgeClusterRole:         "",
						infrav1alpha1.HostedCLuster:           instance.Spec.HostCluster,
						"infra.edgewize.io/advertise-address": strings.Join(instance.Spec.AdvertiseAddress, "_"),
					},
				},
				Spec: ksclusterv1alpha1.ClusterSpec{
					JoinFederation: true,
					Provider:       "EdgeWize",
					Connection: ksclusterv1alpha1.Connection{
						Type:       ksclusterv1alpha1.ConnectionTypeDirect,
						KubeConfig: []byte(instance.Status.KubeConfig),
					},
				},
			}
			err := r.Create(ctx, member)
			if err != nil {
				logger.Error(err, "create cluster error", "name", instance.Name)
				return err
			}
		} else {
			logger.Error(err, "get kubesphere member cluster error", "name", instance.Name)
		}
	}

	edge := &infrav1alpha1.Cluster{}
	err = r.Get(ctx, types.NamespacedName{Name: instance.Name}, edge)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(4).Info("cluster not found, create new cluster", "name", instance.Name)
			edge = &infrav1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: instance.Name,
					Labels: map[string]string{
						infrav1alpha1.EdgeClusterRole: "",
					},
				},
				Spec: infrav1alpha1.ClusterSpec{
					HostCluster: instance.Spec.HostCluster,
					Provider:    "EdgeWize",
					Connection: infrav1alpha1.Connection{
						Type:       infrav1alpha1.ConnectionTypeDirect,
						KubeConfig: []byte(instance.Status.KubeConfig),
					},
				},
			}
			err := r.Create(ctx, edge)
			if err != nil {
				logger.Error(err, "create cluster error", "name", instance.Name)
				return err
			}
		}
	}

	return nil
}

func (r *Reconciler) getEdgeClusterConfig(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) (map[string]interface{}, error) {
	logger := log.FromContext(ctx, "cluster", instance.Name)
	valuesMap, err := r.GetEdgewizeValuesConfigMap(ctx)
	if err != nil {
		klog.Error("get values map error", err.Error())
		return nil, err
	}

	val, ok := valuesMap[ClusterComponentName]
	if !ok {
		logger.Error(NotFoundError, "cluster component not found in values map", "component", ClusterComponentName)
		return nil, NotFoundError
	}

	return infrav1alpha1.ValueString(val).ToValues(), nil
}

func (r *Reconciler) getEdgeKubeConfig(ctx context.Context, extKubeConfig string, namespace string) ([]byte, error) {
	logger := log.FromContext(ctx, "namespace", namespace, "extKubeConfig", extKubeConfig)
	secret := &corev1.Secret{}
	service := &corev1.Service{}

	if _, err := os.Stat(extKubeConfig); err == nil {
		return nil, err
	}

	extKubeConfigAbs := filepath.Join(homedir.HomeDir(), ".kube", extKubeConfig)
	extConfig, err := clientcmd.BuildConfigFromFlags("", extKubeConfigAbs)
	if err != nil {
		return nil, err
	}
	extClientset, err := kubernetes.NewForConfig(extConfig)
	if err != nil {
		return nil, err
	}

	secret, err = extClientset.CoreV1().Secrets(namespace).Get(context.Background(), fmt.Sprintf("vc-%s", namespace), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	service, err = extClientset.CoreV1().Services(namespace).Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	data, ok := secret.Data["config"]
	if !ok {
		return nil, errors.New("kubeconfig does not exist")
	}
	kubeConfig, err := clientcmd.Load(data)
	if err != nil {
		return nil, err
	}
	edgeCluster := kubeConfig.Contexts[kubeConfig.CurrentContext].Cluster
	if service.Spec.Ports != nil && len(service.Spec.Ports) > 0 {
		kubeConfig.Clusters[edgeCluster].Server = fmt.Sprintf("https://%s:%d", service.Spec.ClusterIP, service.Spec.Ports[0].Port)
	} else {
		return nil, errors.New("edge cluster service port does not exist")
	}
	data, err = clientcmd.Write(*kubeConfig)
	if err != nil {
		logger.Error(err, "encode edge cluster kube config error")
		return nil, err
	}
	return data, nil
}

func (r *Reconciler) installEdgeCluster(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) (err error) {
	// 前置条件： 拥有Meta集群的 kubeconfig，可以操作 Meta 集群安装 EdgeCluster
	// 创建 EdgeCluster，并保存证书
	logger := log.FromContext(ctx, "InstallEdgeCluster", instance.Name)
	logger.V(3).Info("install edge cluster", "cluster", instance.Name)

	extKubeConfig, err := r.LoadExternalKubeConfig(ctx, instance)
	if err != nil {
		return err
	}

	// 获取相关配置
	values, err := r.getEdgeClusterConfig(ctx, nn, instance)
	if err != nil {
		logger.Error(err, "get vcluster values error, use default")
		values = map[string]interface{}{}
	}

	createNamespace := true // 默认创建 namespace
	nsExisted := r.IsNamespaceExisted(ctx, extKubeConfig, instance.Spec.Namespace)
	createNamespace = createNamespace && !nsExisted
	if instance.Annotations == nil {
		instance.Annotations = make(map[string]string)
	}
	clientset, err := r.getClusterClientset(filepath.Join("external", instance.Name))
	if err != nil {
		return err
	}
	err = r.InitCert(ctx, "edgewize-root-ca", instance.Spec.Namespace, nil, clientset)
	if err != nil {
		klog.Error("init edgewize certs error, use default", err)
		return err
	}
	status, err := InstallChart(instance.Spec.Distro, instance.Name, instance.Spec.Namespace, extKubeConfig, createNamespace, values)
	if err != nil {
		logger.Error(err, "install edge cluster error", "status", status,
			"distro", instance.Spec.Distro, "name", instance.Name, "namespace", instance.Spec.Namespace)
		return err
	}
	if status != infrav1alpha1.RunningStatus {
		logger.Error(InstallEdgeClusterError, "install edge cluster error, status is not running", "status", status)
		return err
	}

	//instance.Status.ConfigFile = extKubeConfig  此逻辑只用于 undoReconcile，通过 ConfigFile 保存外部 extKubeConfig
	//其实可以通过判断 installType 来决定是否反安装 kubeconfig ，通过 LoadExternalKubeConfig 获取 外部 kubeconfig

	return nil
}

func (r *Reconciler) prepareEdgeCluster(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) (kubeconfig []byte, err error) {
	// 前置条件： 边缘集群已经开始安装
	// 1. 拿到边缘集群 kubeconfig ，此处需要等待大约 2min
	// 2. 创建 Namespace 和 Pull Image Secret

	logger := log.FromContext(ctx, "InstallEdgeCluster", instance.Name)
	logger.V(3).Info("prepare edge cluster", "cluster", instance.Name)

	extKubeConfig, err := r.LoadExternalKubeConfig(ctx, instance)
	if err != nil {
		return nil, err
	}
	// 获取 EdgeCluster 的 kubeconfig, 此 kubeconfig 存放于 metaCluster 中，命名为 vc-<edgeClusterName>
	config, err := r.getEdgeKubeConfig(ctx, extKubeConfig, instance.Name)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (r *Reconciler) initEdgeCluster(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (err error) {
	logger := log.FromContext(ctx, "initEdgeCluster", instance.Name)
	logger.V(3).Info("init edge cluster", "cluster", instance.Name)

	err = SaveEdgeClusterKubeconfig(instance.Name, []byte(instance.Status.KubeConfig))
	if err != nil {
		logger.Error(err, "save edge cluster kubeconfig error")
		return err
	}

	// 创建 Namespace
	err = r.prepareNamespace(ctx, instance)
	if err != nil {
		return err
	}
	// 创建 Pull Image Secret
	err = r.prepareImagePullSecret(ctx, instance)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) prepareNamespace(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	clientset, err := r.getClusterClientset(instance.Name)
	if err != nil {
		return err
	}
	for _, name := range SystemNamespaces {
		namespace, err := clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				klog.V(3).Infof("namespace %s not found, skip", name)
				continue
			}
			klog.Errorf("get namespace %s error: %v", name, err)
			return err
		}
		namespace.Labels["kubesphere.io/workspace"] = "system-workspace"
		namespace.Labels["kubesphere.io/namespace"] = name

		_, err = clientset.CoreV1().Namespaces().Update(ctx, namespace, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("update namespace %s error: %v", name, err)
			return err
		}
	}

	// create namespace if not exist,which namespace  in ComponentsNamespaces
	for _, component := range ComponentsNamespaces {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: component.namespace,
				Labels: map[string]string{
					"kubesphere.io/workspace": "system-workspace",
					"kubesphere.io/namespace": component.namespace,
				},
			},
		}
		_, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				klog.Warningf("namespace %s already exists", component.namespace)
			} else {
				klog.Errorf("create namespace %s error: %s", component.namespace, err.Error())
				return err
			}
		}
	}

	for _, namespace := range ExtendedNamespaces {
		err := createSystemNamespace(ctx, clientset, namespace)
		if err != nil {
			return err
		}
	}
	return nil
}

func createSystemNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespace string) error {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
			Labels: map[string]string{
				"kubesphere.io/workspace": "system-workspace",
				"kubesphere.io/namespace": namespace,
			},
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Warningf("namespace %s already exists", namespace)
		} else {
			klog.Errorf("create namespace %s error: %s", namespace, err.Error())
			return err
		}
	}
	return nil
}

func (r *Reconciler) prepareImagePullSecret(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	clientset, err := r.getClusterClientset(instance.Name)
	if err != nil {
		return err
	}

	hostSecret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: CurrentNamespace, Name: "zpk-deploy-secret"}
	err = r.Get(ctx, key, hostSecret)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			klog.Warning("image-pull-secret nou found ,skip")
			return nil
		} else {
			klog.Error("get image-pull-secret error ", err.Error())
			return err
		}
	}
	for _, namespace := range SystemNamespaces {
		edgeDeploySecret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, EdgeDeploySecret, metav1.GetOptions{})
		// if found edge-deploy-secret, skip
		if err == nil {
			klog.V(3).Infof("secret edge-deploy-secret exists, skip. edge-deploy-secret: %v", edgeDeploySecret.String())
			continue
		}
		// if not found edge-deploy-secret, create
		if client.IgnoreNotFound(err) != nil {
			klog.Error("get secret edge-deploy-secret error", err.Error())
			return err
		}

		edgeSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      EdgeDeploySecret,
				Namespace: namespace,
			},
			Immutable:  hostSecret.Immutable,
			Data:       hostSecret.Data,
			StringData: hostSecret.StringData,
			Type:       hostSecret.Type,
		}
		klog.V(3).Infof("secret edge-deploy-secret content: %s", edgeSecret.String())
		_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, edgeSecret, metav1.CreateOptions{})
		if err != nil {
			klog.Error("create secret edge-deploy-secret error", err)
			return err
		}
	}

	for _, component := range ComponentsNamespaces {
		edgeDeploySecret, err := clientset.CoreV1().Secrets(component.namespace).Get(ctx, EdgeDeploySecret, metav1.GetOptions{})
		// if found edge-deploy-secret, skip
		if err == nil {
			klog.V(3).Infof("secret edge-deploy-secret exists, skip. edge-deploy-secret: %v", edgeDeploySecret.String())
			continue
		}
		// if not found edge-deploy-secret, create
		if client.IgnoreNotFound(err) != nil {
			klog.Error("get secret edge-deploy-secret error", err.Error())
			return err
		}

		edgeSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      EdgeDeploySecret,
				Namespace: component.namespace,
			},
			Immutable:  hostSecret.Immutable,
			Data:       hostSecret.Data,
			StringData: hostSecret.StringData,
			Type:       hostSecret.Type,
		}
		klog.V(3).Infof("secret edge-deploy-secret content: %s", edgeSecret.String())
		_, err = clientset.CoreV1().Secrets(component.namespace).Create(ctx, edgeSecret, metav1.CreateOptions{})
		if err != nil {
			klog.Error("create secret edge-deploy-secret error", err)
			return err
		}
	}

	for _, namespace := range ExtendedNamespaces {
		err := createImagePullSecret(ctx, clientset, namespace, hostSecret)
		if err != nil {
			return err
		}
	}
	return nil
}

func createImagePullSecret(ctx context.Context, clientset *kubernetes.Clientset, namespace string, hostSecret *corev1.Secret) error {
	edgeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      EdgeDeploySecret,
			Namespace: namespace,
		},
		Immutable:  hostSecret.Immutable,
		Data:       hostSecret.Data,
		StringData: hostSecret.StringData,
		Type:       hostSecret.Type,
	}
	klog.V(3).Infof("secret edge-deploy-secret content: %s", edgeSecret.String())
	secret, err := clientset.CoreV1().Secrets(namespace).Create(ctx, edgeSecret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Warningf("secret edge-deploy-secret already exists in namespace %, secret content: %s", namespace, secret.String())
		} else {
			klog.Errorf("create secret edge-deploy-secret failed in namespace: %s, error: %s", namespace, err.Error())
			return err
		}
	}
	return nil
}

func (r *Reconciler) installComponents(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error {
	// 前置条件： 拥有边缘集群的 kubeconfig，并且已经处理好边缘所需的资源
	// 依次安装 components 组件
	// TODO 此处的组件安装内容需要优化逻辑
	logger := r.Logger.WithValues("instance", instance.Name)
	if instance.Status.KubeConfig == "" {
		err := fmt.Errorf("kubeconfig is null")
		logger.Error(err, "skip install")
		return err
	}

	err := SaveEdgeClusterKubeconfig(instance.Name, []byte(instance.Status.KubeConfig)) // 保存 kubeconfig
	if err != nil {
		logger.Error(err, "Save edge cluster kubeconfig error")
		return err
	}

	return r.InstallEdgeClusterComponents(ctx, instance)
}

func (r *Reconciler) updateServices(ctx context.Context, name types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error {
	// 前置条件： 组件已安装完毕
	// 更新 KSE 集群中的配置方便 Gateway 等服务使用
	// TODO 此处的内容需要从组件安装中移出来，优化逻辑
	return nil
}

func (r *Reconciler) needUpgrade(instance *infrav1alpha1.EdgeCluster) bool {
	upgrade := false

	newComponents := make(map[string]infrav1alpha1.Component)
	if instance.Spec.Components != nil {
		for _, component := range instance.Spec.Components {
			newComponents[component.Name] = component
		}
	}

	oldComponents := instance.Status.Components
	if oldComponents == nil {
		oldComponents = make(map[string]infrav1alpha1.Component)
	}

	for _, item := range ComponentsNamespaces {
		name := item.name
		newComponent, ok := newComponents[name]
		if !ok {
			newComponent = infrav1alpha1.Component{}
		}
		oldComponent, ok := oldComponents[name]
		if !ok {
			oldComponent = infrav1alpha1.Component{}
		}
		if !reflect.DeepEqual(oldComponent.Values, newComponent.Values) {
			upgrade = true
			break
		}
	}
	return upgrade
}

func doReconcile(ctx context.Context, r EdgeClusterOperator, log logr.Logger, instanceName types.NamespacedName, instance *infrav1alpha1.EdgeCluster) (ctrl.Result, error) {
	logger := log.WithValues("instance", instance.Name)
	logger.V(3).Info("doReconcile")
	setStatus := func(instance *infrav1alpha1.EdgeCluster, status infrav1alpha1.SystemStatus) {
		instance.Status.Status = status
		logger.V(3).WithCallDepth(1).Info("set status", "status", instance.Status.Status)
	}
	updateStatus := func(instance *infrav1alpha1.EdgeCluster, status infrav1alpha1.SystemStatus) (ctrl.Result, error) {
		instance.Status.Status = status
		logger.V(3).WithCallDepth(1).Info("set status", "status", instance.Status.Status)
		err := r.UpdateEdgeClusterStatus(ctx, instanceName, instance)
		if err != nil {
			logger.Error(err, "update edge cluster status error")
			return ctrl.Result{Requeue: true}, err
		}
		return ctrl.Result{Requeue: true}, err
	}
	if instance.Name == "" || instance.Spec.Namespace == "" {
		return ctrl.Result{}, errors.New("cluster name and namespace cannot be empty")
	}

	if instance.Status.Status == infrav1alpha1.DeprecatedRunningStatus { // DeprecatedRunningStatus is old status, need to update
		return updateStatus(instance, infrav1alpha1.ComponentSuccessStatus)
	}

	//updateStatus(instance, infrav1alpha1.ComponentFailedStatus)
	//if err:=r.UpdateEdgeClusterStatus(ctx, instanceName, instance) ;err!=nil{
	//	return ctrl.Result{}, err
	//}
	if instance.Status.Status == "" {
		return updateStatus(instance, infrav1alpha1.InitialStatus)
	}

	if instance.Status.Status == infrav1alpha1.ComponentSuccessStatus && r.needUpgrade(instance) { // Upgrade Component and update services
		return updateStatus(instance, infrav1alpha1.ComponentFailedStatus)
	}

	for {
		switch instance.Status.Status {
		default:
			logger.Error(fmt.Errorf("unknown status"), "unknown status", "status", instance.Status.Status)
		case infrav1alpha1.InitialStatus, infrav1alpha1.VerifyFailedStatus:
			setStatus(instance, infrav1alpha1.VerifyingStatus)
			err := r.verifyEdgeCluster(ctx, instanceName, instance)
			if err != nil {
				logger.Error(err, "verify EdgeCluster error")
				return updateStatus(instance, infrav1alpha1.VerifyFailedStatus)
			}

			err = r.createKSClusterCR(ctx, instanceName, instance)
			if err != nil {
				logger.Error(err, "create Kubesphere ClusterCR error")
				return updateStatus(instance, infrav1alpha1.VerifyFailedStatus)
			}
			//err = updateStatus(instance, infrav1alpha1.VerifySuccessStatus)
			//if err != nil {
			//	return ctrl.Result{}, err
			//}
			return updateStatus(instance, infrav1alpha1.VerifySuccessStatus)
		case infrav1alpha1.VerifySuccessStatus, infrav1alpha1.ClusterFailedStatus:
			setStatus(instance, infrav1alpha1.ClusterInstallingStatus)
			switch instance.Spec.Type {
			case infrav1alpha1.InstallTypeManual:
			case infrav1alpha1.InstallTypeAuto:
				err := r.installEdgeCluster(ctx, instanceName, instance)
				if err != nil {
					logger.Error(err, "install EdgeCluster error")
					return updateStatus(instance, infrav1alpha1.ClusterFailedStatus)
				}
			default:
				logger.Error(fmt.Errorf("unknown install type"), "unknown install type", "InstallType", instance.Spec.Type)
				return updateStatus(instance, infrav1alpha1.ClusterFailedStatus)
			}

			return updateStatus(instance, infrav1alpha1.ClusterSuccessStatus)
		case infrav1alpha1.ClusterSuccessStatus, infrav1alpha1.PrepareFailedStatus:
			setStatus(instance, infrav1alpha1.PreparingStatus)
			switch instance.Spec.Type {
			case infrav1alpha1.InstallTypeManual: // 上一阶段已经赋值 kubeconfig
				instance.Status.KubeConfig = string(instance.Spec.KubeConfig)
			case infrav1alpha1.InstallTypeAuto: // 只有自动安装才需要等待 vc-{clusterName} secret 创建完成
				kubeconfig, err := r.prepareEdgeCluster(ctx, instanceName, instance)
				if err != nil {
					logger.Error(err, "prepare EdgeCluster error")
					return updateStatus(instance, infrav1alpha1.PrepareFailedStatus)
				}
				instance.Status.KubeConfig = string(kubeconfig)
			default:
				logger.Error(fmt.Errorf("unknown install type"), "unknown install type", "InstallType", instance.Spec.Type)
				return updateStatus(instance, infrav1alpha1.PrepareFailedStatus)
			}
			//err = updateStatus(instance, infrav1alpha1.PrepareSuccessStatus)
			//if err != nil {
			//	return ctrl.Result{}, err
			//}
			err := r.initEdgeCluster(ctx, instance)
			if err != nil {
				logger.Error(err, "init EdgeCluster error")
				return updateStatus(instance, infrav1alpha1.PrepareFailedStatus)
			}
			return updateStatus(instance, infrav1alpha1.PrepareSuccessStatus)
		case infrav1alpha1.PrepareSuccessStatus, infrav1alpha1.ComponentFailedStatus:
			setStatus(instance, infrav1alpha1.ComponentInstallingStatus)
			err := r.installComponents(ctx, instanceName, instance)
			if err != nil {
				logger.Error(err, "install components error")
				return updateStatus(instance, infrav1alpha1.ComponentFailedStatus)
			}

			return updateStatus(instance, infrav1alpha1.ComponentSuccessStatus)
		case infrav1alpha1.ComponentSuccessStatus:
			logger.V(5).Info("EdgeCluster is ready", "name", instanceName,
				"status", instance.Status.Status, "type", instance.Spec.Type, "kubeconfig", instance.Status.KubeConfig,
				"version", instance.Spec.Version, "components", instance.Spec.Components, "status.components", instance.Status.Components)
			StartWatchEdgeClusterResource(instance.Name, instance.Status.KubeConfig, r.(*Reconciler).Client)
			extKubeConfig, err := r.LoadExternalKubeConfigAbs(ctx, instance)
			if err != nil {
				return ctrl.Result{}, err
			}
			StartWatchExternClusterResource(instance.Name, extKubeConfig, r.(*Reconciler).Client)
			return ctrl.Result{}, nil
			// ServiceUpdating 应该是后台的服务，不应该放在 Reconcile 的行为中
			//case infrav1alpha1.ComponentSuccessStatus:
			//	updateStatus(instance, infrav1alpha1.ServiceUpdatingStatus)
			//	err := r.updateServices(ctx, instanceName, instance)
			//	if err != nil {
			//		updateStatus(instance, infrav1alpha1.ServiceUpdateFailedStatus)
			//		return ctrl.Result{}, err
			//	}
			//	updateStatus(instance, infrav1alpha1.ServiceUpdateSuccessStatus)
			//
			//	if err := r.UpdateEdgeClusterStatus(ctx, instanceName, instance); err != nil {
			//		return ctrl.Result{}, err
			//	}
			//case infrav1alpha1.ServiceUpdateSuccessStatus:
			//	return ctrl.Result{}, nil
		}
	}
}

func (r *Reconciler) InstallEdgeClusterComponents(ctx context.Context, instance *infrav1alpha1.EdgeCluster) error {
	logger := r.Logger.WithValues("InstallEdgeClusterComponents", instance.Name)
	var err error
	clientset, err := r.getClusterClientset(instance.Name)
	if err != nil {
		return err
	}

	components := make(map[string]infrav1alpha1.Component)
	for _, component := range instance.Spec.Components {
		components[component.Name] = component
	}
	// map range
	for _, item := range ComponentsNamespaces {
		componentName := item.name
		component, ok := components[componentName]
		if !ok {
			logger.V(3).Info("component not found", "componentName", componentName)
			continue
		}
		switch componentName {
		case "edgewize":
			err = r.ReconcileEdgeWize(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install edgewize agent error")
				return err
			}
		case "whizard-edge-agent":
			err = r.ReconcileWhizardEdgeAgent(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install whizard-edge-agent error")
				return err
			}
		case "cloudcore":
			err = r.ReconcileCloudCore(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install cloudcore error")
				return err
			}
		case "fluent-operator":
			err = r.ReconcileFluentOperator(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install fluent-operator error")
				return err
			}
		case "ks-core":
			err = r.ReconcileKSCore(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install ks-core error")
				return err
			}
		case "edge-ota-server":
			err = r.ReconcileEdgeOtaServer(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install edge-ota-server error")
				return err
			}
		case "kubefed":
			err = r.ReconcileKubefed(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install kubefed error")
				return err
			}
		case "eventbus":
			err = r.ReconcileEventbus(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install eventbus error")
				return err
			}
		case "router-manager":
			err = r.ReconcileRouterManager(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install router-manager error")
				return err
			}
		case "modelmesh":
			err = r.ReconcileModelMesh(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install router-manager error")
				return err
			}
		case "hami-device-plugin":
			err = r.ReconcileHamiDevicePlugin(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install hami-device-plugin error")
				return err
			}
		case "hami-scheduler":
			err = r.ReconcileHamiScheduler(ctx, instance, component, clientset)
			if err != nil {
				logger.Error(err, "install hami-scheduler error")
				return err
			}
		default:
			logger.Info(fmt.Sprintf("unknown component %s", component.Name))
		}
	}
	return nil
}

func (r *Reconciler) UpdateEdgeCluster(ctx context.Context, nn types.NamespacedName, updateFunc updateEdgeClusterFunc) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		instance := &infrav1alpha1.EdgeCluster{}
		if err := r.Get(ctx, nn, instance); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		err = updateFunc(instance)
		if err != nil {
			return err
		}
		err = r.Update(ctx, instance)
		return err
	})
}

func (r *Reconciler) UpdateEdgeClusterStatus(ctx context.Context, nn types.NamespacedName, instance *infrav1alpha1.EdgeCluster) error {
	fmt.Println("[+]UpdateEdgeClusterStatus", instance.Name)
	err := r.Status().Update(ctx, instance)
	return err
}

// LoadExternalKubeConfig 从 ConfigMap 加载外部的 kubeconfig 保存到本地目录
func (r *Reconciler) LoadExternalKubeConfig(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (string, error) {
	externalNamespace := instance.Name
	path := filepath.Join("external", externalNamespace)
	file := filepath.Join(homedir.HomeDir(), ".kube", path)
	if _, err := os.Stat(file); err == nil {
		return path, nil
	} else {
		return "", err
	}
}

// LoadExternalKubeConfig 从 ConfigMap 加载外部的 kubeconfig 保存到本地目录
func (r *Reconciler) LoadExternalKubeConfigAbs(ctx context.Context, instance *infrav1alpha1.EdgeCluster) (string, error) {
	externalNamespace := instance.Name
	path := filepath.Join("external", externalNamespace)
	file := filepath.Join(homedir.HomeDir(), ".kube", path)
	if _, err := os.Stat(file); err == nil {
		return file, nil
	} else {
		return "", err
	}
}

func (r *Reconciler) CleanEdgeClusterResources(name, namespace, kubeconfig string) error {
	clientset, err := r.getClusterClientset(kubeconfig)
	if err != nil {
		return err
	}

	err = clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), fmt.Sprintf("data-%s-0", name), metav1.DeleteOptions{})
	if err != nil {
		klog.Error("delete edgecluster pvc error", fmt.Sprintf("data-%s-0", name), err.Error(), "instance:", name)
		return err
	}
	err = clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), fmt.Sprintf("data-%s-etcd-0", name), metav1.DeleteOptions{})
	if err != nil {
		klog.Error("delete edgecluster pvc error", fmt.Sprintf("data-%s-etcd-0", name), err.Error(), "instance:", name)
		return err
	}
	// delete local kubeconfig dile
	//path := filepath.Join(homedir.HomeDir(), ".kube", name)
	//err = os.Remove(path)
	//if err != nil {
	//	klog.Error("delete kubeconfig file error", err.Error(), "instance:", name)
	//	return err
	//}
	return nil
}

func (r *Reconciler) GetDefaultComponents(ctx context.Context) ([]infrav1alpha1.Component, error) {
	valuesMap, err := r.GetEdgewizeValuesConfigMap(ctx)
	if err != nil {
		klog.Error("get values map error", err.Error())
		return nil, err
	}
	result := make([]infrav1alpha1.Component, 0)
	for _, item := range ComponentsNamespaces {
		name, namespace := item.name, item.namespace
		val, ok := valuesMap[name]
		if !ok {
			klog.V(3).Infof("component %s not found in values map", name)
			continue
		}
		component := infrav1alpha1.Component{
			Name:            name,
			File:            fmt.Sprintf("charts/edge/%s", name),
			Values:          infrav1alpha1.ValueString(val),
			Namespace:       namespace,
			SystemNamespace: true,
		}
		result = append(result, component)
	}
	return result, nil
}

func (r *Reconciler) GetEdgewizeValuesConfigMap(ctx context.Context) (map[string]string, error) {
	configmap := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
		Name:      EdgeWizeValuesConfigName,
	}
	err := r.Get(ctx, key, configmap)
	if err != nil {
		klog.Error("get configmap error", err.Error())
		return nil, err
	}
	return configmap.Data, nil
}

//func (r *Reconciler) GetValuesFromConfigMap(ctx context.Context, component string) (chartutil.Values, error) {
//	configmap := &corev1.ConfigMap{}
//	key := types.NamespacedName{
//		Namespace: CurrentNamespace,
//		Name:      EdgeWizeValuesConfigName,
//	}
//	err := r.Get(ctx, key, configmap)
//	if err != nil {
//		return nil, err
//	}
//	values := make(map[string]interface{})
//	if content := configmap.Data[component]; content != "" {
//		strings.Replace(content, "$name", component, 1) // TODO
//		values, err = chartutil.ReadValues([]byte(content))
//		if err != nil {
//			return nil, err
//		}
//		return values, nil
//	}
//	return values, nil
//}

func (r *Reconciler) IsNamespaceExisted(ctx context.Context, kubeconfig, namespace string) bool {
	clientset, err := r.getClusterClientset(kubeconfig)
	if err != nil {
		return false
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

func (r *Reconciler) getClusterKubeConfig(clusterName string) (*rest.Config, error) {
	file := filepath.Join(homedir.HomeDir(), ".kube", clusterName)
	config, err := clientcmd.BuildConfigFromFlags("", file)
	if err != nil {
		return nil, fmt.Errorf("get cluster kubeconfig(%s) error: %w", file, err)
	}
	return config, nil
}

func (r *Reconciler) getClusterClientset(clusterName string) (*kubernetes.Clientset, error) {
	config, err := r.getClusterKubeConfig(clusterName)
	if err != nil {
		klog.Errorf("create rest config (clusterName:%s, ) error: %s", clusterName, err.Error())
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("create k8s client from restconfig (clusterName:%s) error: %s", clusterName, err.Error())
		return nil, err
	}
	return clientset, nil
}

func getClientSetByKubeConfig(kubeconfig string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func getClientSetByKubeConfigFile(kubeconfigFile string) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigFile)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}
