package imdeployment

import (
	"context"
	"fmt"
	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/constants"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strconv"
	"time"
)

const (
	IMDControllerName = "imDeployment"
	IMDFinalizer      = "imdeployment.finalizer.apps.edgewize.io"
)

var (
	DefaultMinHostPort int
	DefaultMaxHostPort int
)

// Reconciler reconciles a Workspace object
type Reconciler struct {
	client.Client
	client.Reader
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Reader == nil {
		r.Reader = mgr.GetAPIReader()
	}

	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(IMDControllerName)
	}

	if r.MaxConcurrentReconciles <= 0 {
		r.MaxConcurrentReconciles = 1
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(IMDControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		WithEventFilter(predicate.GenerationChangedPredicate{
			Funcs: predicate.Funcs{
				UpdateFunc: func(e event.UpdateEvent) bool {
					oldDeploy := e.ObjectOld.(*apisappsv1alpha1.InferModelDeployment)
					newDeploy := e.ObjectNew.(*apisappsv1alpha1.InferModelDeployment)
					return !equality.Semantic.DeepEqual(oldDeploy.Spec, newDeploy.Spec)
				},
			},
		}).
		For(&apisappsv1alpha1.InferModelDeployment{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=,resources=deployments/status,verbs=get

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("received imDeployment reconcile request, %v", req)

	rootCtx := context.Background()
	instance := &apisappsv1alpha1.InferModelDeployment{}
	if err := r.Reader.Get(rootCtx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !sliceutil.HasString(instance.ObjectMeta.Finalizers, IMDFinalizer) {
			klog.V(4).Infof("inferModelDeployment is created, add finalizer and update, req: %v, finalizer: %s", req, IMDFinalizer)
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *apisappsv1alpha1.InferModelDeployment) error {
				_instance.ObjectMeta.Finalizers = append(_instance.ObjectMeta.Finalizers, IMDFinalizer)
				return r.Update(rootCtx, _instance)
			}); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		if sliceutil.HasString(instance.ObjectMeta.Finalizers, IMDFinalizer) {
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *apisappsv1alpha1.InferModelDeployment) error {
				// remove our finalizer from the list and update it.
				_instance.ObjectMeta.Finalizers = sliceutil.RemoveString(_instance.ObjectMeta.Finalizers, func(item string) bool {
					return item == IMDFinalizer
				})
				klog.V(4).Infof("remove finalizer from inferModelDeployment %s", req.Name)
				return r.Update(rootCtx, _instance)
			}); err != nil {
				klog.Error("update inferModelDeployment %s finalizer failed, err: %v", req.Name, err)
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	return r.doReconcile(ctx, instance)
}

func (r *Reconciler) UpdateInstance(ctx context.Context, nn types.NamespacedName, updateFunc func(*apisappsv1alpha1.InferModelDeployment) error) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		instance := &apisappsv1alpha1.InferModelDeployment{}
		if err := r.Reader.Get(ctx, nn, instance); err != nil {
			return client.IgnoreNotFound(err)
		}
		return updateFunc(instance)
	})
}

func (r *Reconciler) UpdateStatus(ctx context.Context, instance *apisappsv1alpha1.InferModelDeployment) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		return r.Status().Update(ctx, instance)
	})
}

