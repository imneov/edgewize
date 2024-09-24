package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InferModelTemplateSpec struct {
	ServiceGroup map[string]int `json:"serviceGroup,omitempty"`
}

type InferModelTemplateStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
// InferModelTemplate is the schema for the InferModelTemplates API
type InferModelTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferModelTemplateSpec   `json:"spec,omitempty"`
	Status InferModelTemplateStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InferModelTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferModelTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferModelTemplate{}, &InferModelTemplateList{})
}
