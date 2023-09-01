package edgecluster

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	ksclusterv1alpha1 "kubesphere.io/api/cluster/v1alpha1"
	"os/exec"
	"reflect"
	"strings"

	"k8s.io/client-go/util/retry"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chartutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *Reconciler) ReconcileWhizardEdgeAgent(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileWhizardEdgeAgent", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install whizard-edge-agent")
		return nil
	}
	namespace := component.Namespace
	// if instance.Status.EdgewizeMonitor == "" means edge cluster is first install
	if instance.Status.EdgewizeMonitor == "" {

	}

	values := component.Values.ToValues()
	err := r.SetMonitorComponent(ctx, values, instance)
	if err != nil {
		logger.Error(err, "get gateway svc ip error, need to configure manually", "instance", instance.Name)
	}
	klog.V(3).Infof("whizard-edge-agent values: %v, instance: %s", values, instance.Name)

	upgrade := false
	if instance.Status.Components == nil {
		instance.Status.Components = make(map[string]infrav1alpha1.Component)
	}
	oldComponent, ok := instance.Status.Components[component.Name]
	if ok {
		upgrade = !reflect.DeepEqual(oldComponent.Values, component.Values)
	} else {
		upgrade = true
	}
	klog.V(3).Infof("whizard-edge-agent upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("whizard-edge-agent", "whizard-edge-agent", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install whizard-edge-agent error", "instance", instance.Name)
		instance.Status.EdgewizeMonitor = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("whizard-edge-agent status: %s, instance: %s", status, instance.Name)
	instance.Status.EdgewizeMonitor = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
		err = r.RegisterWhizardEdgeGatewayRouters(ctx, instance, clientset)
		if err != nil {
			logger.Error(err, "update whizard edge gateway config failed")
			instance.Status.EdgewizeMonitor = infrav1alpha1.ErrorStatus
		}
	}
	return nil
}

func (r *Reconciler) ReconcileEdgeOtaServer(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileEdgeOtaServer", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install edge-ota-server")
		return nil
	}

	namespace := component.Namespace
	if instance.Status.EdgeOtaServer == "" {

	}
	values := component.Values.ToValues()
	klog.V(3).Infof("ReconcileEdgeOtaServer: %v", values)
	upgrade := false
	if instance.Status.Components == nil {
		instance.Status.Components = make(map[string]infrav1alpha1.Component)
	}
	oldComponent, ok := instance.Status.Components[component.Name]
	if ok {
		upgrade = !reflect.DeepEqual(oldComponent.Values, component.Values)
	} else {
		upgrade = true
	}
	if instance.Status.EdgeOtaServer == infrav1alpha1.InstallingStatus {
		klog.V(3).Infof("edge-ota-server is installing, skip upgrade")
		upgrade = false
	} else if instance.Status.EdgeOtaServer == infrav1alpha1.ErrorStatus {
		upgrade = true
	}
	klog.V(3).Infof("edge-ota-server upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("edge-ota-server", "edge-ota-server", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install edge-ota-server error")
		instance.Status.EdgeOtaServer = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("edge-ota-server status: %s, instance: %s", status, instance.Name)
	instance.Status.EdgeOtaServer = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
		err := r.UpdateEdgeOtaService(ctx, namespace, instance, clientset)
		if err != nil {
			logger.Info("update edgewize-edge-ota-service error", "error", err)
		}
	}
	return nil

}

func (r *Reconciler) ReconcileKSCore(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileKSCore", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install ks-core")
		return nil
	}

	namespace := component.Namespace
	if instance.Status.KSCore == "" {
		err := r.retryOnError(func() error {
			return r.applyYaml(instance.Name, "charts/edge/cluster-configuration.yaml")
		}, 3)
		if err != nil {
			klog.Error("apply cluster-configuration.yaml error", err)
			return err
		}
	}
	values := component.Values.ToValues()
	err := r.SetKSCoreValues(ctx, values)
	if err != nil {
		logger.Error(err, "set ks-core values error, skip", "instance", instance.Name)
	}
	klog.V(3).Infof("ks-core values: %v, instance: %s", values, instance.Name)
	upgrade := false
	if instance.Status.Components == nil {
		instance.Status.Components = make(map[string]infrav1alpha1.Component)
	}
	oldComponent, ok := instance.Status.Components[component.Name]
	if ok {
		upgrade = !reflect.DeepEqual(oldComponent.Values, component.Values)
	} else {
		upgrade = true
	}
	klog.V(3).Infof("ks-core upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("ks-core", "ks-core", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install ks-core error")
		instance.Status.KSCore = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("ks-core status: %s, instance: %s", status, instance.Name)
	instance.Status.KSCore = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
		err = r.retryOnError(func() error {
			return r.applyYaml(instance.Name, "charts/edge/role-templates.yaml")
		}, 3)
		if err != nil {
			klog.Error("apply role-templates.yaml error", err)
			return err
		}
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			member := &ksclusterv1alpha1.Cluster{}
			err = r.Get(ctx, types.NamespacedName{Name: instance.Name}, member)
			if err != nil {
				logger.Error(err, "get kubesphere member cluster error")
				return err
			}
			member.Spec.Connection.KubeConfig = []byte(instance.Status.KubeConfig)
			err = r.Update(ctx, member)
			return err
		})
		if err != nil {
			return err
		}

	}
	return nil
}