func (r *Reconciler) doReconcile(ctx context.Context, oldInstance *apisappsv1alpha1.InferModelDeployment) (ctrl.Result, error) {
	instance := oldInstance.DeepCopy()

	klog.V(4).Infof("start reconcile, name: %s, namespace: %s", instance.Name, instance.Namespace)

	var err error
	var updated bool
	// sync image pull secrets
	if instance.Annotations == nil || instance.Annotations[apisappsv1alpha1.ImagePullSecretAnnotation] == "" {
		updated, err = r.syncImagePullSecret(instance)
		if err != nil {
			klog.Errorf("sync image pull secret failed: %v", err)
			return ctrl.Result{}, err
		}

		if updated {
			klog.V(4).Infof("sync image secret for imDeployment [%s/%s]", instance.GetNamespace(), instance.GetName())
		}
	}

	// select random HostPort
	updated, err = r.setNodeHostPort(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	// attach hostPort to EdgeNode annotation
	if updated {
		klog.V(4).Infof("update hostPost for imDeployment [%s/%s]", instance.GetNamespace(), instance.GetName())
		err = r.Update(ctx, instance)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	r.deployWorkloads(ctx, instance)

	err = r.UpdateStatus(ctx, instance)
	if err != nil {
		klog.Errorf("update imd status failed, name: %s, err: %v", instance.Name, err)
	}

	klog.V(4).Infof("one reconcile over, name: %s, namespace: %s", instance.Name, instance.Namespace)
	return ctrl.Result{}, err
}

func (r *Reconciler) deployWorkloads(ctx context.Context, instance *apisappsv1alpha1.InferModelDeployment) {
	notDeployedWorkloads := []string{}
	expectedWorkloads := r.getWorkloadInfos(instance)
	for uniName := range expectedWorkloads {
		if instance.Status.WorkLoadInstances != nil {
			workloadStatus, found := instance.Status.WorkLoadInstances[uniName]
			if found && workloadStatus.Deployed {
				continue
			}
		}

		notDeployedWorkloads = append(notDeployedWorkloads, uniName)
	}

	if len(notDeployedWorkloads) == 0 {
		klog.V(4).Infof("all workloads are deployed for [%s/%s] imDeployment", instance.Namespace, instance.Name)
		return
	}

	if instance.Status.WorkLoadInstances == nil {
		instance.Status.WorkLoadInstances = make(map[string]apisappsv1alpha1.WorkLoadStatus, len(notDeployedWorkloads))
	}

	for _, name := range notDeployedWorkloads {
		var deployName string
		workloadStatus, ok := instance.Status.WorkLoadInstances[name]
		if ok {
			deployName = workloadStatus.Name
		} else {
			deployName = fmt.Sprintf("%s-%s", instance.Name, rand.String(5))
			workloadStatus = apisappsv1alpha1.WorkLoadStatus{
				Name:     deployName,
				Deployed: false,
			}
		}

		var createDeploy *appsv1.Deployment
		createDeploy, err := r.buildDeployment(deployName, instance, expectedWorkloads[name])
		if err != nil {
			klog.Errorf("build deploy for workload %s failed, %v", name, err)
			return
		}

		err = r.Create(ctx, createDeploy)
		if err != nil {
			klog.Errorf("create deployment failed, name: %s, err: %v", deployName, err)
			workloadStatus.Deployed = false
			workloadStatus.Message = err.Error()
		} else {
			klog.V(3).Infof("create deployment [%s/%s] successfully as workload for imDeployment [%s/%s]",
				createDeploy.GetNamespace(), createDeploy.GetName(), instance.GetNamespace(), instance.GetName())
			workloadStatus.Deployed = true
			workloadStatus.Message = ""
		}

		workloadStatus.LastTransitionTime = metav1.Time{Time: time.Now()}
		instance.Status.WorkLoadInstances[name] = workloadStatus
	}
}

func (r *Reconciler) syncImagePullSecret(instance *apisappsv1alpha1.InferModelDeployment) (updated bool, err error) {
	if instance.Annotations == nil {
		instance.Annotations = map[string]string{}
	}
	if instance.Annotations[apisappsv1alpha1.ImagePullSecretAnnotation] != "" {
		klog.V(4).Infof("image pull secret already exists, skip sync")
		return
	}

	imagePullSecrets := instance.Spec.DeploymentTemplate.Spec.Template.Spec.ImagePullSecrets
	if len(imagePullSecrets) == 0 {
		return
	}

	for _, imagePullSecret := range imagePullSecrets {
		originSecret := &corev1.Secret{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Name: imagePullSecret.Name, Namespace: constants.DefaultNamespace}, originSecret)
		if err != nil {
			klog.Errorf("get image pull secret failed, name: %s, err: %v", imagePullSecret.Name, err)
			return
		}
		addedNamespace := map[string]bool{}
		for _, selector := range instance.Spec.NodeSelectors {
			if addedNamespace[selector.Project] {
				continue
			}
			targetSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      originSecret.Name,
					Namespace: selector.Project,
				},
				Immutable:  originSecret.Immutable,
				Data:       originSecret.Data,
				StringData: originSecret.StringData,
				Type:       originSecret.Type,
			}
			err = r.Create(context.Background(), targetSecret)
			if err != nil {
				if apierrors.IsAlreadyExists(err) {
					klog.V(4).Infof("image pull secret already exists, secretName: %s, namespace: %s", imagePullSecret.Name, selector.Project)
				} else {
					klog.Error(err)
					return
				}
			}
			addedNamespace[selector.Project] = true
		}
	}
	klog.V(3).Infof("sync image pull secret for instance %s success", instance.Name)
	instance.Annotations[apisappsv1alpha1.ImagePullSecretAnnotation] = "true"
	updated = true
	return
}

