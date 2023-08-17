// Copyright 2022 The KubeSphere Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package apptemplate

import (
	"context"
	"sort"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/klog/v2"

	"github.com/edgewize-io/edgewize/pkg/api"
	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/api/apps/v1alpha1"
	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	appslisteners "github.com/edgewize-io/edgewize/pkg/client/listers/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/informers"
	resources "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3"
)

type Operator interface {
	ListAppTemplate(ctx context.Context, workspace string, queryParam *query.Query) (*api.ListResult, error)
	GetAppTemplate(ctx context.Context, workspace, name string) (*appsv1alpha1.AppTemplate, error)
}

func NewAppTemplateOperator(informers informers.InformerFactory) Operator {
	return &appSetOperator{
		appTemplateLister:          informers.KubeSphereSharedInformerFactory().Apps().V1alpha1().AppTemplates().Lister(),
		appTemplateVersionListener: informers.KubeSphereSharedInformerFactory().Apps().V1alpha1().AppTemplateVersions().Lister(),
	}
}

type appSetOperator struct {
	appTemplateLister          appslisteners.AppTemplateLister
	appTemplateVersionListener appslisteners.AppTemplateVersionLister
}

func (o *appSetOperator) GetAppTemplate(ctx context.Context, workspace, name string) (*appsv1alpha1.AppTemplate, error) {
	appTemplate, err := o.appTemplateLister.Get(name)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("get appTemplate: %v", appTemplate)
	return o.updateVersions(appTemplate)
}

func (o *appSetOperator) listAppTemplates(ctx context.Context, workspace string, selector labels.Selector) ([]runtime.Object, error) {
	if workspace != "" {
		requirement, err := labels.NewRequirement(apisappsv1alpha1.LabelWorkspace, selection.Equals, []string{workspace})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*requirement)
	}
	appTemplates, err := o.appTemplateLister.List(selector)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("list appTemplates: %v, size: %d", appTemplates, len(appTemplates))
	var appSets = make([]runtime.Object, 0)

	for _, appTemplate := range appTemplates {
		ret, err := o.updateVersions(appTemplate)
		if err != nil {
			return nil, err
		}
		appSets = append(appSets, ret)
	}
	return appSets, nil
}

func (o *appSetOperator) ListAppTemplate(ctx context.Context, workspace string, queryParam *query.Query) (*api.ListResult, error) {
	appTemplates, err := o.listAppTemplates(ctx, workspace, queryParam.Selector())
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("list appTemplates: %v", appTemplates)

	listResult := resources.DefaultList(appTemplates, queryParam, func(left, right runtime.Object, field query.Field) bool {
		klog.V(4).Infof("compare appTemplate: %v, %v", left, right)
		return resources.DefaultObjectMetaCompare(left.(*appsv1alpha1.AppTemplate).ObjectMeta, right.(*appsv1alpha1.AppTemplate).ObjectMeta, field)
	}, func(obj runtime.Object, filter query.Filter) bool {
		klog.V(4).Infof("filter appTemplate: %v", obj)
		return resources.DefaultObjectMetaFilter(obj.(*appsv1alpha1.AppTemplate).ObjectMeta, filter)
	})

	return listResult, nil
}

func (o *appSetOperator) updateVersions(appTemplate *apisappsv1alpha1.AppTemplate) (*appsv1alpha1.AppTemplate, error) {
	ret := &appsv1alpha1.AppTemplate{
		AppTemplate: *appTemplate,
		Spec: appsv1alpha1.AppTemplateSpec{
			LatestVersion: "",
			VersionList:   make([]*apisappsv1alpha1.AppTemplateVersion, 0),
		},
	}
	selector := labels.SelectorFromSet(map[string]string{apisappsv1alpha1.LabelAppTemplate: appTemplate.Name})
	klog.V(4).Infof("list appTemplateVersions: %v", selector)
	list, err := o.appTemplateVersionListener.List(selector)
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
	ret.Spec.LatestVersion = list[0].Labels[apisappsv1alpha1.LabelAppTemplateVersion]
	ret.Spec.VersionList = list
	klog.V(4).Infof("update appTemplateVersions: %v", ret)
	return ret, nil
}
