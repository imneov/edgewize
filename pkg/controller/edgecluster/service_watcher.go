package edgecluster

import (
	"context"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"runtime/pprof"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
)

var watchedEdgeCluster sync.Map

func StartWatchEdgeClusterResource(clusterName, kubeconfig string, cli client.Client) {
	if _, ok := watchedEdgeCluster.Load(clusterName); ok {
		return
	} else {
		if kubeconfig == "" {
			return
		}
		clientset, err := getClientSetByKubeConfig(kubeconfig)
		if err != nil {
			return
		}
		stopChan := make(chan struct{})
		klog.Infof("start new goroutine to watch cloudcore service change in edgecluster %s", clusterName)

		cloudcoreWatcher := &ServiceWatcher{
			ClusterName: clusterName,
			Name:        "cloudcore",
			Namespace:   "kubeedge",
			hostClient:  cli,
			edgeClient:  clientset,
			StopChan:    stopChan,
			handler:     &CloudCoreServiceWatcher{},
		}

		edgeOATWatcher := &ServiceWatcher{
			ClusterName: clusterName,
			Name:        "edge-ota-server",
			Namespace:   CurrentNamespace,
			hostClient:  cli,
			edgeClient:  clientset,
			StopChan:    stopChan,
			handler:     &EdgeOTAServiceWatcher{},
		}
		whizardWatcher := &ServiceWatcher{
			ClusterName: clusterName,
			Name:        MonitorPromServiceName,
			Namespace:   MonitorNamespace,
			hostClient:  cli,
			edgeClient:  clientset,
			StopChan:    stopChan,
			handler:     &WhizardServiceWatcher{},
		}

		go cloudcoreWatcher.Run()
		go edgeOATWatcher.Run()
		go whizardWatcher.Run()
		watchedEdgeCluster.Store(clusterName, stopChan)
	}
}

func StopWatchEdgeClusterResource(clusterName string) {
	if value, ok := watchedEdgeCluster.Load(clusterName); ok {
		stopChan := value.(chan struct{})
		close(stopChan)
		watchedEdgeCluster.Delete(clusterName)
	}
}

//---------------------------------------------------

type WatcherHandler interface {
	Update(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) error
	Delete(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) error
}

var _ WatcherHandler = &CloudCoreServiceWatcher{}
var _ WatcherHandler = &EdgeOTAServiceWatcher{}
var _ WatcherHandler = &WhizardServiceWatcher{}

type ServiceWatcher struct {
	ClusterName string
	Name        string
	Namespace   string
	hostClient  client.Client
	edgeClient  *kubernetes.Clientset
	StopChan    chan struct{}
	handler     WatcherHandler
}

func (w *ServiceWatcher) Run() {
	// 创建一个 Deployment 的 Informer
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return w.edgeClient.CoreV1().Services(w.Namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return w.edgeClient.CoreV1().Services(w.Namespace).Watch(context.TODO(), options)
			},
		},
		&corev1.Service{},
		0,
		cache.Indexers{},
	)

	// 注册事件处理程序
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    w.onCreate,
		UpdateFunc: w.onUpdate,
		DeleteFunc: w.onDelete,
	})
	if err != nil {
		return
	}
	// 启动 Informer
	informer.Run(w.StopChan)
	klog.V(1).Infof("informer stopped, run delete.")
	var threadProfile = pprof.Lookup("threadcreate")
	klog.V(1).Infof("current threads counts: %d", threadProfile.Count())
	err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		err = w.handler.Delete(context.Background(), w.ClusterName, nil, w.hostClient)
		if err != nil {
			klog.Errorf("update service error, %v", w)
		}
		return err
	})
	if err != nil {
		klog.Errorf("update service error, %v", w)
	}
}

func (w *ServiceWatcher) onCreate(obj interface{}) {
	service := obj.(*corev1.Service)
	if service.Name != w.Name {
		klog.V(3).Infof("not target service, skip")
		return
	}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		err = w.handler.Update(context.Background(), w.ClusterName, service, w.hostClient)
		if err != nil {
			klog.Errorf("update service error, %v", w)
			return
		}
		return nil
	})
	if err != nil {
		klog.Errorf("update service error, %v", w)
		return
	}
	klog.V(3).Infof("update service successful, target: %v", w)
}

func (w *ServiceWatcher) onUpdate(newObj, oldObj interface{}) {
	service := newObj.(*corev1.Service)
	if service.Name != w.Name {
		klog.V(3).Infof("not target service, skip")
		return
	}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		err = w.handler.Update(context.Background(), w.ClusterName, service, w.hostClient)
		if err != nil {
			klog.Errorf("update service error, %v", w)
			return
		}
		return nil
	})
	if err != nil {
		klog.Errorf("update service error, %v", w)
		return
	}
	klog.V(3).Infof("update service successful, target: %v", w)

}

func (w *ServiceWatcher) onDelete(obj interface{}) {
	service := obj.(*corev1.Service)
	if service.Name != w.Name {
		klog.V(3).Infof("not target service, skip")
		return
	}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		err = w.handler.Delete(context.Background(), w.ClusterName, service, w.hostClient)
		if err != nil {
			klog.Errorf("update service error, %v", w)
			return
		}
		return nil
	})
	if err != nil {
		klog.Errorf("update service error, %v", w)
		return
	}
	klog.V(3).Infof("update service successful, target: %v", w)
}

type CloudCoreServiceWatcher struct {
}