func (r *Reconciler) setNodeHostPort(ctx context.Context, instance *apisappsv1alpha1.InferModelDeployment) (updated bool, err error) {
	klog.V(4).Infof("start to set node hostPort for imDeployment [%s/%s]", instance.GetNamespace(), instance.GetName())
	newModelNodeSelector := []apisappsv1alpha1.ModelNodeSelector{}
	for _, selectorItem := range instance.Spec.NodeSelectors {
		if selectorItem.HostPort != "" {
			continue
		}

		nodeInfo := &corev1.Node{}
		err = r.Client.Get(ctx, types.NamespacedName{Name: selectorItem.NodeName}, nodeInfo)
		if err != nil {
			klog.Errorf("get node [%s] info failed, %v", selectorItem.NodeName, err)
			return
		}

		nodeInfo = nodeInfo.DeepCopy()
		nodeAnnotations := nodeInfo.GetAnnotations()
		if nodeAnnotations == nil {
			nodeAnnotations = make(map[string]string, 1)
		}

		var assignedHostPortValue string
		oldAvailHostPortValue := nodeAnnotations[constants.IMServerNextHostPortAnnotation]
		if oldAvailHostPortValue == "" {
			assignedHostPortValue = strconv.Itoa(DefaultMinHostPort)
		} else {
			assignedHostPortValue = oldAvailHostPortValue
		}

		// increased by 1
		var nodeNextAvailHostPortValue string
		nodeNextAvailHostPortValue, err = increasedByOne(assignedHostPortValue)
		if err != nil {
			klog.Errorf("get increased node [%s] next available hostPort from [%s] failed", nodeInfo.GetName(), assignedHostPortValue)
			return
		}

		nodeAnnotations[constants.IMServerNextHostPortAnnotation] = nodeNextAvailHostPortValue
		nodeInfo.Annotations = nodeAnnotations
		err = r.Client.Update(ctx, nodeInfo)
		if err != nil {
			klog.Errorf("fail to set node [%s] next available hostPort to [%s]", nodeInfo.GetName(), nodeNextAvailHostPortValue)
			return
		}

		klog.V(4).Infof("update edge node [%s] next available hostPort to [%s]", nodeInfo.GetName(), nodeNextAvailHostPortValue)
		selectorItem.HostPort = assignedHostPortValue
		newModelNodeSelector = append(newModelNodeSelector, selectorItem)
		updated = true
	}

	if !updated {
		klog.V(4).Infof("skip setting node hostPort for imDeployment [%s/%s]", instance.GetNamespace(), instance.GetName())
	} else {
		instance.Spec.NodeSelectors = newModelNodeSelector
	}
	return
}

func (r *Reconciler) getWorkloadInfos(instance *apisappsv1alpha1.InferModelDeployment) map[string]apisappsv1alpha1.ModelNodeSelector {
	result := make(map[string]apisappsv1alpha1.ModelNodeSelector)
	for _, item := range instance.Spec.NodeSelectors {
		uniqueName := fmt.Sprintf("%s-%s-%s", item.Project, item.NodeGroup, item.NodeName)
		result[uniqueName] = item
	}

	return result
}

func (r *Reconciler) buildDeployment(
	name string,
	instance *apisappsv1alpha1.InferModelDeployment,
	modelSelector apisappsv1alpha1.ModelNodeSelector,
) (deployment *appsv1.Deployment, err error) {

	podTemplateWithCustomResRequest, err := r.injectCustomConfigToPodTemplate(instance, modelSelector.ResourceRequest)
	if err != nil {
		return
	}

	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: modelSelector.Project,
			Labels: map[string]string{
				"app":                                   name,
				apisappsv1alpha1.LabelNodeGroup:         modelSelector.NodeGroup,
				apisappsv1alpha1.LabelNode:              modelSelector.NodeName,
				apisappsv1alpha1.LabelIMTemplate:        instance.Spec.IMTemplateName,
				apisappsv1alpha1.LabelIMTemplateVersion: instance.Spec.Version,
				apisappsv1alpha1.LabelIMDeployment:      instance.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: instance.Spec.DeploymentTemplate.Spec.Replicas,
			Template: podTemplateWithCustomResRequest,
			Strategy: instance.Spec.DeploymentTemplate.Spec.Strategy,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                              name,
					apisappsv1alpha1.LabelIMDeployment: instance.Name,
				},
			},
		},
	}

	err = r.injectAdditionalLabels(name, instance, modelSelector, deployment)
	if err != nil {
		return
	}

	// 部署到指定节点
	if modelSelector.NodeName != "" {
		deployment.Spec.Template.Spec.NodeSelector = map[string]string{
			corev1.LabelHostname: modelSelector.NodeName,
		}
	}
	return
}

func mergeLabels(additionLabels, originLabels map[string]string) map[string]string {
	if originLabels == nil {
		originLabels = make(map[string]string, len(additionLabels))
	}

	for key, value := range additionLabels {
		originLabels[key] = value
	}

	return originLabels
}