func (r *Reconciler) ReconcileKubefed(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileKubefed", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install kubefed")
		return nil
	}

	namespace := component.Namespace
	if instance.Status.Kubefed == "" {

	}
	values := component.Values.ToValues()
	klog.V(3).Infof("Kubefed values: %v, instance: %s", values, instance.Name)
	upgrade := false
	if instance.Status.Components == nil {
		instance.Status.Components = make(map[string]infrav1alpha1.Component)
	}
	oldComponent, ok := instance.Status.Components[component.Name]
	if ok {
		upgrade = !reflect.DeepEqual(oldComponent.Values, component.Values)
	} else {
		upgrade = true
	}
	klog.V(3).Infof("kubefed upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("kubefed", "kubefed", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install kubefed error")
		instance.Status.Kubefed = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("kubefed status: %s, instance: %s", status, instance.Name)
	instance.Status.Kubefed = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
		err = r.retryOnError(func() error {
			return r.applyYaml(instance.Name, "charts/edge/federatedcrds.yaml")
		}, 3)
		if err != nil {
			klog.Error("apply federatedcrds.yaml error", err)
			return err
		}
	}
	return nil
}

func (r *Reconciler) ReconcileEdgeWize(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileEdgeWize", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install edgewize agent")
		return nil
	}
	namespace := component.Namespace
	if instance.Status.EdgeWize == "" {
		err := r.InitCert(ctx, "edgewize-root-ca", namespace, nil, clientset)
		if err != nil {
			klog.Warning("init edgewize certs error, use default", err)
		}
	}
	values := component.Values.ToValues()
	values["role"] = "member"
	values["edgeClusterName"] = instance.Name
	klog.V(3).Infof("edgewize values: %v, instance: %s", values, instance.Name)
	upgrade := false
	if instance.Status.Components == nil {
		instance.Status.Components = make(map[string]infrav1alpha1.Component)
	}
	oldComponent, ok := instance.Status.Components[component.Name]
	if ok {
		upgrade = !reflect.DeepEqual(oldComponent.Values, component.Values)
	} else {
		upgrade = true
	}
	klog.V(3).Infof("edgewize upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("edgewize", "edgewize", namespace, instance.Name, values, upgrade)
	if err != nil {
		instance.Status.EdgeWize = infrav1alpha1.ErrorStatus
		logger.Error(err, "install edgewize error")
		return err
	}
	klog.V(3).Infof("edgewize status: %s, instance: %s", status, instance.Name)
	instance.Status.EdgeWize = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
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

func (r *Reconciler) ReconcileCloudCore(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileCloudCore", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install cloudcore")
		return nil
	}
	namespace := component.Namespace
	if instance.Status.CloudCore == "" {
		err := r.InitCert(ctx, "cloudhub", namespace, SignCloudCoreCert, clientset)
		if err != nil {
			klog.Warning("init cloudhub certs error, use default", err)
		}
	}
	values := component.Values.ToValues()
	err := SetCloudCoreValues(values, instance)
	if err != nil {
		klog.Warningf("set cloudcore values error, err: %v", err)
	}
	klog.V(3).Infof("cloudcore values: %v, instance: %s", values, instance.Name)
	upgrade := false
	if instance.Status.Components == nil {
		instance.Status.Components = make(map[string]infrav1alpha1.Component)
	}
	oldComponent, ok := instance.Status.Components[component.Name]
	if ok {
		upgrade = !reflect.DeepEqual(oldComponent.Values, component.Values)
	} else {
		upgrade = true
	}
	klog.V(3).Infof("cloudcore upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("cloudcore", "cloudcore", namespace, instance.Name, values, upgrade)
	if err != nil {
		klog.Warning("install cloudcore error, will try again at the next Reconcile.", "error", err)
		instance.Status.CloudCore = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("cloudcore status: %s, instance: %s", status, instance.Name)
	instance.Status.CloudCore = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
		err := r.UpdateCloudCoreService(ctx, "kubeedge", instance, clientset)
		if err != nil {
			logger.Info("update edgewize-cloudcore-service error", "error", err)
		}
	}
	return nil
}

