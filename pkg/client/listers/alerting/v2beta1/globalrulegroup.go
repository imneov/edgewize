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

package v2beta1

import (
	v2beta1 "github.com/edgewize-io/edgewize/pkg/apis/alerting/v2beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// GlobalRuleGroupLister helps list GlobalRuleGroups.
// All objects returned here must be treated as read-only.
type GlobalRuleGroupLister interface {
	// List lists all GlobalRuleGroups in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v2beta1.GlobalRuleGroup, err error)
	// Get retrieves the GlobalRuleGroup from the index for a given name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v2beta1.GlobalRuleGroup, error)
	GlobalRuleGroupListerExpansion
}

// globalRuleGroupLister implements the GlobalRuleGroupLister interface.
type globalRuleGroupLister struct {
	indexer cache.Indexer
}

// NewGlobalRuleGroupLister returns a new GlobalRuleGroupLister.
func NewGlobalRuleGroupLister(indexer cache.Indexer) GlobalRuleGroupLister {
	return &globalRuleGroupLister{indexer: indexer}
}

// List lists all GlobalRuleGroups in the indexer.
func (s *globalRuleGroupLister) List(selector labels.Selector) (ret []*v2beta1.GlobalRuleGroup, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v2beta1.GlobalRuleGroup))
	})
	return ret, err
}

// Get retrieves the GlobalRuleGroup from the index for a given name.
func (s *globalRuleGroupLister) Get(name string) (*v2beta1.GlobalRuleGroup, error) {
	obj, exists, err := s.indexer.GetByKey(name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v2beta1.Resource("globalrulegroup"), name)
	}
	return obj.(*v2beta1.GlobalRuleGroup), nil
}