func (r *Reconciler) injectCustomConfigToPodTemplate(
	instance *apisappsv1alpha1.InferModelDeployment,
	resRequest map[string]int,
) (newPodTemplate corev1.PodTemplateSpec, err error) {
	newPodTemplate = instance.Spec.DeploymentTemplate.Spec.Template
	tolerations := []corev1.Toleration{
		{
			Key:      "aicp.group/worker",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:      "aicp.group/resource_group",
			Operator: corev1.TolerationOpExists,
		},
		{
			Key:      "node-role.kubernetes.io/edge",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			// Fix https://github.com/kubeedge/kubeedge/issues/3736
			Key:      corev1.TaintNodeUnreachable,
			Operator: corev1.TolerationOpExists,
		},
	}

	for _, toleration := range newPodTemplate.Spec.Tolerations {
		tolerations = append(tolerations, toleration)
	}

	newPodTemplate.Spec.Tolerations = tolerations

	//if len(resRequest) > 0 {
	//	newPodTemplate.Spec.SchedulerName = constants.AicpHamiSchedulerName
	//}

	containers := []corev1.Container{}
	for _, container := range newPodTemplate.Spec.Containers {
		for resKey, resValue := range resRequest {
			var quantity resource.Quantity
			quantity, err = resource.ParseQuantity(strconv.Itoa(resValue))
			if err != nil {
				klog.Errorf("failed to parse quantity for [%s]", resKey)
				return
			}

			if len(container.Resources.Limits) == 0 {
				container.Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
			}

			container.Resources.Limits[corev1.ResourceName(resKey)] = quantity
		}

		containers = append(containers, container)
	}

	newPodTemplate.Spec.Containers = containers
	return
}

func (r *Reconciler) injectAdditionalLabels(
	name string,
	instance *apisappsv1alpha1.InferModelDeployment,
	modelSelector apisappsv1alpha1.ModelNodeSelector,
	newDeployment *appsv1.Deployment,
) (err error) {
	versions := &apisappsv1alpha1.InferModelTemplateVersionList{}
	listOpts := []client.ListOption{
		client.MatchingLabels{
			apisappsv1alpha1.LabelIMTemplate:        instance.Spec.IMTemplateName,
			apisappsv1alpha1.LabelIMTemplateVersion: instance.Spec.Version,
		},
	}

	err = r.Client.List(context.Background(), versions, listOpts...)
	if err != nil {
		return
	}

	if len(versions.Items) == 0 {
		err = fmt.Errorf("imTemplate [%s] version [%s] not found", instance.Spec.IMTemplateName, instance.Spec.Version)
		return
	}

	templateLabels := map[string]string{
		"app":                                   name,
		apisappsv1alpha1.LabelNodeGroup:         modelSelector.NodeGroup,
		apisappsv1alpha1.LabelNode:              modelSelector.NodeName,
		apisappsv1alpha1.LabelIMTemplate:        instance.Spec.IMTemplateName,
		apisappsv1alpha1.LabelIMTemplateVersion: instance.Spec.Version,
		apisappsv1alpha1.LabelIMDeployment:      instance.Name,
		constants.WebhookModelServerInjectLabel: constants.WebhookInjectEnable,
		constants.IMServerDeployNameLabel:       newDeployment.GetName(),
		constants.IMServerDeployNamespaceLabel:  newDeployment.GetNamespace(),
	}

	if modelSelector.HostPort != "" {
		templateLabels[constants.IMServerBrokerHostPortLabel] = modelSelector.HostPort
	}

	for groupName, percent := range versions.Items[0].Spec.ServiceGroup {
		templateLabels[constants.ServiceGroupPrefix+groupName] = strconv.Itoa(percent)
		newDeployment.Labels[constants.IMServiceGroupClientAssignedPrefix+groupName] = ""
	}

	newDeployment.Spec.Template.Labels = mergeLabels(templateLabels, newDeployment.Spec.Template.Labels)
	return
}

func increasedByOne(hostPortValue string) (incrHostPortValue string, err error) {
	hostPort, err := strconv.Atoi(hostPortValue)
	if err != nil {
		return
	}

	// limited from 30550 to 31050
	incrHostPort := hostPort + 1
	if incrHostPort > DefaultMaxHostPort {
		incrHostPort = DefaultMinHostPort
	}

	incrHostPortValue = strconv.Itoa(incrHostPort)
	return
}

func init() {
	var err error
	DefaultMinHostPort, err = strconv.Atoi(os.Getenv("MIN_HOST_PORT"))
	if err != nil {
		DefaultMinHostPort = constants.DefaultMinHostPort
	}

	DefaultMaxHostPort, err = strconv.Atoi(os.Getenv("MAX_HOST_PORT"))
	if err != nil {
		DefaultMaxHostPort = constants.DefaultMaxHostPort
	}

	if DefaultMinHostPort >= DefaultMaxHostPort {
		DefaultMinHostPort = constants.DefaultMinHostPort
		DefaultMaxHostPort = constants.DefaultMaxHostPort
	}
}
