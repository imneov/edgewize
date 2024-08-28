package imtemplate

import (
	"context"
	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/utils/sliceutil"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	IMTControllerName = "imTemplate"
	IMTFinalizer      = "imTemplate.finalizer.apps.edgewize.io"
)

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
		r.Recorder = mgr.GetEventRecorderFor(IMTControllerName)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(IMTControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.MaxConcurrentReconciles,
		}).
		For(&apisappsv1alpha1.InferModelTemplate{}).
		Watches(
			&source.Kind{Type: &apisappsv1alpha1.InferModelTemplateVersion{}},
			handler.EnqueueRequestsFromMapFunc(r.syncServiceGroup),
		).Complete(r)
}

func (r *Reconciler) syncServiceGroup(obj client.Object) (templateRequests []reconcile.Request) {
	klog.V(4).Infof("infer model template version %s updated, trigger service group updating reconcile", obj.GetName())
	templateRequests = []reconcile.Request{}
	imTemplateName, ok := obj.GetLabels()[apisappsv1alpha1.LabelIMTemplate]
	if !ok {
		klog.Warningf("failed to get template label from imTemplateVersion %s", obj.GetName())
		return
	}

	templateRequests = append(templateRequests, reconcile.Request{types.NamespacedName{Name: imTemplateName}})
	return
}

// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeltemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps.edgewize.io,resources=infermodeltemplateversions,verbs=get;list;watch;delete

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	klog.Infof("received imTemplate reconcile request, %v", req)

	rootCtx := context.Background()
	instance := &apisappsv1alpha1.InferModelTemplate{}
	if err := r.Get(rootCtx, req.NamespacedName, instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !sliceutil.HasString(instance.ObjectMeta.Finalizers, IMTFinalizer) {
			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *apisappsv1alpha1.InferModelTemplate) error {
				_instance.ObjectMeta.Finalizers = append(_instance.ObjectMeta.Finalizers, IMTFinalizer)
				return r.Update(rootCtx, _instance)
			}); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}
	} else {
		if sliceutil.HasString(instance.ObjectMeta.Finalizers, IMTFinalizer) {
			if _, err := r.undoReconcile(instance); err != nil {
				klog.Errorf("undoReconcile failed, imTemplate name %s, err: %v", instance.Name, err)
			}

			if err := r.UpdateInstance(rootCtx, req.NamespacedName, func(_instance *apisappsv1alpha1.InferModelTemplate) error {
				// remove our finalizer from the list and update it.
				_instance.ObjectMeta.Finalizers = sliceutil.RemoveString(_instance.ObjectMeta.Finalizers, func(item string) bool {
					return item == IMTFinalizer
				})
				klog.V(4).Infof("add finalizer to InferModelTemplate %s", req.Name)
				return r.Update(rootCtx, _instance)
			}); err != nil {
				klog.Error("update imTemplate %s finalizer failed, err: %v", req.Name, err)
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	return r.doReconcile(instance)
}

func (r *Reconciler) UpdateInstance(ctx context.Context, nn types.NamespacedName, updateFunc func(*apisappsv1alpha1.InferModelTemplate) error) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		instance := &apisappsv1alpha1.InferModelTemplate{}
		if err := r.Get(ctx, nn, instance); err != nil {
			return client.IgnoreNotFound(err)
		}
		return updateFunc(instance)
	})
}

func (r *Reconciler) undoReconcile(instance *apisappsv1alpha1.InferModelTemplate) (ctrl.Result, error) {
	rootCtx := context.Background()
	var err error
	selector := labels.SelectorFromSet(map[string]string{apisappsv1alpha1.LabelIMTemplate: instance.Name})
	imTemplateVersions := apisappsv1alpha1.InferModelTemplateList{}
	if err = r.List(rootCtx, &imTemplateVersions, &client.ListOptions{LabelSelector: selector}); err == nil {
		for _, item := range imTemplateVersions.Items {
			err = r.Delete(rootCtx, &item)
		}
	}
	return ctrl.Result{}, err
}

func (r *Reconciler) doReconcile(instance *apisappsv1alpha1.InferModelTemplate) (ctrl.Result, error) {
	rootCtx := context.Background()
	var err error
	selector := labels.SelectorFromSet(map[string]string{apisappsv1alpha1.LabelIMTemplate: instance.Name})
	imTemplateVersions := apisappsv1alpha1.InferModelTemplateVersionList{}
	if err = r.List(rootCtx, &imTemplateVersions, &client.ListOptions{LabelSelector: selector}); err == nil {
		for _, item := range imTemplateVersions.Items {
			imTemplateVersion := item.DeepCopy()
			if reflect.DeepEqual(imTemplateVersion.Spec.ServiceGroup, instance.Spec.ServiceGroup) {
				continue
			}

			imTemplateVersion.Spec.ServiceGroup = instance.Spec.ServiceGroup
			err = r.Update(rootCtx, imTemplateVersion)
			if err != nil {
				klog.Error("sync serviceGroup from imTemplate %s to imTemplateVersion %s failed, err: %v",
					instance.GetName(), imTemplateVersion.GetName(), err)
				break
			}
		}
	}

	return ctrl.Result{}, err
}
