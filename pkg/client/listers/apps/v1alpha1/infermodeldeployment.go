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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// InferModelDeploymentLister helps list InferModelDeployments.
// All objects returned here must be treated as read-only.
type InferModelDeploymentLister interface {
	// List lists all InferModelDeployments in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.InferModelDeployment, err error)
	// InferModelDeployments returns an object that can list and get InferModelDeployments.
	InferModelDeployments(namespace string) InferModelDeploymentNamespaceLister
	InferModelDeploymentListerExpansion
}

// inferModelDeploymentLister implements the InferModelDeploymentLister interface.
type inferModelDeploymentLister struct {
	indexer cache.Indexer
}

// NewInferModelDeploymentLister returns a new InferModelDeploymentLister.
func NewInferModelDeploymentLister(indexer cache.Indexer) InferModelDeploymentLister {
	return &inferModelDeploymentLister{indexer: indexer}
}

// List lists all InferModelDeployments in the indexer.
func (s *inferModelDeploymentLister) List(selector labels.Selector) (ret []*v1alpha1.InferModelDeployment, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.InferModelDeployment))
	})
	return ret, err
}

// InferModelDeployments returns an object that can list and get InferModelDeployments.
func (s *inferModelDeploymentLister) InferModelDeployments(namespace string) InferModelDeploymentNamespaceLister {
	return inferModelDeploymentNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// InferModelDeploymentNamespaceLister helps list and get InferModelDeployments.
// All objects returned here must be treated as read-only.
type InferModelDeploymentNamespaceLister interface {
	// List lists all InferModelDeployments in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.InferModelDeployment, err error)
	// Get retrieves the InferModelDeployment from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.InferModelDeployment, error)
	InferModelDeploymentNamespaceListerExpansion
}

// inferModelDeploymentNamespaceLister implements the InferModelDeploymentNamespaceLister
// interface.
type inferModelDeploymentNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all InferModelDeployments in the indexer for a given namespace.
func (s inferModelDeploymentNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.InferModelDeployment, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.InferModelDeployment))
	})
	return ret, err
}

// Get retrieves the InferModelDeployment from the indexer for a given namespace and name.
func (s inferModelDeploymentNamespaceLister) Get(name string) (*v1alpha1.InferModelDeployment, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("infermodeldeployment"), name)
	}
	return obj.(*v1alpha1.InferModelDeployment), nil
}
