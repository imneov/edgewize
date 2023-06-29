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
package edgeappset

import (
	"context"

	"github.com/edgewize-io/edgewize/pkg/api"
	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/api/apps/v1alpha1"
	apisappsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	appslisteners "github.com/edgewize-io/edgewize/pkg/client/listers/apps/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/informers"
	resources "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	appsv1 "k8s.io/client-go/listers/apps/v1"
)

type Operator interface {
	ListEdgeAppSets(ctx context.Context, namespace string, queryParam *query.Query) (*api.ListResult, error)
	GetEdgeAppSet(ctx context.Context, namespace, name string) (*appsv1alpha1.EdgeAppSet, error)
}

func NewAppSetOperator(informers informers.InformerFactory) Operator {
	return &appSetOperator{
		edgeAppSetLister:   informers.KubeSphereSharedInformerFactory().Apps().V1alpha1().EdgeAppSets().Lister(),
		deploymentListener: informers.KubernetesSharedInformerFactory().Apps().V1().Deployments().Lister(),
	}
}

type appSetOperator struct {
	edgeAppSetLister   appslisteners.EdgeAppSetLister
	deploymentListener appsv1.DeploymentLister
}

func (o *appSetOperator) GetEdgeAppSet(ctx context.Context, namespace, name string) (*appsv1alpha1.EdgeAppSet, error) {
	edgeAppSet, err := o.edgeAppSetLister.EdgeAppSets(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	return o.updateEdgeAppSet(edgeAppSet)
}

func (o *appSetOperator) listEdgeAppSets(ctx context.Context, namespace string, selector labels.Selector) ([]runtime.Object, error) {
	edgeAppSets, err := o.edgeAppSetLister.EdgeAppSets(namespace).List(selector)
	if err != nil {
		return nil, err
	}
	var appSets = make([]runtime.Object, 0)

	for _, edgeAppSet := range edgeAppSets {
		ret, err := o.updateEdgeAppSet(edgeAppSet)
		if err != nil {
			return nil, err
		}
		appSets = append(appSets, ret)
	}
	return appSets, nil
}

func (o *appSetOperator) ListEdgeAppSets(ctx context.Context, namespace string, queryParam *query.Query) (*api.ListResult, error) {
	edgeAppSets, err := o.listEdgeAppSets(ctx, namespace, queryParam.Selector())
	if err != nil {
		return nil, err
	}

	listResult := resources.DefaultList(edgeAppSets, queryParam, func(left, right runtime.Object, field query.Field) bool {
		return resources.DefaultObjectMetaCompare(left.(*appsv1alpha1.EdgeAppSet).ObjectMeta, right.(*appsv1alpha1.EdgeAppSet).ObjectMeta, field)
	}, func(obj runtime.Object, filter query.Filter) bool {
		return resources.DefaultObjectMetaFilter(obj.(*appsv1alpha1.EdgeAppSet).ObjectMeta, filter)
	})

	return listResult, nil
}

func (o *appSetOperator) updateEdgeAppSet(edgeAppSet *apisappsv1alpha1.EdgeAppSet) (*appsv1alpha1.EdgeAppSet, error) {
	ret := &appsv1alpha1.EdgeAppSet{
		EdgeAppSet: *edgeAppSet,
		Status: appsv1alpha1.EdgeAppSetStatus{
			WorkloadStats: appsv1alpha1.WorkloadStats{},
			Workloads:     make([]*v1.Deployment, 0),
		},
	}
	selector := labels.SelectorFromSet(map[string]string{apisappsv1alpha1.LabelEdgeAppSet: edgeAppSet.Name})
	list, err := o.deploymentListener.Deployments(edgeAppSet.Namespace).List(selector)
	if err != nil {
		return nil, err
	}
	for _, deployment := range list {
		status := apisappsv1alpha1.Progressing
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
		} else if status == apisappsv1alpha1.Progressing {
			ret.Status.WorkloadStats.Progressing++
		}
		ret.Status.Workloads = append(ret.Status.Workloads, deployment.DeepCopy())
	}

	return ret, nil
}
