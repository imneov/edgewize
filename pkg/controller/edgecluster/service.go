package edgecluster

import (
	"context"
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
		watchedEdgeCluster.Store(clusterName, stopChan)
		klog.Infof("start new goroutine to watch cloudcore service change in edgecluster %s", clusterName)
		var targets = []ServiceWatch{
			&CloudCoreServiceWatch{
				ClusterName: clusterName,
				Name:        "cloudcore",
				Namespace:   "kubeedge",
			},
			&EdgeOTAServiceWatch{
				ClusterName: clusterName,
				Name:        "edge-ota-server",
				Namespace:   CurrentNamespace,
			},
		}

		go run(clientset, cli, stopChan, targets)
	}
}

func StopWatchEdgeClusterResource(clusterName, kubeconfig string, cli client.Client) {
	if value, ok := watchedEdgeCluster.Load(clusterName); ok {
		stopChan := value.(chan struct{})
		stopChan <- struct{}{}
		watchedEdgeCluster.Delete(clusterName)
	}
}

func run(clientset *kubernetes.Clientset, cli client.Client, stopCh chan struct{}, targets []ServiceWatch) {
	// 创建一个 Deployment 的 Informer
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.CoreV1().Services(metav1.NamespaceAll).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.CoreV1().Services(metav1.NamespaceAll).Watch(context.TODO(), options)
			},
		},
		&corev1.Service{},
		0,
		cache.Indexers{},
	)

	// 注册事件处理程序
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			for _, target := range targets {
				if target.GetServiceNamespace() == service.Namespace && target.GetServiceName() == service.Name {
					retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
						err = target.Update(context.Background(), service, cli)
						if err != nil {
							klog.Errorf("update service error, %v", target)
							return
						}
						return nil
					})
					klog.V(3).Infof("update service %s successful, target: %v", target.GetServiceName(), target)
					break
				}
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			service := newObj.(*corev1.Service)
			for _, target := range targets {
				if target.GetServiceNamespace() == service.Namespace && target.GetServiceName() == service.Name {
					retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
						err = target.Update(context.Background(), service, cli)
						if err != nil {
							klog.Errorf("update service error, %v", target)
							return
						}
						return nil
					})
					klog.V(3).Infof("update service %s successful, target: %v", target.GetServiceName(), target)
					break
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			service := obj.(*corev1.Service)
			for _, target := range targets {
				if target.GetServiceNamespace() == service.Namespace && target.GetServiceName() == service.Name {
					retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
						err = target.Delete(context.Background(), service, cli)
						if err != nil {
							klog.Errorf("update service error, %v", target)
							return
						}
						return nil
					})
					klog.V(3).Infof("update service %s successful, target: %v", target.GetServiceName(), target)
					break
				}
			}
		},
	})
	if err != nil {
		return
	}
	// 启动 Informer
	informer.Run(stopCh)
	for _, target := range targets {
		err := target.Delete(context.Background(), nil, cli)
		if err != nil {
			klog.Errorf("update service error, %v", target)
			continue
		}
	}
}

type ServiceWatch interface {
	GetServiceName() string
	GetServiceNamespace() string
	GetEdgeClusterName() string
	Update(ctx context.Context, svc *corev1.Service, cli client.Client) error
	Delete(ctx context.Context, svc *corev1.Service, cli client.Client) error
}

var _ ServiceWatch = &CloudCoreServiceWatch{}
var _ ServiceWatch = &EdgeOTAServiceWatch{}

type CloudCoreServiceWatch struct {
	ClusterName string
	Name        string
	Namespace   string
}

func (c *CloudCoreServiceWatch) GetServiceName() string {
	return c.Name
}

func (c *CloudCoreServiceWatch) GetServiceNamespace() string {
	return c.Namespace
}

func (c *CloudCoreServiceWatch) GetEdgeClusterName() string {
	return c.ClusterName
}

func (c *CloudCoreServiceWatch) Update(ctx context.Context, svc *corev1.Service, cli client.Client) error {
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
	svcMap[c.ClusterName] = svc.Spec
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

func (c *CloudCoreServiceWatch) Delete(ctx context.Context, svc *corev1.Service, cli client.Client) error {
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
	klog.V(3).Infof("delete cloudcore service: %s", c.ClusterName)
	delete(svcMap, c.ClusterName)
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

type EdgeOTAServiceWatch struct {
	ClusterName string
	Name        string
	Namespace   string
}

func (e *EdgeOTAServiceWatch) GetServiceName() string {
	return e.Name
}

func (e *EdgeOTAServiceWatch) GetServiceNamespace() string {
	return e.Namespace
}

func (e *EdgeOTAServiceWatch) GetEdgeClusterName() string {
	return e.ClusterName
}

func (e *EdgeOTAServiceWatch) Update(ctx context.Context, svc *corev1.Service, cli client.Client) error {
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
	otaServerName := fmt.Sprintf("otaserver-%s", e.ClusterName)
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

func (e *EdgeOTAServiceWatch) Delete(ctx context.Context, svc *corev1.Service, cli client.Client) error {
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
	otaServerName := fmt.Sprintf("otaserver-%s", e.ClusterName)
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
