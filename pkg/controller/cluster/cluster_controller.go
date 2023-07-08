/*
Copyright 2020 KubeSphere Authors

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

package cluster

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	kubesphere "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	clusterinformer "github.com/edgewize-io/edgewize/pkg/client/informers/externalversions/infra/v1alpha1"
	clusterlister "github.com/edgewize-io/edgewize/pkg/client/listers/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/utils/k8sutil"
)

// Cluster controller only runs under multicluster mode. Cluster controller is following below steps,
//   1. Wait for cluster agent is ready if connection type is proxy
//   2. Join cluster into federation control plane if kubeconfig is ready.
//   3. Pull cluster version and configz, set result to cluster status
// Also put all clusters back into queue every 5 * time.Minute to sync cluster status, this is needed
// in case there aren't any cluster changes made.
// Also check if all of the clusters are ready by the spec.connection.kubeconfig every resync period

const (
	// maxRetries is the number of times a service will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the
	// sequence of delays between successive queuings of a service.
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15

	kubefedNamespace  = "kube-federation-system"
	openpitrixRuntime = "openpitrix.io/runtime"
	kubesphereManaged = "kubesphere.io/managed"

	// Actually host cluster name can be anything, there is only necessary when calling JoinFederation function
	hostClusterName = "kubesphere"

	// allocate kubernetesAPIServer port in range [portRangeMin, portRangeMax] for agents if port is not specified
	// kubesphereAPIServer port is defaulted to kubernetesAPIServerPort + 10000
	portRangeMin = 6000
	portRangeMax = 7000

	// proxy format
	proxyFormat = "%s/api/v1/namespaces/kubesphere-system/services/:ks-apiserver:80/proxy/%s"

	// probe cluster timeout
	probeClusterTimeout = 3 * time.Second
)

// Cluster template for reconcile host cluster if there is none.
var hostCluster = &infrav1alpha1.Cluster{
	ObjectMeta: metav1.ObjectMeta{
		Name: "host",
		Annotations: map[string]string{
			"kubesphere.io/description": "The description was created by KubeSphere automatically. " +
				"It is recommended that you use the Host Cluster to manage clusters only " +
				"and deploy workloads on Member Clusters.",
		},
		Labels: map[string]string{
			infrav1alpha1.HostClusterRole: "",
			kubesphereManaged:             "true",
		},
	},
	Spec: infrav1alpha1.ClusterSpec{
		HostCluster: "host",
		Provider:    "edgewize",
		Connection: infrav1alpha1.Connection{
			Type: infrav1alpha1.ConnectionTypeDirect,
		},
	},
}

type ClusterController struct {
	eventBroadcaster record.EventBroadcaster
	eventRecorder    record.EventRecorder

	// build this only for host cluster
	k8sClient  kubernetes.Interface
	hostConfig *rest.Config

	ksClient kubesphere.Interface

	clusterLister    clusterlister.ClusterLister
	clusterHasSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	workerLoopPeriod time.Duration

	resyncPeriod time.Duration

	hostClusterName string
}

func NewClusterController(
	k8sClient kubernetes.Interface,
	ksClient kubesphere.Interface,
	config *rest.Config,
	clusterInformer clusterinformer.ClusterInformer,
	resyncPeriod time.Duration,
	hostClusterName string,
) *ClusterController {

	broadcaster := record.NewBroadcaster()
	broadcaster.StartLogging(func(format string, args ...interface{}) {
		klog.Info(fmt.Sprintf(format, args))
	})
	broadcaster.StartRecordingToSink(&corev1.EventSinkImpl{Interface: k8sClient.CoreV1().Events("")})
	recorder := broadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "cluster-controller"})

	c := &ClusterController{
		eventBroadcaster: broadcaster,
		eventRecorder:    recorder,
		k8sClient:        k8sClient,
		ksClient:         ksClient,
		hostConfig:       config,
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cluster"),
		workerLoopPeriod: time.Second,
		resyncPeriod:     resyncPeriod,
		hostClusterName:  hostClusterName,
	}
	c.clusterLister = clusterInformer.Lister()
	c.clusterHasSynced = clusterInformer.Informer().HasSynced

	clusterInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueCluster,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCluster := oldObj.(*infrav1alpha1.Cluster)
			newCluster := newObj.(*infrav1alpha1.Cluster)
			if !reflect.DeepEqual(oldCluster.Spec, newCluster.Spec) || newCluster.DeletionTimestamp != nil {
				c.enqueueCluster(newObj)
			}
		},
		DeleteFunc: c.enqueueCluster,
	}, resyncPeriod)

	return c
}

func (c *ClusterController) Start(ctx context.Context) error {
	return c.Run(3, ctx.Done())
}

func (c *ClusterController) Run(workers int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.V(0).Info("starting cluster controller")
	defer klog.Info("shutting down cluster controller")

	if !cache.WaitForCacheSync(stopCh, c.clusterHasSynced) {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, c.workerLoopPeriod, stopCh)
	}

	// refresh cluster configz every resync period
	go wait.Until(func() {
		if err := c.reconcileHostCluster(); err != nil {
			klog.Errorf("Error create host cluster, error %v", err)
		}

		if err := c.resyncClusters(); err != nil {
			klog.Errorf("failed to reconcile cluster ready status, err: %v", err)
		}
	}, c.resyncPeriod, stopCh)

	<-stopCh
	return nil
}

func (c *ClusterController) worker() {
	for c.processNextItem() {
	}
}

func (c *ClusterController) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)

	err := c.syncCluster(key.(string))
	c.handleErr(err, key)
	return true
}

// reconcileHostCluster will create a host cluster if there are no clusters labeled 'cluster-role.kubesphere.io/host'
func (c *ClusterController) reconcileHostCluster() error {
	clusters, err := c.clusterLister.List(labels.SelectorFromSet(labels.Set{infrav1alpha1.HostClusterRole: ""}))
	if err != nil {
		return err
	}

	hostKubeConfig, err := buildKubeconfigFromRestConfig(c.hostConfig)
	if err != nil {
		return err
	}

	// no host cluster, create one
	if len(clusters) == 0 {
		hostCluster.Spec.Connection.KubeConfig = hostKubeConfig
		hostCluster.Name = c.hostClusterName
		_, err = c.ksClient.InfraV1alpha1().Clusters().Create(context.TODO(), hostCluster, metav1.CreateOptions{})
		return err
	} else if len(clusters) > 1 {
		return fmt.Errorf("there MUST not be more than one host clusters, while there are %d", len(clusters))
	}

	// only deal with cluster managed by kubesphere
	cluster := clusters[0].DeepCopy()
	managedByKubesphere, ok := cluster.Labels[kubesphereManaged]
	if !ok || managedByKubesphere != "true" {
		return nil
	}

	// no kubeconfig, not likely to happen
	if len(cluster.Spec.Connection.KubeConfig) == 0 {
		cluster.Spec.Connection.KubeConfig = hostKubeConfig
	} else {
		// if kubeconfig are the same, then there is nothing to do
		if bytes.Equal(cluster.Spec.Connection.KubeConfig, hostKubeConfig) {
			return nil
		}
	}

	// update host cluster config
	_, err = c.ksClient.InfraV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
	return err
}

func (c *ClusterController) resyncClusters() error {
	clusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		key, _ := cache.MetaNamespaceKeyFunc(cluster)
		c.queue.Add(key)
	}

	return nil
}

func (c *ClusterController) syncCluster(key string) error {
	klog.V(5).Infof("starting to sync cluster %s", key)
	startTime := time.Now()

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("not a valid controller key %s, %#v", key, err)
		return err
	}

	defer func() {
		klog.V(4).Infof("Finished syncing cluster %s in %s", name, time.Since(startTime))
	}()

	cluster, err := c.clusterLister.Get(name)
	if err != nil {
		// cluster not found, possibly been deleted
		// need to do the cleanup
		if errors.IsNotFound(err) {
			return nil
		}

		klog.Errorf("Failed to get cluster with name %s, %#v", name, err)
		return err
	}

	if cluster.ObjectMeta.DeletionTimestamp.IsZero() {
		// The object is not being deleted, so if it does not have our finalizer,
		// then lets add the finalizer and update the object. This is equivalent
		// registering our finalizer.
		if !sets.NewString(cluster.ObjectMeta.Finalizers...).Has(infrav1alpha1.Finalizer) {
			cluster.ObjectMeta.Finalizers = append(cluster.ObjectMeta.Finalizers, infrav1alpha1.Finalizer)
			if cluster, err = c.ksClient.InfraV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	} else {
		// The object is being deleted
		if sets.NewString(cluster.ObjectMeta.Finalizers...).Has(infrav1alpha1.Finalizer) {
			// remove our cluster finalizer
			err = c.ksClient.InfraV1alpha1().EdgeClusters("edgewize-system").Delete(context.Background(), cluster.Name, metav1.DeleteOptions{})
			if err != nil {
				klog.Errorf("Failed to delete edge cluster with name %s, %#v", name, err)
			}
			finalizers := sets.NewString(cluster.ObjectMeta.Finalizers...)
			finalizers.Delete(infrav1alpha1.Finalizer)
			cluster.ObjectMeta.Finalizers = finalizers.List()
			if _, err = c.ksClient.InfraV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
		return nil
	}

	// save a old copy of cluster
	oldCluster := cluster.DeepCopy()

	if len(cluster.Spec.Connection.KubeConfig) == 0 {
		klog.V(5).Infof("Skipping to join cluster %s cause the kubeconfig is empty", cluster.Name)
		return nil
	}

	clusterConfig, err := clientcmd.RESTConfigFromKubeConfig(cluster.Spec.Connection.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create cluster config for %s: %s", cluster.Name, err)
	}

	clusterClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to create cluster client for %s: %s", cluster.Name, err)
	}

	// cluster is ready, we can pull kubernetes cluster info through agent
	// since there is no agent necessary for host cluster, so updates for host cluster
	// is safe.
	if len(cluster.Spec.Connection.KubernetesAPIEndpoint) == 0 {
		cluster.Spec.Connection.KubernetesAPIEndpoint = clusterConfig.Host
	}

	serverVersion, err := clusterClient.Discovery().ServerVersion()
	if err != nil {
		klog.Errorf("Failed to get kubernetes version, %#v", err)
		return err
	}
	cluster.Status.KubernetesVersion = serverVersion.GitVersion

	labelSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "vcluster.loft.sh/fake-node",
				Operator: "DoesNotExist",
			},
		},
	}
	nodes, err := clusterClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		klog.Errorf("Failed to get cluster nodes, %#v", err)
		return err
	}
	cluster.Status.NodeCount = len(nodes.Items)

	// Use kube-system namespace UID as cluster ID
	kubeSystem, err := clusterClient.CoreV1().Namespaces().Get(context.TODO(), metav1.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		return err
	}
	cluster.Status.UID = kubeSystem.UID

	readyCondition := infrav1alpha1.ClusterCondition{
		Type:               infrav1alpha1.ClusterReady,
		Status:             v1.ConditionTrue,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             string(infrav1alpha1.ClusterReady),
		Message:            "Cluster is available now",
	}
	c.updateClusterCondition(cluster, readyCondition)

	if err = c.updateKubeConfigExpirationDateCondition(cluster); err != nil {
		klog.Errorf("sync KubeConfig expiration date for cluster %s failed: %v", cluster.Name, err)
		return err
	}

	if !reflect.DeepEqual(oldCluster.Status, cluster.Status) {
		_, err = c.ksClient.InfraV1alpha1().Clusters().Update(context.TODO(), cluster, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Failed to update cluster status, %#v", err)
			return err
		}
	}

	return nil
}

func (c *ClusterController) enqueueCluster(obj interface{}) {
	cluster := obj.(*infrav1alpha1.Cluster)

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("get cluster key %s failed", cluster.Name))
		return
	}

	c.queue.Add(key)
}

func (c *ClusterController) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		klog.V(2).Infof("Error syncing cluster %s, retrying, %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}

	klog.V(4).Infof("Dropping cluster %s out of the queue.", key)
	c.queue.Forget(key)
	utilruntime.HandleError(err)
}

// updateClusterCondition updates condition in cluster conditions using giving condition
// adds condition if not existed
func (c *ClusterController) updateClusterCondition(cluster *infrav1alpha1.Cluster, condition infrav1alpha1.ClusterCondition) {
	if cluster.Status.Conditions == nil {
		cluster.Status.Conditions = make([]infrav1alpha1.ClusterCondition, 0)
	}

	newConditions := make([]infrav1alpha1.ClusterCondition, 0)
	for _, cond := range cluster.Status.Conditions {
		if cond.Type == condition.Type {
			continue
		}
		newConditions = append(newConditions, cond)
	}

	newConditions = append(newConditions, condition)
	cluster.Status.Conditions = newConditions
}

func parseKubeConfigExpirationDate(kubeconfig []byte) (time.Time, error) {
	config, err := k8sutil.LoadKubeConfigFromBytes(kubeconfig)
	if err != nil {
		return time.Time{}, err
	}
	if config.CertData == nil {
		return time.Time{}, fmt.Errorf("empty CertData")
	}
	block, _ := pem.Decode(config.CertData)
	if block == nil {
		return time.Time{}, fmt.Errorf("pem.Decode failed, got empty block data")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return time.Time{}, err
	}
	return cert.NotAfter, nil
}

func (c *ClusterController) updateKubeConfigExpirationDateCondition(cluster *infrav1alpha1.Cluster) error {
	if _, ok := cluster.Labels[infrav1alpha1.HostClusterRole]; ok {
		return nil
	}
	// we don't need to check member clusters which using proxy mode, their certs are managed and will be renewed by tower.
	if cluster.Spec.Connection.Type == infrav1alpha1.ConnectionTypeProxy {
		return nil
	}

	klog.V(5).Infof("sync KubeConfig expiration date for cluster %s", cluster.Name)
	notAfter, err := parseKubeConfigExpirationDate(cluster.Spec.Connection.KubeConfig)
	if err != nil {
		return fmt.Errorf("parseKubeConfigExpirationDate for cluster %s failed: %v", cluster.Name, err)
	}
	expiresInSevenDays := v1.ConditionFalse
	if time.Now().AddDate(0, 0, 7).Sub(notAfter) > 0 {
		expiresInSevenDays = v1.ConditionTrue
	}

	c.updateClusterCondition(cluster, infrav1alpha1.ClusterCondition{
		Type:               infrav1alpha1.ClusterKubeConfigCertExpiresInSevenDays,
		Status:             expiresInSevenDays,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             string(infrav1alpha1.ClusterKubeConfigCertExpiresInSevenDays),
		Message:            notAfter.String(),
	})
	return nil
}
