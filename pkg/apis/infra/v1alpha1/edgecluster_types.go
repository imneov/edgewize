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
	"helm.sh/helm/v3/pkg/chartutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SystemStatus string

const (
	InitialStatus             SystemStatus = "initial"
	VerifyingStatus           SystemStatus = "verifying"
	VerifySuccessStatus       SystemStatus = "verify_success"
	VerifyFailedStatus        SystemStatus = "verify_failed"
	ClusterInstallingStatus   SystemStatus = "cluster_installing"
	ClusterFailedStatus       SystemStatus = "cluster_failed"
	ClusterSuccessStatus      SystemStatus = "cluster_success"
	PreparingStatus           SystemStatus = "preparing"
	PrepareFailedStatus       SystemStatus = "prepare_failed"
	PrepareSuccessStatus      SystemStatus = "prepare_success"
	ComponentInstallingStatus SystemStatus = "component_installing"
	ComponentFailedStatus     SystemStatus = "component_failed"
	ComponentSuccessStatus    SystemStatus = "component_success"
	DeprecatedRunningStatus   SystemStatus = "running" //Deprecated
	//ServiceUpdatingStatus      SystemStatus = "service_updating"
	//ServiceUpdateFailedStatus  SystemStatus = "service_update_success"
	//ServiceUpdateSuccessStatus SystemStatus = "service_update_success"
)

type Status string

const (
	RunningStatus      Status = "running"
	PendingStatus      Status = "pending"
	InstallingStatus   Status = "installing"
	UninstallingStatus Status = "uninstalling"
	ErrorStatus        Status = "error"
)

type InstallType string

const (
	InstallTypeAuto   InstallType = "auto"
	InstallTypeManual InstallType = "manual"
)

type ValueString string

func (v ValueString) ToValues() map[string]interface{} {
	values, err := chartutil.ReadValues([]byte(v))
	if err != nil {
		return chartutil.Values{}
	}
	return values
}

type Component struct {
	Name            string      `json:"name,omitempty"`
	File            string      `json:"file,omitempty"`
	Namespace       string      `json:"namespace,omitempty"`
	SystemNamespace bool        `json:"systemNamespace,omitempty"`
	Values          ValueString `json:"values,omitempty"`
	Dependencies    []string    `json:"dependencies,omitempty"`
}

// EdgeClusterSpec defines the desired state of EdgeCluster
type EdgeClusterSpec struct {
	// Type is the edge cluster installation method, auto or manual
	Type InstallType `json:"type,omitempty"`

	// Namespace is the target namespace in the metacluster where the edge cluster will be installed , in auto mode
	Namespace string `json:"namespace,omitempty"`

	// Distro is Kubernetes distro to use for the virtual cluster. Allowed distros: k3s, k0s, k8s, eks (default "k3s"), in auto mode
	Distro string `json:"distro,omitempty"`

	// Version is the edge cluster distro version. TODO, in auto mode
	Version string `json:"version,omitempty"`

	// Components will install in the edgecluster , in auto mode
	Components []Component `json:"components,omitempty"`

	// Location is the location of the current cluster. TODO
	Location string `json:"location,omitempty"`

	// KubeConfig is the edge cluster kubeconfig, encode by base64 , in manual,auto mode
	KubeConfig []byte `json:"kubeConfig,omitempty"`

	// AdvertiseAddress is the advertise address of the Cloudcore in edge cluster, in manual mode
	AdvertiseAddress []string `json:"advertiseAddress,omitempty"`

	// HostCluster is the cluster in which the edge cluster will be created and hosted (default to "host")
	// it depends on the multi-cluster component.
	HostCluster string `json:"hostCluster,omitempty"`
}

// EdgeClusterStatus defines the observed state of EdgeCluster
type EdgeClusterStatus struct {

	// Status is the edge cluster release installation status
	Status SystemStatus `json:"status,omitempty"`

	ConfigFile string `json:"configFile,omitempty"`

	// KubeConfig is the edge cluster kubeconfig, encode by base64
	KubeConfig string `json:"kubeConfig,omitempty"`

	Components map[string]Component `json:"components,omitempty"`

	ComponentStatus map[string]Status `json:"componentStatus,omitempty"`

	EdgeWize Status `json:"edgewize,omitempty"`

	KSCore Status `json:"ksCore,omitempty"`

	Kubefed Status `json:"kubefed,omitempty"`

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
// +kubebuilder:printcolumn:name="KS-Core",type=string,priority=1,JSONPath=`.status.ksCore`
// +kubebuilder:printcolumn:name="Kubefed",type=string,priority=1,JSONPath=`.status.kubefed`

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
