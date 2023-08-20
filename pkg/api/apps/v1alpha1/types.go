package v1alpha1

import (
	appsv1 "k8s.io/api/apps/v1"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
)

type WorkloadStats struct {
	Processing int `json:"processing,omitempty"`
	Succeeded   int `json:"succeeded,omitempty"`
	Failed      int `json:"failed,omitempty"`
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
