package imservicegroup

import (
	"context"
	"fmt"
	"github.com/edgewize-io/edgewize/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"time"
)

const IMServiceGroup = "imServiceGroup"

// Reconciler reconciles a Workspace object
type Reconciler struct {
	client.Client
	Recorder                record.EventRecorder
	MaxConcurrentReconciles int
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor(IMServiceGroup)
	}
	if r.MaxConcurrentReconciles <= 0 {
		r.MaxConcurrentReconciles = 1
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named(IMServiceGroup).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		For(&appsv1.Deployment{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=,resources=deployments/status,verbs=get

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	rootCtx := context.Background()
	instance := &appsv1.Deployment{}
	if err := r.Get(rootCtx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	podLabels := instance.Spec.Template.GetLabels()
	if podLabels == nil {
		return ctrl.Result{}, nil
	}

	enabled, ok := podLabels[constants.WebhookApplicationInjectLabel]
	if !ok || enabled != constants.WebhookInjectEnable {
		return ctrl.Result{}, nil
	}

	serverDeployName, ok := podLabels[constants.IMServerDeployNameLabel]
	if !ok {
		return ctrl.Result{}, nil
	}

	serverDeployNamespace, ok := podLabels[constants.IMServerDeployNamespaceLabel]
	if !ok {
		return ctrl.Result{}, nil
	}

	serverDeploy := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: serverDeployName, Namespace: serverDeployNamespace}, serverDeploy)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		klog.Errorf("get in server deployment [%s/%s] failed", serverDeployNamespace, serverDeployName)
		return ctrl.Result{}, err
	}

	serverDeploy = serverDeploy.DeepCopy()

	currServiceGroup, ok := podLabels[constants.IMProxyServiceGroupLabel]
	if !ok || currServiceGroup == constants.IMDefaultServiceGroup {
		klog.V(4).Infof("client deployment [%s/%s] join DEFAULT service group", instance.Namespace, instance.Name)
		return ctrl.Result{}, nil
	}

	currAssignedClientName := fmt.Sprintf("%s-%s", instance.GetName(), instance.GetNamespace())
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		err = r.assignServiceGroup(ctx, serverDeploy, currServiceGroup, currAssignedClientName)
		if err != nil {
			klog.Errorf("assign server [%s/%s] serviceGroup [%s] label failed, %v",
				serverDeployNamespace, serverDeployName, currServiceGroup, err)
		}
	} else {
		err = r.unassignServiceGroup(ctx, serverDeploy, currServiceGroup, currAssignedClientName)
		if err != nil {
			klog.Errorf("unassign server [%s/%s] serviceGroup [%s] label failed, %v",
				serverDeployNamespace, serverDeployName, currServiceGroup, err)
		}
	}

	return ctrl.Result{RequeueAfter: 3 * time.Second}, err
}

func (r *Reconciler) assignServiceGroup(ctx context.Context, serverDeploy *appsv1.Deployment, serviceGroup, currAssignedClientName string) (err error) {
	serverDeployLabels := serverDeploy.GetLabels()
	if serverDeployLabels == nil {
		serverDeployLabels = make(map[string]string, 1)
	}

	assignedLabelKey := constants.IMServiceGroupClientAssignedPrefix + serviceGroup
	assignedClientName, ok := serverDeployLabels[assignedLabelKey]
	if !ok {
		klog.Warningf("serverGroup [%s] not support in server [%s/%s]",
			serviceGroup, serverDeploy.GetNamespace(), serverDeploy.GetName())
		return
	}

	if assignedClientName != "" && assignedClientName != currAssignedClientName {
		err = fmt.Errorf("server [%s/%s] serviceGroup %s had been assigned to [%s], fail to assign to %s",
			serverDeploy.GetNamespace(), serverDeploy.GetName(),
			serviceGroup, assignedClientName, currAssignedClientName)
		return
	}

	serverDeployLabels[assignedLabelKey] = currAssignedClientName
	err = r.Update(ctx, serverDeploy)
	return
}

func (r *Reconciler) unassignServiceGroup(ctx context.Context, serverDeploy *appsv1.Deployment, serviceGroup, currAssignedClientName string) (err error) {
	serverDeployLabels := serverDeploy.GetLabels()
	if serverDeployLabels == nil {
		return
	}

	assignedLabelKey := constants.IMServiceGroupClientAssignedPrefix + serviceGroup
	assignedClientName, ok := serverDeployLabels[assignedLabelKey]
	if !ok || assignedClientName != currAssignedClientName {
		return
	}

	serverDeployLabels[assignedLabelKey] = ""
	err = r.Update(ctx, serverDeploy)
	return
}
