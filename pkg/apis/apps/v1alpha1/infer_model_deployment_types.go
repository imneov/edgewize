package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	LabelIMTemplate        = "apps.edgewize.io/imtemplate"
	LabelIMTemplateVersion = "apps.edgewize.io/imtemplateversion"
	LabelIMDeployment      = "apps.edgewize.io/imdeployment"
)

type ModelNodeSelector struct {
	Project         string         `json:"project,omitempty"`
	NodeGroup       string         `json:"nodeGroup,omitempty"`
	NodeName        string         `json:"nodeName,omitempty"`
	HostPort        string         `json:"hostPort,omitempty"`
	ResourceRequest map[string]int `json:"resourceRequest,omitempty"`
}

type InnerDeploymentTemplate struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Specification of the desired behavior of the Deployment.
	// +optional
	Spec ModelDeploymentSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

type InferModelDeploymentSpec struct {
	IMTemplateName     string                  `json:"imTemplateName,omitempty"`
	Version            string                  `json:"version,omitempty"`
	DeploymentTemplate InnerDeploymentTemplate `json:"deploymentTemplate,omitempty"`
	NodeSelectors      []ModelNodeSelector     `json:"nodeSelectors,omitempty"`
}

type WorkLoadStatus struct {
	Name               string      `json:"name"`
	Deployed           bool        `json:"deployed"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	Message            string      `json:"message"`
}

type InferModelDeploymentStatus struct {
	WorkLoadInstances map[string]WorkLoadStatus `json:"workLoadInstances"`
}

//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +k8s:openapi-gen=true

// InferModelDeployment is the schema for the inferModelTemplates API
type InferModelDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InferModelDeploymentSpec   `json:"spec,omitempty"`
	Status InferModelDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type InferModelDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InferModelDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferModelDeployment{}, &InferModelDeploymentList{})
}
