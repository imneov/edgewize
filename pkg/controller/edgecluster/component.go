package edgecluster

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os/exec"
	"reflect"
	"strings"

	ksclusterv1alpha1 "kubesphere.io/api/cluster/v1alpha1"

	"k8s.io/client-go/util/retry"

	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
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

const AuditEdgeMode = "edge"

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
		logger.Error(err, "set monitor component error, need to configure manually", "instance", instance.Name)
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
	err := r.SetKSCoreValues(ctx, values, instance)
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
			if string(member.Spec.Connection.KubeConfig) != instance.Status.KubeConfig {
				member.Spec.Connection.KubeConfig = []byte(instance.Status.KubeConfig)
				err = r.Update(ctx, member)
				return err
			}
			logger.V(3).Info("no need to update kubesphere member cluster")
			return nil
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
	}
	return nil
}

func (r *Reconciler) ReconcileFluentOperator(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileCloudCore", instance.Name)
	if IsClusterTypeManual(instance) {
		logger.Info("clusterType is manual, skip install fluent-operator")
		return nil
	}

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

func (r *Reconciler) ReconcileEventbus(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileEventbus", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install eventbus")
		return nil
	}

	namespace := component.Namespace
	if instance.Status.Eventbus == "" {

	}
	emqxService := &corev1.Service{}
	key := types.NamespacedName{
		Name:      "emqx",
		Namespace: "edgewize-system",
	}

	err := r.Get(ctx, key, emqxService)
	if err != nil {
		return err
	}
	values := component.Values.ToValues()
	values["clusterName"] = instance.Name
	if emqx, ok := values["emqx"]; ok {
		if emqxMap, ok := emqx.(map[string]interface{}); ok {
			emqxMap["clusterName"] = instance.Name
			emqxMap["messageGatewayInner"] = emqxService.Spec.ClusterIP + ":1883"
			values["emqx"] = emqxMap
		}
	}

	if len(instance.Spec.AdvertiseAddress) == 0 {
		logger.V(4).Info("advertise address, skip install eventbus")
		return nil
	}
	values["messageGateway"] = fmt.Sprintf("%s:%v", instance.Spec.AdvertiseAddress[0], values["gatewayPort"])
	klog.V(3).Infof("ReconcileEventbus: %v", values)
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
	if instance.Status.Eventbus == infrav1alpha1.InstallingStatus {
		klog.V(3).Infof("event-bus is installing, skip upgrade")
		upgrade = false
	} else if instance.Status.Eventbus == infrav1alpha1.ErrorStatus {
		upgrade = true
	}
	klog.V(3).Infof("eventbus upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("eventbus", "eventbus", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install eventbus error")
		instance.Status.RouterManager = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("eventbus status: %s, instance: %s", status, instance.Name)
	instance.Status.Eventbus = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
	}
	return nil
}

func (r *Reconciler) ReconcileRouterManager(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileRouterManager", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install router-manager")
		return nil
	}

	namespace := component.Namespace
	if instance.Status.RouterManager == "" {

	}
	values := component.Values.ToValues()
	klog.V(3).Infof("ReconcileRouterManager: %v", values)
	values["clusterName"] = instance.Name
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
	if instance.Status.RouterManager == infrav1alpha1.InstallingStatus {
		klog.V(3).Infof("router-manager is installing, skip upgrade")
		upgrade = false
	} else if instance.Status.RouterManager == infrav1alpha1.ErrorStatus {
		upgrade = true
	}
	klog.V(3).Infof("router-manager upgrade: %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("router-manager", "router-manager", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install router-manager error")
		instance.Status.RouterManager = infrav1alpha1.ErrorStatus
		return err
	}
	klog.V(3).Infof("router-manager status: %s, instance: %s", status, instance.Name)
	instance.Status.RouterManager = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
	}
	return nil
}

func (r *Reconciler) ReconcileModelMesh(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileModelMesh", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install modelmesh")
		return nil
	}

	namespace := component.Namespace
	values := component.Values.ToValues()
	klog.V(3).Infof("ReconcileModelMesh: %v", values)

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

	klog.V(3).Infof(": %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("modelmesh", "modelmesh", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install modelmesh error", "instance", instance.Name)
		instance.Status.EdgewizeMonitor = infrav1alpha1.ErrorStatus
		return err
	}

	klog.V(3).Infof("modelmesh status: %s, instance: %s", status, instance.Name)
	instance.Status.ModelMesh = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
	}
	return nil

}

