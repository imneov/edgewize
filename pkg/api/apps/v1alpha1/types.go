package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
)

type WorkloadStats struct {
	Processing int `json:"processing,omitempty"`
	Succeeded  int `json:"succeeded,omitempty"`
	Failed     int `json:"failed,omitempty"`
}

type EdgeAppSetStatus struct {
	WorkloadStats WorkloadStats        `json:"workloadStats,omitempty"`
	Workloads     []*appsv1.Deployment `json:"workloads,omitempty"`
}

type EdgeAppSet struct {
	appsv1alpha1.EdgeAppSet `json:",inline"`
	Status                  EdgeAppSetStatus `json:"status,omitempty"`
}

type AppTemplate struct {
	appsv1alpha1.AppTemplate `json:",inline"`
	Spec                     AppTemplateSpec `json:"spec,omitempty"`
}

type AppTemplateSpec struct {
	VersionList   []*appsv1alpha1.AppTemplateVersion `json:"versionList,omitempty"`
	LatestVersion string                             `json:"latestVersion,omitempty"`
}

type InferModelTemplate struct {
	appsv1alpha1.InferModelTemplate `json:",inline"`
	Spec                            InferModelTemplateSpec `json:"spec,omitempty"`
}

type InferModelTemplateSpec struct {
	ServiceGroup  map[string]int                            `json:"serviceGroup,omitempty"`
	VersionList   []*appsv1alpha1.InferModelTemplateVersion `json:"versionList,omitempty"`
	LatestVersion string                                    `json:"latestVersion,omitempty"`
}

type InferModelDeploymentStatus struct {
	WorkloadStats WorkloadStats        `json:"workloadStats,omitempty"`
	Workloads     []*appsv1.Deployment `json:"workloads,omitempty"`
}

type InferModelDeployment struct {
	appsv1alpha1.InferModelDeployment `json:",inline"`
	Status                            InferModelDeploymentStatus `json:"status,omitempty"`
}

type TemplateItem struct {
	ResourceName    string         `json:"resourceName,omitempty"`
	Description     string         `json:"description,omitempty"`
	ResourceRequest map[string]int `json:"resourceRequest,omitempty"`
}

type NodeSpecifications struct {
	// "GPU", "NPU" or "COMMON"
	Type string `json:"type,omitempty"`
	// NPU: "huawei-Ascend310", "huawei-Ascend310P"
	// GPU: "nvidia.com/gpu"
	DevModel          string         `json:"devModel,omitempty"`
	ResourceTemplates []TemplateItem `json:"resources"`
	Virtualized       bool           `json:"virtualized"`
}

type ResourceTemplateConfig struct {
	NPU DevResourceTemplate `yaml:"NPU"`
	GPU DevResourceTemplate `yaml:"GPU"`
}

type DevResourceTemplate struct {
	Virtualized map[string][]TemplateDetail `yaml:"virtualized"`
	Physical    map[string][]TemplateDetail `yaml:"physical"`
}

type TemplateDetail struct {
	ResourceName    string         `yaml:"resourceName"`
	ResourceRequest map[string]int `yaml:"resourceRequest"`
	Description     string         `yaml:"description"`
}

type RunningInferModelServers struct {
	Servers []InferModelServer `json:"servers"`
}

type InferModelServer struct {
	Name          string               `json:"name,omitempty"`
	Namespace     string               `json:"namespace,omitempty"`
	ServiceGroups []ServiceGroupStatus `json:"serviceGroups"`
}

type ServiceGroupStatus struct {
	Name   string `json:"name,omitempty"`
	IsUsed bool   `json:"isUsed"`
}