func (r *Reconciler) ReconcileFluentOperator(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileCloudCore", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.Info("kubeconfig is null, skip install fluent-operator")
		return nil
	}
	namespace := component.Namespace
	if instance.Status.FluentOperator == "" {

	}
	values := component.Values.ToValues()
	klog.V(3).Infof("fluent-operator values: %v, instance: %s", values, instance.Name)
	upgrade := false
	if instance.Status.Components == nil {
		instance.Status.Components = make(map[string]infrav1alpha1.Component)
	}
	oldComponent, ok := instance.Status.Components[component.Name]
	if ok {
		upgrade = !reflect.DeepEqual(oldComponent.Values, component.Values)
	} else {
		upgrade = true
	}
	klog.V(3).Infof("fluent-operator upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("fluent-operator", "fluent-operator", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install fluent-operator error")
		instance.Status.FluentOperator = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("fluent-operator status: %s, instance: %s", status, instance.Name)
	instance.Status.FluentOperator = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
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

	whizardEdgeProxyConf := values["whizard_edge_proxy"].(map[string]interface{})
	whizardEdgeProxyConf["tenant"] = instance.Name
	values["whizard_edge_proxy"] = whizardEdgeProxyConf
	return
}

func (r *Reconciler) RegisterWhizardEdgeGatewayRouters(ctx context.Context, instance *infrav1alpha1.EdgeCluster, clientset *kubernetes.Clientset) (err error) {
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

func (r *Reconciler) UpdateEdgeOtaService(ctx context.Context, namespace string, instance *infrav1alpha1.EdgeCluster, clientset *kubernetes.Clientset) error {
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, "edge-ota-server", metav1.GetOptions{})
	if err != nil {
		klog.Error("get service edge-ota-server error ", err.Error())
		return err
	}
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
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
			//	cm.Data = make(map[string]string) // TODO
		}
	}
	otaServerName := fmt.Sprintf("otaserver-%s", instance.Name)
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
	err = r.Update(ctx, cm)
	if err != nil {
		klog.Error("update edgewize-cloudcore-service configmap error ", err.Error())
		return err
	}
	return nil
}

func (r *Reconciler) UpdateCloudCoreService(ctx context.Context, namespace string, instance *infrav1alpha1.EdgeCluster, clientset *kubernetes.Clientset) error {
	svc, err := clientset.CoreV1().Services(namespace).Get(ctx, "cloudcore", metav1.GetOptions{})
	if err != nil {
		klog.Error("get service cloudcore error ", err.Error())
		return err
	}
	cm := &corev1.ConfigMap{}
	key := types.NamespacedName{
		Namespace: CurrentNamespace,
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
			//	cm.Data = make(map[string]string) // TODO
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
		Namespace: CurrentNamespace,
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

func (r *Reconciler) InitCert(ctx context.Context, name, namespace string, serverCertFunc func(crt, key []byte) ([]byte, []byte, error), clientset *kubernetes.Clientset) error {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			klog.Errorf("get secret %s error: %v", name, err)
			return err
		}
	} else {
		klog.Infof("secret %s exists, skip.", name)
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

	secret = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"rootCA.crt": pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: caCrt}),
			"rootCA.key": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: caKey}),
		},
	}
	if serverCertFunc != nil {
		serverCrt, serverKey, err := serverCertFunc(caCrt, caKey)
		if err != nil {
			klog.Errorf("generate server cert error: %v", err)
		} else {
			secret.Data["server.crt"] = pem.EncodeToMemory(&pem.Block{Type: certutil.CertificateBlockType, Bytes: serverCrt})
			secret.Data["server.key"] = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: serverKey})
		}
	}

	_, err = clientset.CoreV1().Secrets(namespace).Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("create secret %s error: %v", name, err)
		return err
	}
	return nil
}

func (r *Reconciler) applyYaml(kubeconfig, filepath string) error {
	cmd := fmt.Sprintf("apply -f %s --kubeconfig %s/.kube/%s --server-side=true --force-conflicts", filepath, homedir.HomeDir(), kubeconfig)
	output, err := exec.Command("/usr/local/bin/kubectl", strings.Split(cmd, " ")...).Output()
	if err != nil {
		klog.Errorf("apply %s error: %s", filepath, err)
		klog.V(3).Infof("apply %s command: %s, output: %s", filepath, cmd, string(output))
		return err
	}
	klog.V(3).Infof("apply %s success", filepath)
	return nil
}

func (r *Reconciler) retryOnError(run func() error, maxTimes int) error {
	var err error
	for i := 0; i < maxTimes; i++ {
		if err = run(); err != nil {
			klog.Errorf("run error, retry..., times: %d/%d", i, maxTimes)
			continue
		} else {
			break
		}
	}
	return err
}
