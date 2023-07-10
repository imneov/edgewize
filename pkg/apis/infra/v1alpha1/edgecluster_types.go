/*
Copyright 2023.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Status string

const (
	RunningStatus      Status = "running"
	InstallingStatus   Status = "installing"
	UninstallingStatus Status = "uninstalling"
	ErrorStatus        Status = "error"
)

type InstallType string

const (
	InstallTypeAuto   InstallType = "auto"
	InstallTypeManual InstallType = "manual"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EdgeClusterSpec defines the desired state of EdgeCluster
type EdgeClusterSpec struct {
	Alias string `json:"alias,omitempty"`

	// Namespace is the target namespace where the edge cluster will be installed
	Namespace string `json:"namespace,omitempty"`

	// HostCluster is the cluster in which the edge cluster will be created and hosted (default to "host")
	// it depends on the multi-cluster component.
	HostCluster string `json:"hostCluster,omitempty"`

	// Distro is Kubernetes distro to use for the virtual cluster. Allowed distros: k3s, k0s, k8s, eks (default "k3s")
	Distro string `json:"distro,omitempty"`

	Description string `json:"description,omitempty"`

	// Version is the edge cluster distro version. TODO
	Version string `json:"version,omitempty"`

	// Location is the location of the current cluster. TODO
	Location string `json:"location,omitempty"`

	// Components will install in the edgecluster ,default is "edgewize,whizard-edge-agent,cloudcore,fluent-operator"
	//   -xxx means does not install xxx
	//   edgewize will always install
	Components string `json:"components,omitempty"`

	AdvertiseAddress []string `json:"advertiseAddress,omitempty"`

	// Type is the edge cluster installation method
	Type InstallType `json:"type,omitempty"`

	// KubeConfig is the edge cluster kubeconfig, encode by base64
	KubeConfig []byte `json:"kubeConfig,omitempty"`
}

// EdgeClusterStatus defines the observed state of EdgeCluster
type EdgeClusterStatus struct {

	// Status is the edge cluster release installation status
	Status Status `json:"status,omitempty"`

	ConfigFile string `json:"configFile,omitempty"`

	// KubeConfig is the edge cluster kubeconfig, encode by base64
	KubeConfig string `json:"kubeConfig,omitempty"`

	EdgeWize Status `json:"edgewize,omitempty"`

	KSCore Status `json:"ksCore,omitempty"`

	CloudCore Status `json:"cloudcore,omitempty"`

	FluentOperator Status `json:"fluentOperator,omitempty"`

	EdgewizeMonitor Status `json:"edgewizeMonitor,omitempty"`

	EdgeOtaServer Status `json:"edgeOtaServer,omitempty"`
}

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="NameSpace",type=string,JSONPath=`.spec.namespace`
// +kubebuilder:printcolumn:name="Distro",type=string,JSONPath=`.spec.distro`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="EdgeWize",type=string,priority=1,JSONPath=`.status.edgewize`
// +kubebuilder:printcolumn:name="CloudCore",type=string,priority=1,JSONPath=`.status.cloudcore`
// +kubebuilder:printcolumn:name="FluentOperator",type=string,priority=1,JSONPath=`.status.fluentOperator`
// +kubebuilder:printcolumn:name="EdgewizeMonitor",type=string,priority=1,JSONPath=`.status.edgewizeMonitor`
// EdgeCluster is the Schema for the edgeclusters API
type EdgeCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeClusterSpec   `json:"spec,omitempty"`
	Status EdgeClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EdgeClusterList contains a list of EdgeCluster
type EdgeClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EdgeCluster{}, &EdgeClusterList{})
}
