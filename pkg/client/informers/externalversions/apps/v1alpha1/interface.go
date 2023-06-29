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
	internalinterfaces "github.com/edgewize-io/edgewize/pkg/client/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// AppTemplates returns a AppTemplateInformer.
	AppTemplates() AppTemplateInformer
	// AppTemplateVersions returns a AppTemplateVersionInformer.
	AppTemplateVersions() AppTemplateVersionInformer
	// EdgeAppSets returns a EdgeAppSetInformer.
	EdgeAppSets() EdgeAppSetInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// AppTemplates returns a AppTemplateInformer.
func (v *version) AppTemplates() AppTemplateInformer {
	return &appTemplateInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// AppTemplateVersions returns a AppTemplateVersionInformer.
func (v *version) AppTemplateVersions() AppTemplateVersionInformer {
	return &appTemplateVersionInformer{factory: v.factory, tweakListOptions: v.tweakListOptions}
}

// EdgeAppSets returns a EdgeAppSetInformer.
func (v *version) EdgeAppSets() EdgeAppSetInformer {
	return &edgeAppSetInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