func (r *Reconciler) ReconcileHamiDevicePlugin(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileHamiDevicePlugin", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install hamiDevicePlugin")
		return nil
	}

	if len(instance.Spec.AdvertiseAddress) == 0 {
		logger.V(4).Info("advertise address, skip install hamiDevicePlugin")
		return nil
	}

	namespace := component.Namespace
	values := component.Values.ToValues()
	err := r.SetEdgeApiServerConfig(ctx, values, instance)
	if err != nil {
		klog.Warningf("set hamiDevicePlugin cloud apiServerConfig values error, err: %v", err)
	}
	klog.V(3).Infof("ReconcileHamiDevicePlugin: %v", values)

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

	klog.V(3).Infof(": %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("hami-device-plugin", "hami-device-plugin", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install hamiDevicePlugin error", "instance", instance.Name)
		instance.Status.HamiDevicePlugin = infrav1alpha1.ErrorStatus
		return err
	}

	klog.V(3).Infof("hamiDevicePlugin status: %s, instance: %s", status, instance.Name)
	instance.Status.HamiDevicePlugin = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
	}
	return nil
}

func (r *Reconciler) ReconcileHamiScheduler(ctx context.Context, instance *infrav1alpha1.EdgeCluster, component infrav1alpha1.Component, clientset *kubernetes.Clientset) error {
	logger := log.FromContext(ctx, "ReconcileHamiScheduler", instance.Name)
	if instance.Status.KubeConfig == "" {
		logger.V(4).Info("kubeconfig is null, skip install hamiScheduler")
		return nil
	}

	namespace := component.Namespace
	values := component.Values.ToValues()
	klog.V(3).Infof("ReconcileHamiScheduler: %v", values)

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

	klog.V(3).Infof(": %v, instance: %s", upgrade, instance.Name)
	status, err := UpgradeChart("hami-scheduler", "hami-scheduler", namespace, instance.Name, values, upgrade)
	if err != nil {
		logger.Error(err, "install hamiScheduler error", "instance", instance.Name)
		instance.Status.HamiScheduler = infrav1alpha1.ErrorStatus
		return err
	}

	klog.V(3).Infof("hamiScheduler status: %s, instance: %s", status, instance.Name)
	instance.Status.HamiScheduler = status
	if status == infrav1alpha1.RunningStatus {
		instance.Status.Components[component.Name] = component
	}
	return nil
}

func (r *Reconciler) SetKSCoreValues(ctx context.Context, values chartutil.Values, instance *infrav1alpha1.EdgeCluster) error {
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

		auditingCfg, err := r.GetKSCoreAuditingCfg(ctx, value, instance.Name)
		if err != nil {
			klog.Errorf("set auditing config error, err: %v", err)
			return err
		}

		if len(auditingCfg) > 0 {
			err = r.ManageExternalKSCoreAuditing(auditingCfg, instance, values)
			if err != nil {
				klog.Errorf("set manual auditing config error, err: %v", err)
				return err
			}
			config["auditing"] = auditingCfg
		}
	}
	return nil
}

func (r *Reconciler) ManageExternalKSCoreAuditing(auditingCfg map[string]interface{}, instance *infrav1alpha1.EdgeCluster, values chartutil.Values) (err error) {
	switch instance.Spec.Type {
	case infrav1alpha1.InstallTypeAuto:
		return
	case infrav1alpha1.InstallTypeManual:
		auditingCfg["host"], err = values.PathValue("config.auditing.host")
		if err != nil {
			klog.Errorf("parse manual opensearch host error, err: %v", err)
			return
		}
	}

	return
}

