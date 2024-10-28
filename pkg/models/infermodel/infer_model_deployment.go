package infermodel

import (
	"context"
	"fmt"
	"github.com/edgewize-io/edgewize/pkg/constants"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"
	"strconv"
	"strings"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/listers/apps/v1"

	"github.com/edgewize-io/edgewize/pkg/api"
	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/api/apps/v1alpha1"
	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	kubesphere "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	appslisteners "github.com/edgewize-io/edgewize/pkg/client/listers/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/informers"
	resources "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3"
)

type IMDOperator interface {
	ListInferModelDeployments(ctx context.Context, namespace string, queryParam *query.Query) (*api.ListResult, error)
	GetInferModelDeployment(ctx context.Context, namespace, name string) (*appsv1alpha1.InferModelDeployment, error)
	DeleteInferModelDeployment(ctx context.Context, namespace string, name string, deleteWorkloads bool) (*appsv1alpha1.InferModelDeployment, error)
	ListNodeSpecifications(ctx context.Context, nodeName string) (*appsv1alpha1.NodeSpecifications, error)
	ListRunningInferModelServers(ctx context.Context, nodegroup string) (*appsv1alpha1.RunningInferModelServers, error)
}

func NewInferModelDeploymentOperator(ksclient kubesphere.Interface, client kubernetes.Interface, informers informers.InformerFactory) IMDOperator {
	return &imdOperator{
		client:             client,
		ksclient:           ksclient,
		imdLister:          informers.KubeSphereSharedInformerFactory().Apps().V1alpha1().InferModelDeployments().Lister(),
		deploymentListener: informers.KubernetesSharedInformerFactory().Apps().V1().Deployments().Lister(),
	}
}

type imdOperator struct {
	client             kubernetes.Interface
	ksclient           kubesphere.Interface
	imdLister          appslisteners.InferModelDeploymentLister
	deploymentListener appsv1.DeploymentLister
}

func (o *imdOperator) DeleteInferModelDeployment(ctx context.Context, namespace string, name string, deleteWorkloads bool) (*appsv1alpha1.InferModelDeployment, error) {
	imd, err := o.ksclient.AppsV1alpha1().InferModelDeployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	// 删除
	err = o.ksclient.AppsV1alpha1().InferModelDeployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}

	if deleteWorkloads {
		// current only	support deployment
		for _, selector := range imd.Spec.NodeSelectors {
			labelSelector := labels.Set{apisappsv1alpha1.LabelIMDeployment: name}.AsSelector()
			listOptions := metav1.ListOptions{LabelSelector: labelSelector.String()}
			err = o.client.AppsV1().Deployments(selector.Project).DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions)
			if err != nil {
				return nil, err
			}
		}

	}
	result := &appsv1alpha1.InferModelDeployment{
		InferModelDeployment: *imd,
		Status:               appsv1alpha1.InferModelDeploymentStatus{},
	}
	return result, nil
}

