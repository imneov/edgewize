/*
Copyright 2020 KubeSphere Authors

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

package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AppTemplateVersionSpec struct {

	// Version of the AppTemplateVersion.
	Version string `json:"version,omitempty"`

	DeploymentSpec *DeploymentSpec `json:"deploymentSpec,omitempty"`
}

type DeploymentSpec struct {
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
	// Template describes the pods that will be created.
	Template v1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`
	// The deployment strategy to use to replace existing pods with new ones.
	// +optional
	// +patchStrategy=retainKeys
	Strategy appsv1.DeploymentStrategy `json:"strategy,omitempty" patchStrategy:"retainKeys" protobuf:"bytes,4,opt,name=strategy"`
}

type AppTemplateVersionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster

// AppTemplateVersion is the schema for the appTemplates API
type AppTemplateVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppTemplateVersionSpec   `json:"spec,omitempty"`
	Status AppTemplateVersionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AppTemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppTemplateVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AppTemplateVersion{}, &AppTemplateVersionList{})
}
