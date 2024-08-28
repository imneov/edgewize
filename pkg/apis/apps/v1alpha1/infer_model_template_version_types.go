package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type InferModelTemplateVersionSpec struct {

	// Version of the InferModelTemplateVersion.
	Version string `json:"version,omitempty"`

	ServiceGroup map[string]int `json:"serviceGroup,omitempty"`

	ModelServerSpec *ModelServerSpec `json:"modelServerSpec,omitempty"`

	DeploymentSpec *ModelDeploymentSpec `json:"deploymentSpec,omitempty"`
}

type ModelServerSpec struct {
	Name            string            `json:"name,omitempty"`
	Version         string            `json:"version,omitempty"`
	Description     string            `json:"description,omitempty"`
	Servables       []ServableConfigs `json:"servables,omitempty"`
	Image           ImageConfig       `json:"image,omitempty"`
	HardwareSupport []Hardware        `json:"hardwareSupport,omitempty"`
}

type ModelDeploymentSpec struct {
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
	// Template describes the pods that will be created.
	Template v1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"`
	// The deployment strategy to use to replace existing pods with new ones.
	// +optional
	// +patchStrategy=retainKeys
	Strategy appsv1.DeploymentStrategy `json:"strategy,omitempty" patchStrategy:"retainKeys" protobuf:"bytes,4,opt,name=strategy"`
}

type ServableConfigs struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	ModelFile   string         `json:"modelFile,omitempty"`
	ModelFormat string         `json:"modelFormat,omitempty"`
	Methods     []MethodDetail `json:"methods,omitempty"`
}

type MethodDetail struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// Each Method is Markdown format content (Base64 encoded)
	Readme string `json:"readme,omitempty"`
}

type Hardware struct {
	Type   string   `json:"type,omitempty"`
	Models []string `json:"models,omitempty"`
}

type ImageConfig struct {
	Registry   string `json:"registry,omitempty"`
	Repository string `json:"repository,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

type InferModelTemplateVersionStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster

// InferModelTemplateVersion is the schema for the InferModelTemplates API
type InferModelTemplateVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferModelTemplateVersionSpec   `json:"spec,omitempty"`
	Status InferModelTemplateVersionStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InferModelTemplateVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferModelTemplateVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferModelTemplateVersion{}, &InferModelTemplateVersionList{})
}