func (r *Reconciler) GetKSCoreAuditingCfg(ctx context.Context, ksCfgValue chartutil.Values, clusterName string) (auditingCfg map[string]interface{}, err error) {
	auditingCfg = make(map[string]interface{})
	hostAuditing, err := ksCfgValue.Table("auditing")
	if err != nil {
		klog.Errorf("parse host auditing cfg error, err: %v", err)
		return
	}

	auditingCfg = hostAuditing.AsMap()
	if len(auditingCfg) == 0 {
		return
	}

	auditingCfg["enable"] = false
	auditingCfg["host"], err = r.ParseOpensearchPath(ctx)
	if err != nil {
		klog.Errorf("get host opensearch svc failed, err: %v", err)
		return
	}

	auditingCfg["indexPrefix"] = clusterName
	auditingCfg["mode"] = ""
	return
}

func (r *Reconciler) ParseOpensearchPath(ctx context.Context) (string, error) {
	osService := &corev1.Service{}
	key := types.NamespacedName{
		Name:      "opensearch-cluster-data",
		Namespace: "kubesphere-logging-system",
	}

	err := r.Get(ctx, key, osService)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://%s:9200", osService.Spec.ClusterIP), nil
}

func (r *Reconciler) ParseDefaultAuditingWebhookPath(ctx context.Context) (string, error) {
	webhookService := &corev1.Service{}
	key := types.NamespacedName{
		Name:      "kube-auditing-webhook-svc",
		Namespace: "kubesphere-logging-system",
	}

	err := r.Get(ctx, key, webhookService)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://%s:6443/audit/webhook/event", webhookService.Spec.ClusterIP), nil
}

func IsClusterTypeManual(instance *infrav1alpha1.EdgeCluster) bool {
	return instance.Spec.Type == infrav1alpha1.InstallTypeManual
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
	whizardAgentConf := map[string]interface{}{"tenant": instance.Name}
	whizardEdgeProxyConf := values["whizard_edge_proxy"].(map[string]interface{})
	whizardEdgeProxyConf["tenant"] = instance.Name
	values["clusterType"] = string(instance.Spec.Type)

	switch instance.Spec.Type {
	case infrav1alpha1.InstallTypeAuto:
		whizardAgentConf["gateway_address"], err = r.GetWhizardGatewayInternalAddr(ctx)
		if err != nil {
			return
		}
	case infrav1alpha1.InstallTypeManual:
		// push data to Prometheus directly
		whizardEdgeProxyConf["edge_gateway_address"] = fmt.Sprintf("http://prometheus-k8s.kubesphere-monitoring-system.svc:9090")
		whizardAgentConf["gateway_address"], err = values.PathValue("whizard_agent_proxy.gateway_address")
		if err != nil {
			return
		}
	}

	values["whizard_agent_proxy"] = whizardAgentConf
	values["whizard_edge_proxy"] = whizardEdgeProxyConf
	return
}

func (r *Reconciler) SetEdgeApiServerConfig(ctx context.Context, values chartutil.Values, instance *infrav1alpha1.EdgeCluster) (err error) {
	kubeApiServerConfig := values["apiServerConfig"].(map[string]interface{})
	kubeApiServerConfig["gatewayAddress"] = instance.Spec.AdvertiseAddress[0]
	dnsName := GetDefaultEdgeClusterDnsName(instance.Name)
	if len(instance.Spec.DNSNames) == 0 {
		klog.Warningf("edge cluster [%s] dnsNames is empty", instance.Name)
	} else {
		dnsName = instance.Spec.DNSNames[0]
	}

	kubeApiServerConfig["gatewayDnsName"] = dnsName
	values["apiServerConfig"] = kubeApiServerConfig
	return
}

func (r *Reconciler) GetWhizardGatewayInternalAddr(ctx context.Context) (addr string, err error) {
	gatewayService := &corev1.Service{}
	key := types.NamespacedName{
		Namespace: MonitorNamespace,
		Name:      WhizardGatewayServiceName,
	}
	err = r.Get(ctx, key, gatewayService)
	if err != nil {
		return
	}

	if gatewayService.Spec.Ports != nil && len(gatewayService.Spec.Ports) > 0 {
		gatewayIP := gatewayService.Spec.ClusterIP
		gatewayPort := gatewayService.Spec.Ports[0].Port
		addr = fmt.Sprintf("http://%s:%d", gatewayIP, gatewayPort)
	}

	return
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
