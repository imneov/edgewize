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

package resource

import (
	"errors"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	"github.com/edgewize-io/edgewize/pkg/api"
	infrav1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/infra/v1alpha1"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	"github.com/edgewize-io/edgewize/pkg/informers"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/cluster"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/configmap"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/cronjob"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/customresourcedefinition"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/daemonset"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/deployment"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/ingress"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/job"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/namespace"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/networkpolicy"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/node"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/persistentvolume"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/persistentvolumeclaim"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/pod"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/secret"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/service"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/serviceaccount"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/statefulset"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/volumesnapshot"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/volumesnapshotclass"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/volumesnapshotcontent"
)

var ErrResourceNotSupported = errors.New("resource is not supported")

type ResourceGetter struct {
	clusterResourceGetters    map[schema.GroupVersionResource]v1alpha3.Interface
	namespacedResourceGetters map[schema.GroupVersionResource]v1alpha3.Interface
}

func NewResourceGetter(factory informers.InformerFactory, cache cache.Cache) *ResourceGetter {
	namespacedResourceGetters := make(map[schema.GroupVersionResource]v1alpha3.Interface)
	clusterResourceGetters := make(map[schema.GroupVersionResource]v1alpha3.Interface)

	namespacedResourceGetters[schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}] = deployment.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}] = daemonset.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}] = statefulset.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}] = service.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}] = configmap.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}] = secret.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}] = pod.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}] = serviceaccount.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}] = ingress.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}] = networkpolicy.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}] = job.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "batch", Version: "v1beta1", Resource: "cronjobs"}] = cronjob.New(factory.KubernetesSharedInformerFactory())
	clusterResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumes"}] = persistentvolume.New(factory.KubernetesSharedInformerFactory())
	namespacedResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}] = persistentvolumeclaim.New(factory.KubernetesSharedInformerFactory(), factory.SnapshotSharedInformerFactory())
	namespacedResourceGetters[snapshotv1.SchemeGroupVersion.WithResource("volumesnapshots")] = volumesnapshot.New(factory.SnapshotSharedInformerFactory())
	clusterResourceGetters[snapshotv1.SchemeGroupVersion.WithResource("volumesnapshotclasses")] = volumesnapshotclass.New(factory.SnapshotSharedInformerFactory())
	clusterResourceGetters[snapshotv1.SchemeGroupVersion.WithResource("volumesnapshotcontents")] = volumesnapshotcontent.New(factory.SnapshotSharedInformerFactory())
	clusterResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}] = node.New(factory.KubernetesSharedInformerFactory())
	clusterResourceGetters[schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}] = namespace.New(factory.KubernetesSharedInformerFactory())
	clusterResourceGetters[schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}] = customresourcedefinition.New(factory.ApiExtensionSharedInformerFactory())

	// kubesphere resources
	clusterResourceGetters[infrav1alpha1.SchemeGroupVersion.WithResource(infrav1alpha1.ResourcesPluralCluster)] = cluster.New(factory.KubeSphereSharedInformerFactory())
	return &ResourceGetter{
		namespacedResourceGetters: namespacedResourceGetters,
		clusterResourceGetters:    clusterResourceGetters,
	}
}

// TryResource will retrieve a getter with resource name, it doesn't guarantee find resource with correct group version
// need to refactor this use schema.GroupVersionResource
func (r *ResourceGetter) TryResource(clusterScope bool, resource string) v1alpha3.Interface {
	if clusterScope {
		for k, v := range r.clusterResourceGetters {
			if k.Resource == resource {
				return v
			}
		}
	}
	for k, v := range r.namespacedResourceGetters {
		if k.Resource == resource {
			return v
		}
	}
	return nil
}

func (r *ResourceGetter) Get(resource, namespace, name string) (runtime.Object, error) {
	clusterScope := namespace == ""
	getter := r.TryResource(clusterScope, resource)
	if getter == nil {
		return nil, ErrResourceNotSupported
	}
	return getter.Get(namespace, name)
}

func (r *ResourceGetter) List(resource, namespace string, query *query.Query) (*api.ListResult, error) {
	clusterScope := namespace == ""
	getter := r.TryResource(clusterScope, resource)
	if getter == nil {
		return nil, ErrResourceNotSupported
	}
	return getter.List(namespace, query)
}
