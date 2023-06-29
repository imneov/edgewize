/*
Copyright 2020 The KubeSphere Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package externalversions

import (
	"fmt"

	v2beta1 "github.com/edgewize-io/edgewize/pkg/apis/alerting/v2beta1"
	v1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	cache "k8s.io/client-go/tools/cache"
)

// GenericInformer is type of SharedIndexInformer which will locate and delegate to other
// sharedInformers based on type
type GenericInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() cache.GenericLister
}

type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}

// ForResource gives generic access to a shared informer of the matching type
// TODO extend this to unknown resources with a client pool
func (f *sharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	switch resource {
	// Group=alerting.kubesphere.io, Version=v2beta1
	case v2beta1.SchemeGroupVersion.WithResource("clusterrulegroups"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Alerting().V2beta1().ClusterRuleGroups().Informer()}, nil
	case v2beta1.SchemeGroupVersion.WithResource("globalrulegroups"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Alerting().V2beta1().GlobalRuleGroups().Informer()}, nil
	case v2beta1.SchemeGroupVersion.WithResource("rulegroups"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Alerting().V2beta1().RuleGroups().Informer()}, nil

		// Group=apps.edgewize.io, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithResource("apptemplates"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Apps().V1alpha1().AppTemplates().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("apptemplateversions"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Apps().V1alpha1().AppTemplateVersions().Informer()}, nil
	case v1alpha1.SchemeGroupVersion.WithResource("edgeappsets"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Apps().V1alpha1().EdgeAppSets().Informer()}, nil

		// Group=infra.edgewize.io, Version=v1alpha1
	case infrav1alpha1.SchemeGroupVersion.WithResource("clusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Infra().V1alpha1().Clusters().Informer()}, nil
	case infrav1alpha1.SchemeGroupVersion.WithResource("edgeclusters"):
		return &genericInformer{resource: resource.GroupResource(), informer: f.Infra().V1alpha1().EdgeClusters().Informer()}, nil

	}

	return nil, fmt.Errorf("no informer found for %v", resource)
}
