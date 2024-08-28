package infermodel

import (
	"context"
	"github.com/edgewize-io/edgewize/pkg/api"
	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/api/apps/v1alpha1"
	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	appslisteners "github.com/edgewize-io/edgewize/pkg/client/listers/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/informers"
	resources "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"
	"sort"
)

type IMTOperator interface {
	GetInferModelTemplate(ctx context.Context, workspace, name string) (*appsv1alpha1.InferModelTemplate, error)
	ListInferModelTemplate(ctx context.Context, workspace string, queryParam *query.Query) (*api.ListResult, error)
}

type imtOperator struct {
	imTemplateListener        appslisteners.InferModelTemplateLister
	imTemplateVersionListener appslisteners.InferModelTemplateVersionLister
}

func NewInferModelTemplateOperator(informers informers.InformerFactory) IMTOperator {
	return &imtOperator{
		imTemplateListener:        informers.KubeSphereSharedInformerFactory().Apps().V1alpha1().InferModelTemplates().Lister(),
		imTemplateVersionListener: informers.KubeSphereSharedInformerFactory().Apps().V1alpha1().InferModelTemplateVersions().Lister(),
	}
}

func (i *imtOperator) GetInferModelTemplate(ctx context.Context, workspace, name string) (*appsv1alpha1.InferModelTemplate, error) {
	imTemplate, err := i.imTemplateListener.Get(name)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("get imTemplate: %v", imTemplate)
	return i.updateVersions(imTemplate)
}

func (i *imtOperator) listInferModelTemplates(ctx context.Context, workspace string, selector labels.Selector) ([]runtime.Object, error) {
	if workspace != "" {
		requirement, err := labels.NewRequirement(apisappsv1alpha1.LabelWorkspace, selection.Equals, []string{workspace})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*requirement)
	}
	imTemplates, err := i.imTemplateListener.List(selector)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("list imTemplates: %v, size: %d", imTemplates, len(imTemplates))
	var imSets = make([]runtime.Object, 0)

	for _, imTemplate := range imTemplates {
		ret, err := i.updateVersions(imTemplate)
		if err != nil {
			return nil, err
		}
		imSets = append(imSets, ret)
	}
	return imSets, nil
}

func (i *imtOperator) ListInferModelTemplate(ctx context.Context, workspace string, queryParam *query.Query) (*api.ListResult, error) {
	imTemplates, err := i.listInferModelTemplates(ctx, workspace, queryParam.Selector())
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("list imTemplates: %v", imTemplates)

	listResult := resources.DefaultList(imTemplates, queryParam, func(left, right runtime.Object, field query.Field) bool {
		klog.V(4).Infof("compare imTemplate: %v, %v", left, right)
		return resources.DefaultObjectMetaCompare(left.(*appsv1alpha1.InferModelTemplate).ObjectMeta, right.(*appsv1alpha1.InferModelTemplate).ObjectMeta, field)
	}, func(obj runtime.Object, filter query.Filter) bool {
		klog.V(4).Infof("filter imTemplate: %v", obj)
		return resources.DefaultObjectMetaFilter(obj.(*appsv1alpha1.InferModelTemplate).ObjectMeta, filter)
	})

	return listResult, nil
}

func (i *imtOperator) updateVersions(imTemplate *apisappsv1alpha1.InferModelTemplate) (*appsv1alpha1.InferModelTemplate, error) {
	ret := &appsv1alpha1.InferModelTemplate{
		InferModelTemplate: *imTemplate,
		Spec: appsv1alpha1.InferModelTemplateSpec{
			LatestVersion: "",
			ServiceGroup:  imTemplate.Spec.ServiceGroup,
			VersionList:   make([]*apisappsv1alpha1.InferModelTemplateVersion, 0),
		},
	}
	selector := labels.SelectorFromSet(map[string]string{apisappsv1alpha1.LabelIMTemplate: imTemplate.Name})
	klog.V(4).Infof("list imTemplateVersions: %v", selector)
	list, err := i.imTemplateVersionListener.List(selector)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return ret, nil
	}
	// sort by create timestamp
	sort.Slice(list, func(i, j int) bool {
		return list[j].CreationTimestamp.Before(&list[i].CreationTimestamp)
	})
	ret.Spec.LatestVersion = list[0].Labels[apisappsv1alpha1.LabelIMTemplateVersion]
	ret.Spec.VersionList = list
	klog.V(4).Infof("update inferModelTemplateVersions: %v", ret)
	return ret, nil
}
