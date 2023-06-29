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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Progressing = "progressing"
	Succeeded   = "succeeded"
	Failed      = "failed"
)
const (
	LabelEdgeAppSet         = "apps.edgewize.io/appset"
	LabelNodeGroup          = "apps.edgewize.io/nodegroup"
	LabelNode               = "apps.edgewize.io/node"
	LabelAppTemplate        = "apps.edgewize.io/apptemplate"
	LabelAppTemplateVersion = "apps.edgewize.io/apptemplateversion"
)

type NodeSelector struct {
	Project   string `json:"project,omitempty"`
	NodeGroup string `json:"nodeGroup,omitempty"`
	NodeName  string `json:"nodeName,omitempty"`
}

type EdgeAppSetSpec struct {
	AppTemplateName    string             `json:"appTemplateName,omitempty"`
	Version            string             `json:"version,omitempty"`
	DeploymentTemplate *appsv1.Deployment `json:"deploymentTemplate,omitempty"`
	NodeSelectors      []NodeSelector     `json:"nodeSelectors,omitempty"`
}

type DeploymentStatus struct {
	Status string `json:"status,omitempty"`
}
type EdgeAppSetStatus struct {
	WorkloadCount int                         `json:"workloadCount,omitempty"`
	Deployments   map[string]DeploymentStatus `json:"deployments,omitempty"`
}

//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +k8s:openapi-gen=true

// EdgeAppSet is the schema for the appTemplates API
type EdgeAppSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeAppSetSpec   `json:"spec,omitempty"`
	Status EdgeAppSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type EdgeAppSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeAppSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EdgeAppSet{}, &EdgeAppSetList{})
}
