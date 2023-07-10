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

package v1alpha1

import (
	"context"
	time "time"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	versioned "github.com/edgewize-io/edgewize/pkg/client/clientset/versioned"
	internalinterfaces "github.com/edgewize-io/edgewize/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/edgewize-io/edgewize/pkg/client/listers/apps/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// EdgeAppSetInformer provides access to a shared informer and lister for
// EdgeAppSets.
type EdgeAppSetInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.EdgeAppSetLister
}

type edgeAppSetInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewEdgeAppSetInformer constructs a new informer for EdgeAppSet type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewEdgeAppSetInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredEdgeAppSetInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredEdgeAppSetInformer constructs a new informer for EdgeAppSet type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredEdgeAppSetInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.AppsV1alpha1().EdgeAppSets(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.AppsV1alpha1().EdgeAppSets(namespace).Watch(context.TODO(), options)
			},
		},
		&appsv1alpha1.EdgeAppSet{},
		resyncPeriod,
		indexers,
	)
}

func (f *edgeAppSetInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredEdgeAppSetInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *edgeAppSetInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&appsv1alpha1.EdgeAppSet{}, f.defaultInformer)
}

func (f *edgeAppSetInformer) Lister() v1alpha1.EdgeAppSetLister {
	return v1alpha1.NewEdgeAppSetLister(f.Informer().GetIndexer())
}