func (o *imdOperator) GetInferModelDeployment(ctx context.Context, namespace, name string) (*appsv1alpha1.InferModelDeployment, error) {
	imd, err := o.imdLister.InferModelDeployments(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return o.updateInferModelDeployment(imd)
}

func (o *imdOperator) listInferModelDeployments(namespace string, selector labels.Selector) ([]runtime.Object, error) {
	imds, err := o.imdLister.InferModelDeployments(namespace).List(selector)
	if err != nil {
		return nil, err
	}
	var appSets = make([]runtime.Object, 0)

	for _, imd := range imds {
		ret, err := o.updateInferModelDeployment(imd)
		if err != nil {
			return nil, err
		}
		appSets = append(appSets, ret)
	}
	return appSets, nil
}

func (o *imdOperator) ListInferModelDeployments(ctx context.Context, namespace string, queryParam *query.Query) (*api.ListResult, error) {
	imds, err := o.listInferModelDeployments(namespace, queryParam.Selector())
	if err != nil {
		return nil, err
	}

	listResult := resources.DefaultList(imds, queryParam, func(left, right runtime.Object, field query.Field) bool {
		return resources.DefaultObjectMetaCompare(left.(*appsv1alpha1.InferModelDeployment).ObjectMeta, right.(*appsv1alpha1.InferModelDeployment).ObjectMeta, field)
	}, func(obj runtime.Object, filter query.Filter) bool {
		return resources.DefaultObjectMetaFilter(obj.(*appsv1alpha1.InferModelDeployment).ObjectMeta, filter)
	})

	return listResult, nil
}

func (o *imdOperator) updateInferModelDeployment(imd *apisappsv1alpha1.InferModelDeployment) (*appsv1alpha1.InferModelDeployment, error) {
	ret := &appsv1alpha1.InferModelDeployment{
		InferModelDeployment: *imd,
		Status: appsv1alpha1.InferModelDeploymentStatus{
			WorkloadStats: appsv1alpha1.WorkloadStats{},
			Workloads:     make([]*v1.Deployment, 0),
		},
	}
	selector := labels.SelectorFromSet(map[string]string{apisappsv1alpha1.LabelIMDeployment: imd.Name})
	list, err := o.deploymentListener.Deployments(imd.Namespace).List(selector)
	if err != nil {
		return nil, err
	}
	for _, deployment := range list {
		status := apisappsv1alpha1.Processing
		for _, condition := range deployment.Status.Conditions {
			switch condition.Type {
			case v1.DeploymentAvailable:
				if condition.Status == corev1.ConditionTrue {
					status = apisappsv1alpha1.Succeeded
				}
			case v1.DeploymentReplicaFailure:
				if condition.Status == corev1.ConditionTrue {
					status = apisappsv1alpha1.Failed
				}
			}
		}
		if status == apisappsv1alpha1.Failed {
			ret.Status.WorkloadStats.Failed++
		} else if status == apisappsv1alpha1.Succeeded {
			ret.Status.WorkloadStats.Succeeded++
		} else if status == apisappsv1alpha1.Processing {
			ret.Status.WorkloadStats.Processing++
		}
		ret.Status.Workloads = append(ret.Status.Workloads, deployment.DeepCopy())
	}

	return ret, nil
}

func (o *imdOperator) ListNodeSpecifications(ctx context.Context, nodeName string) (*appsv1alpha1.NodeSpecifications, error) {
	edgeNode, err := o.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	templateConfig, err := o.client.CoreV1().
		ConfigMaps(constants.DefaultEdgewizeNamespace).
		Get(ctx, constants.ResTemplateConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	devType, devModel := GetDevTypeOnNode(edgeNode.GetLabels())

	items, err := o.GetSpecificDevResTemplates(templateConfig, devType, devModel, isVirtualized(edgeNode.GetLabels()), nodeName)
	if err != nil {
		return nil, err
	}

	ret := &appsv1alpha1.NodeSpecifications{
		Type:              devType,
		DevModel:          devModel,
		Virtualized:       isVirtualized(edgeNode.GetLabels()),
		ResourceTemplates: items,
	}

	return ret, nil
}

func (o *imdOperator) ListRunningInferModelServers(ctx context.Context, nodegroup string) (*appsv1alpha1.RunningInferModelServers, error) {
	result := &appsv1alpha1.RunningInferModelServers{
		Servers: []appsv1alpha1.InferModelServer{},
	}

	if nodegroup == "" {
		return result, nil
	}

	deploymentSelector := labels.NewSelector()
	requireTemplate, _ := labels.NewRequirement(apisappsv1alpha1.LabelIMTemplate, selection.Exists, nil)
	requireTemplateVersion, _ := labels.NewRequirement(apisappsv1alpha1.LabelIMTemplateVersion, selection.Exists, nil)
	requireDeployment, _ := labels.NewRequirement(apisappsv1alpha1.LabelIMDeployment, selection.Exists, nil)
	requireNodeGroup, _ := labels.NewRequirement(apisappsv1alpha1.LabelNodeGroup, selection.Equals, []string{nodegroup})
	deploymentSelector = deploymentSelector.Add(*requireTemplate, *requireTemplateVersion, *requireDeployment, *requireNodeGroup)

	namespaces, err := o.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	servers := []appsv1alpha1.InferModelServer{}
	for _, namespace := range namespaces.Items {
		deployments, err := o.deploymentListener.Deployments(namespace.GetName()).List(deploymentSelector)
		if err != nil {
			klog.Errorf("list infer model server deployment in namespace [%s]", namespace.GetName(), err)
			return nil, err
		}

		for _, deployment := range deployments {
			serviceGroupStatus := []appsv1alpha1.ServiceGroupStatus{}
			for key, value := range deployment.GetLabels() {
				if strings.HasPrefix(key, constants.IMServiceGroupClientAssignedPrefix) {
					serviceGroupName := strings.TrimPrefix(key, constants.IMServiceGroupClientAssignedPrefix)
					serviceGroupStatus = append(serviceGroupStatus, appsv1alpha1.ServiceGroupStatus{
						Name:   serviceGroupName,
						IsUsed: value != "",
					})
				}
			}

			serverInstance := appsv1alpha1.InferModelServer{
				Name:          deployment.GetName(),
				Namespace:     deployment.GetNamespace(),
				ServiceGroups: serviceGroupStatus,
			}
			servers = append(servers, serverInstance)
		}
	}

	result.Servers = servers

	return result, nil
}

func GetDevTypeOnNode(nodeLabels map[string]string) (devType, devModel string) {
	devType = constants.EdgeNodeTypeCommon
	if len(nodeLabels) == 0 {
		return
	}

	npuDevModel, ok := nodeLabels[constants.EdgeNodeNPULabel]
	if ok {
		devType = constants.EdgeNodeTypeNPU
		devModel = npuDevModel
		return
	}

	gpuDevModel, ok := nodeLabels[constants.EdgeNodeGPULabel]
	if ok {
		devType = constants.EdgeNodeTypeGPU
		devModel = gpuDevModel
	}

	return
}

func isVirtualized(nodeLabels map[string]string) (virtualized bool) {
	if len(nodeLabels) == 0 {
		return
	}

	_, virtualized = nodeLabels[constants.EdgeNodeVirtualizationLabel]
	return
}

func (o *imdOperator) GetSpecificDevResTemplates(cm *corev1.ConfigMap, devType, devModel string, virtualized bool, nodeName string) (items []appsv1alpha1.TemplateItem, err error) {
	items = []appsv1alpha1.TemplateItem{}
	dataBytes, ok := cm.Data[constants.ResTemplateKey]
	if !ok {
		klog.Warningf("resTemplate [%s] for GPU/NPU is emtpy", cm.GetName())
		return
	}

	templateConfig := appsv1alpha1.ResourceTemplateConfig{}
	err = yaml.Unmarshal([]byte(dataBytes), &templateConfig)
	if err != nil {
		klog.Errorf("parse resTemplate [%s] failed", cm.GetName())
		return
	}

	items = o.ParseDevResTemplate(templateConfig, devType, devModel, virtualized, nodeName)
	return
}

func (o *imdOperator) ParseDevResTemplate(
	resTemplateConfig appsv1alpha1.ResourceTemplateConfig,
	devType string, devModel string, virtualized bool, nodeName string) (items []appsv1alpha1.TemplateItem) {
	items = []appsv1alpha1.TemplateItem{}

	var devResTemplate appsv1alpha1.DevResourceTemplate
	switch devType {
	case constants.EdgeNodeTypeNPU:
		devResTemplate = resTemplateConfig.NPU
	case constants.EdgeNodeTypeGPU:
		devResTemplate = resTemplateConfig.GPU
	default:
		klog.Warningf("devType only support NPU/GPU now, but it is %s", devType)
		return
	}

	var devModelDetails []appsv1alpha1.TemplateDetail
	var found bool
	if virtualized && devResTemplate.Virtualized != nil {
		if devModelDetails, found = devResTemplate.Virtualized[devModel]; !found {
			klog.Warningf("template not found for virtualized devType [%s] devModel [%s] ", devType, devModel)
			return
		}

		// Add Hardware info
		devModelDetails = AddHardwareInfoToVirtualTemplateDesc(devModel, devModelDetails)
	} else if !virtualized {
		var err error
		// if Physical Template not specified by admin, Get from Node Capacity
		if devResTemplate.Physical != nil {
			devModelDetails, found = devResTemplate.Physical[devModel]
			if !found {
				devModelDetails, err = o.GetPhysicalTemplateEntriesFromNodeStatus(nodeName, devModel)
			}
		} else {
			devModelDetails, err = o.GetPhysicalTemplateEntriesFromNodeStatus(nodeName, devModel)
		}

		if err != nil {
			klog.Errorf("get Physical template failed on node [%s] for device [%s]", nodeName, devModel)
			return
		}

	} else {
		klog.Warningf("this resTemplate for devType [%s] devModel [%s] is empty", devType, devModel)
		return
	}

	for index, template := range devModelDetails {
		item := appsv1alpha1.TemplateItem{
			ResourceName:    fmt.Sprintf("template-%d", index),
			Description:     template.Description,
			ResourceRequest: template.ResourceRequest,
		}

		items = append(items, item)
	}

	return
}

func (o *imdOperator) GetPhysicalTemplateEntriesFromNodeStatus(nodeName, devModel string) (entries []appsv1alpha1.TemplateDetail, err error) {
	entries = []appsv1alpha1.TemplateDetail{}
	ctx := context.Background()
	nodeInfo, err := o.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("get node %s failed, %v", nodeName, err)
		return
	}

	resourceName := GetResourceName(devModel)
	if resourceName == "" {
		err = fmt.Errorf("not support device %s now", devModel)
		return
	}

	maxChips, ok := nodeInfo.Status.Capacity[corev1.ResourceName(resourceName)]
	if !ok {
		err = fmt.Errorf("resource %s not registered on Node %s", resourceName, nodeName)
		return
	}

	maxChipCount, err := strconv.Atoi(maxChips.String())
	if err != nil {
		err = fmt.Errorf("parse resource %s:%s failed, %v", resourceName, maxChips.String(), err)
		return
	}

	for i := 1; i <= maxChipCount; i++ {
		entries = append(entries, appsv1alpha1.TemplateDetail{
			Description:     GetPhysicalDesc(devModel, i),
			ResourceRequest: map[string]int{resourceName: i},
		})
	}

	return
}

func GetPhysicalDesc(devModel string, count int) string {
	var dev string
	if strings.HasPrefix(devModel, constants.DeviceHuaweiPrefix) {
		dev = strings.TrimPrefix(devModel, constants.DeviceHuaweiPrefix)
	} else {
		dev = constants.DeviceNvidiaCommon
	}

	return fmt.Sprintf("%d Chip (%s)", count, dev)
}

func GetResourceName(deviceName string) string {
	switch {
	case deviceName == constants.DeviceAscend310:
		return constants.ResourceAscend310
	case deviceName == constants.DeviceAscend310P:
		return constants.ResourceAscend310P
	case strings.Contains(strings.ToLower(deviceName), constants.DeviceNvidiaCommon):
		return constants.ResourceNvidia
	default:
	}

	return ""
}

func AddHardwareInfoToVirtualTemplateDesc(devModel string, devModelDetails []appsv1alpha1.TemplateDetail) []appsv1alpha1.TemplateDetail {
	var dev string
	if strings.HasPrefix(devModel, constants.DeviceHuaweiPrefix) {
		dev = strings.TrimPrefix(devModel, constants.DeviceHuaweiPrefix)
	} else {
		dev = constants.DeviceNvidiaCommon
	}

	result := []appsv1alpha1.TemplateDetail{}
	for _, item := range devModelDetails {
		item.Description = fmt.Sprintf("%s (%s)", item.Description, dev)
		result = append(result, item)
	}

	return result
}