func (w *CloudCoreServiceWatcher) Update(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) error {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
		Name:      "edgewize-cloudcore-service",
	}
	err := cli.Get(ctx, key, cm)
	if err != nil {
		klog.Error("get configmap edgewize-cloudcore-service error ", err.Error())
		return err
	}
	svcMap := ServiceMap{}
	if data, ok := cm.Data[EdgeWizeServers]; ok {
		err = yaml.Unmarshal([]byte(data), &svcMap)
		if err != nil {
			klog.Errorf("invalid %s, err:%v", EdgeWizeServers, err)
			//	cm.Data = make(map[string]string) // TODO
		}
	}
	svcMap[clusterName] = svc.Spec
	data, err := yaml.Marshal(svcMap)
	if err != nil {
		klog.Error("Marshal svc.Spec error", err.Error())
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[EdgeWizeServers] = string(data)
	err = cli.Update(ctx, cm)
	if err != nil {
		klog.Error("update edgewize-cloudcore-service configmap error ", err.Error())
		return err
	}
	return nil
}

func (w *CloudCoreServiceWatcher) Delete(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) error {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
		Name:      "edgewize-cloudcore-service",
	}
	err := cli.Get(ctx, key, cm)
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
	klog.V(3).Infof("delete cloudcore service: %s", clusterName)
	delete(svcMap, clusterName)
	data, err := yaml.Marshal(svcMap)
	if err != nil {
		klog.Error("Marshal svc.Spec error", err.Error())
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[EdgeWizeServers] = string(data)
	err = cli.Update(ctx, cm)
	if err != nil {
		klog.Error("update edgewize-cloudcore-service configmap error ", err.Error())
		return err
	}
	return nil
}

type EdgeOTAServiceWatcher struct {
}

func (w *EdgeOTAServiceWatcher) Update(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) error {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
		Name:      "edgewize-cloudcore-service",
	}
	err := cli.Get(ctx, key, cm)
	if err != nil {
		klog.Error("get configmap edgewize-cloudcore-service error ", err.Error())
		return err
	}
	svcMap := ServiceMap{}
	if data, ok := cm.Data[EdgeWizeServers]; ok {
		err = yaml.Unmarshal([]byte(data), &svcMap)
		if err != nil {
			klog.Errorf("invalid %s, err:%v", EdgeWizeServers, err)
			//	cm.Data = make(map[string]string) // TODO
		}
	}
	otaServerName := fmt.Sprintf("otaserver-%s", clusterName)
	svcMap[otaServerName] = svc.Spec
	data, err := yaml.Marshal(svcMap)
	if err != nil {
		klog.Error("Marshal svc.Spec error", err.Error())
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[EdgeWizeServers] = string(data)
	err = cli.Update(ctx, cm)
	if err != nil {
		klog.Error("update edgewize-cloudcore-service configmap error ", err.Error())
		return err
	}
	return nil
}

func (w *EdgeOTAServiceWatcher) Delete(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) error {
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
		Name:      "edgewize-cloudcore-service",
	}
	err := cli.Get(ctx, key, cm)
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
	otaServerName := fmt.Sprintf("otaserver-%s", clusterName)
	klog.V(3).Infof("delete edgeota service: %s", otaServerName)
	delete(svcMap, otaServerName)
	data, err := yaml.Marshal(svcMap)
	if err != nil {
		klog.Error("Marshal svc.Spec error", err.Error())
		return err
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[EdgeWizeServers] = string(data)
	err = cli.Update(ctx, cm)
	if err != nil {
		klog.Error("update edgewize-cloudcore-service configmap error ", err.Error())
		return err
	}
	return nil
}

type WhizardServiceWatcher struct {
}

func (w *WhizardServiceWatcher) Update(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) error {
	var err error
	var routePath string
	if svc.Spec.ClusterIP != "" {
		for _, item := range svc.Spec.Ports {
			if item.Name == "web" {
				routePath = fmt.Sprintf("http://%s:%d/api/v1/write", svc.Spec.ClusterIP, item.Port)
				break
			}
		}
	}

	if routePath == "" {
		err = errors.New("create routePath for whizard edge gateway failed")
		return err
	}

	edgeGatewayConfigMap := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: MonitorNamespace,
		Name:      WhizardEdgeGatewayConfigName,
	}
	err = cli.Get(ctx, key, edgeGatewayConfigMap)
	if err != nil {
		return err
	}

	cmFile, ok := edgeGatewayConfigMap.Data["config.yaml"]
	if !ok {
		err = errors.New("whizard edge configmap is empty")
		return err
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

	routersMap[clusterName] = routePath
	cmData["routers"] = routersMap

	newCfgFile, err := yaml.Marshal(cmData)
	if err != nil {
		return err
	}

	edgeGatewayConfigMap.Data["config.yaml"] = string(newCfgFile)
	err = cli.Update(ctx, edgeGatewayConfigMap)
	return err
}

func (w *WhizardServiceWatcher) Delete(ctx context.Context, clusterName string, svc *corev1.Service, cli client.Client) (err error) {
	edgeGatewayConfigMap := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: MonitorNamespace,
		Name:      WhizardEdgeGatewayConfigName,
	}
	err = cli.Get(ctx, key, edgeGatewayConfigMap)
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

	delete(routersMap, clusterName)
	cmData["routers"] = routersMap

	newCfgFile, err := yaml.Marshal(cmData)
	if err != nil {
		return
	}

	edgeGatewayConfigMap.Data[EdgeGatewayCM] = string(newCfgFile)
	err = cli.Update(ctx, edgeGatewayConfigMap)
	return
}