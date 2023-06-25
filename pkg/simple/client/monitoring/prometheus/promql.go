/*
Copyright 2019 The KubeSphere Authors.
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

package prometheus

import (
	"fmt"
	"strings"

	"github.com/edgewize-io/edgewize/pkg/constants"
	"github.com/edgewize-io/edgewize/pkg/simple/client/monitoring"
)

const (
	StatefulSet = "StatefulSet"
	DaemonSet   = "DaemonSet"
	Deployment  = "Deployment"
)

var promQLTemplates = map[string]string{
	// cluster
	"cluster_cpu_utilisation":                            ":node_cpu_utilisation:avg1m",
	"cluster_cpu_usage":                                  `round(sum(node:node_cpu_utilisation:avg1m{} * node:node_num_cpu:sum{}), 0.001)`,
	"cluster_cpu_total":                                  "sum(node:node_num_cpu:sum)",
	"cluster_cpu_non_master_total":                       `sum(node:node_num_cpu:sum{role!="master"})`,
	"cluster_memory_utilisation":                         ":node_memory_utilisation:",
	"cluster_memory_available":                           "sum(node:node_memory_bytes_available:sum)",
	"cluster_memory_total":                               "sum(node:node_memory_bytes_total:sum)",
	"cluster_memory_non_master_total":                    `sum(node:node_memory_bytes_total:sum{role!="master"})`,
	"cluster_memory_usage_wo_cache":                      "sum(node:node_memory_bytes_total:sum) - sum(node:node_memory_bytes_available:sum)",
	"cluster_net_utilisation":                            ":node_net_utilisation:sum_irate",
	"cluster_net_bytes_transmitted":                      "sum(node:node_net_bytes_transmitted:sum_irate)",
	"cluster_net_bytes_received":                         "sum(node:node_net_bytes_received:sum_irate)",
	"cluster_disk_read_iops":                             "sum(node:data_volume_iops_reads:sum)",
	"cluster_disk_write_iops":                            "sum(node:data_volume_iops_writes:sum)",
	"cluster_disk_read_throughput":                       "sum(node:data_volume_throughput_bytes_read:sum)",
	"cluster_disk_write_throughput":                      "sum(node:data_volume_throughput_bytes_written:sum)",
	"cluster_disk_size_usage":                            `sum(max(node_filesystem_size_bytes{device=~"/dev/.*", device!~"/dev/loop\\d+", job="node-exporter"} - node_filesystem_avail_bytes{device=~"/dev/.*", device!~"/dev/loop\\d+", job="node-exporter"}) by (device, instance))`,
	"cluster_disk_size_utilisation":                      `cluster:disk_utilization:ratio`,
	"cluster_disk_size_capacity":                         `sum(max(node_filesystem_size_bytes{device=~"/dev/.*", device!~"/dev/loop\\d+", job="node-exporter"}) by (device, instance))`,
	"cluster_disk_size_available":                        `sum(max(node_filesystem_avail_bytes{device=~"/dev/.*", device!~"/dev/loop\\d+", job="node-exporter"}) by (device, instance))`,
	"cluster_disk_inode_total":                           `sum(node:node_inodes_total:)`,
	"cluster_disk_inode_usage":                           `sum(node:node_inodes_total:) - sum(node:node_inodes_free:)`,
	"cluster_disk_inode_utilisation":                     `cluster:disk_inode_utilization:ratio`,
	"cluster_namespace_count":                            `count(kube_namespace_labels)`,
	"cluster_pod_count":                                  `count(kube_pod_info{job="kube-state-metrics"} and on(pod, namespace) (kube_pod_status_phase{job="kube-state-metrics",phase="Running"} ))`,
	"cluster_pod_quota":                                  `sum(max(kube_node_status_capacity{resource="pods"}) by (node) unless on (node) (kube_node_status_condition{condition="Ready",status=~"unknown|false"} > 0))`,
	"cluster_pod_utilisation":                            `cluster:pod_utilization:ratio`,
	"cluster_pod_running_count":                          `cluster:pod_running:count`,
	"cluster_pod_succeeded_count":                        `count(kube_pod_info{job="kube-state-metrics"} and on(pod, namespace) (kube_pod_status_phase{job="kube-state-metrics",phase="Succeeded"} > 0))`,
	"cluster_pod_pending_count":                          `count(kube_pod_info{job="kube-state-metrics"} and on(pod, namespace) (kube_pod_status_phase{job="kube-state-metrics",phase="Pending"} > 0))`,
	"cluster_pod_failed_count":                           `count(kube_pod_info{job="kube-state-metrics"} and on(pod, namespace) (kube_pod_status_phase{job="kube-state-metrics",phase="Failed"} > 0))`,
	"cluster_pod_unknown_count":                          `count(kube_pod_info{job="kube-state-metrics"} and on(pod, namespace) (kube_pod_status_phase{job="kube-state-metrics",phase="Unknown"} > 0))`,
	"cluster_pod_abnormal_count":                         `cluster:pod_abnormal:sum`,
	"cluster_pod_oomkilled_count":                        `sum(kube_pod_container_status_last_terminated_reason{reason="OOMKilled"})`,
	"cluster_pod_evicted_count":                          `sum(kube_pod_status_reason{phase="Evicted"}>0)`,
	"cluster_pod_qos_guaranteed_count":                   `sum(kube_pod_status_phase{phase="Running"} * on(namespace,pod) qos_owner_node:kube_pod_info:{qos="guaranteed"})`,
	"cluster_pod_qos_burstable_count":                    `sum(kube_pod_status_phase{phase="Running"} * on(namespace,pod) qos_owner_node:kube_pod_info:{qos="burstable"})`,
	"cluster_pod_qos_besteffort_count":                   `sum(kube_pod_status_phase{phase="Running"} * on(namespace,pod) qos_owner_node:kube_pod_info:{qos="besteffort"})`,
	"cluster_pod_cpu_usage":                              `round(sum (irate(container_cpu_usage_seconds_total{job="kubelet", container!="POD", container!="", image!=""}[5m]) and on(namespace,pod)node_namespace_pod:kube_pod_info:{}), 0.001)`,
	"cluster_pod_cpu_non_master_usage":                   `round(sum (irate(container_cpu_usage_seconds_total{job="kubelet", container!="POD", container!="", image!=""}[5m]) and on(namespace,pod)node_namespace_pod:kube_pod_info:{role!="master"}), 0.001)`,
	"cluster_pod_cpu_requests_total":                     `sum(sum by(node) (kube_pod_container_resource_requests{job="kube-state-metrics",resource="cpu"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role) (node_namespace_pod:kube_pod_info:{host_ip!="",node!=""}))`,
	"cluster_pod_cpu_requests_non_master_total":          `sum(sum by(node) (kube_pod_container_resource_requests{job="kube-state-metrics",resource="cpu"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role) (node_namespace_pod:kube_pod_info:{host_ip!="",node!="",role!="master"}))`,
	"cluster_pod_cpu_limits_total":                       `sum(sum by(node) (kube_pod_container_resource_limits{job="kube-state-metrics",resource="cpu"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role) (node_namespace_pod:kube_pod_info:{host_ip!="",node!=""}))`,
	"cluster_pod_cpu_limits_non_master_total":            `sum(sum by(node) (kube_pod_container_resource_limits{job="kube-state-metrics",resource="cpu"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role) (node_namespace_pod:kube_pod_info:{host_ip!="",node!="",role!="master"}))`,
	"cluster_pod_memory_usage_wo_cache":                  `sum(sum by (node) (container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}and on(namespace,pod)node_namespace_pod:kube_pod_info:{}))`,
	"cluster_pod_memory_non_master_usage_wo_cache":       `sum(sum by (node) (container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}and on(namespace,pod)node_namespace_pod:kube_pod_info:{role!="master"}))`,
	"cluster_pod_memory_requests_total":                  `sum(sum by(node) (kube_pod_container_resource_requests{job="kube-state-metrics",resource="memory"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role) (node_namespace_pod:kube_pod_info:{host_ip!="",node!=""}))`,
	"cluster_pod_memory_requests_non_master_total":       `sum(sum by(node) (kube_pod_container_resource_requests{job="kube-state-metrics",resource="memory"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role) (node_namespace_pod:kube_pod_info:{host_ip!="",node!="",role!="master"}))`,
	"cluster_pod_memory_limits_total":                    `sum(sum by(node) (kube_pod_container_resource_limits{job="kube-state-metrics",resource="memory"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role) (node_namespace_pod:kube_pod_info:{host_ip!="",node!=""}))`,
	"cluster_pod_memory_limits_non_master_total":         `sum(sum by(node) (kube_pod_container_resource_limits{job="kube-state-metrics",resource="memory"}) * on(node) group_left(host_ip, role) max by(node, host_ip, role)(node_namespace_pod:kube_pod_info:{host_ip!="",node!="",role!="master"}))`,
	"cluster_namespace_quota_cpu_usage":                  `sum(namespace:container_cpu_usage_seconds_total:sum_rate and on(namespace) kube_resourcequota)`,
	"cluster_namespace_quota_cpu_requests_hard_total":    `sum(kube_resourcequota{type="hard",resource="requests.cpu"})`,
	"cluster_namespace_quota_cpu_limits_hard_total":      `sum(kube_resourcequota{type="hard",resource="limits.cpu"})`,
	"cluster_namespace_quota_memory_usage":               `sum(namespace:container_memory_usage_bytes:sum and on(namespace) kube_resourcequota)`,
	"cluster_namespace_quota_memory_usage_wo_cache":      `sum(namespace:container_memory_usage_bytes_wo_cache:sum and on(namespace) kube_resourcequota)`,
	"cluster_namespace_quota_memory_requests_hard_total": `sum(kube_resourcequota{type="hard",resource="requests.memory"})`,
	"cluster_namespace_quota_memory_limits_hard_total":   `sum(kube_resourcequota{type="hard",resource="limits.memory"})`,

	"cluster_node_online":                `sum(kube_node_status_condition{condition="Ready",status="true"})`,
	"cluster_node_offline":               `cluster:node_offline:sum`,
	"cluster_node_total":                 `sum(kube_node_status_condition{condition="Ready"})`,
	"cluster_cronjob_count":              `sum(kube_cronjob_labels)`,
	"cluster_pvc_count":                  `sum(kube_persistentvolumeclaim_info)`,
	"cluster_daemonset_count":            `sum(kube_daemonset_labels)`,
	"cluster_deployment_count":           `sum(kube_deployment_labels)`,
	"cluster_endpoint_count":             `sum(kube_endpoint_labels)`,
	"cluster_hpa_count":                  `sum(kube_horizontalpodautoscaler_labels)`,
	"cluster_job_count":                  `sum(kube_job_labels)`,
	"cluster_statefulset_count":          `sum(kube_statefulset_labels)`,
	"cluster_replicaset_count":           `count(kube_replicaset_labels)`,
	"cluster_service_count":              `sum(kube_service_info)`,
	"cluster_secret_count":               `sum(kube_secret_info)`,
	"cluster_pv_count":                   `sum(kube_persistentvolume_labels)`,
	"cluster_pvc_bytes_total":            `sum(kubelet_volume_stats_capacity_bytes)`,
	"cluster_ingresses_extensions_count": `sum(kube_ingress_labels)`,
	"cluster_load1":                      `sum(node_load1{job="node-exporter"}) / sum(node:node_num_cpu:sum)`,
	"cluster_load5":                      `sum(node_load5{job="node-exporter"}) / sum(node:node_num_cpu:sum)`,
	"cluster_load15":                     `sum(node_load15{job="node-exporter"}) / sum(node:node_num_cpu:sum)`,
	"cluster_pod_abnormal_ratio":         `cluster:pod_abnormal:ratio`,
	"cluster_node_offline_ratio":         `cluster:node_offline:ratio`,
	"cluster_gpu_utilization":            `round(avg(DCGM_FI_PROF_GR_ENGINE_ACTIVE) / 100, 0.00001) or round(avg(DCGM_FI_DEV_GPU_UTIL) / 100, 0.00001)`,
	"cluster_gpu_usage":                  `round(sum(DCGM_FI_PROF_GR_ENGINE_ACTIVE) / 100, 0.00001) or round(sum(DCGM_FI_DEV_GPU_UTIL) / 100, 0.00001)`,
	"cluster_gpu_total":                  `sum(kube_node_status_capacity{resource="nvidia_com_gpu"})`,
	"cluster_gpu_memory_utilization":     `sum(DCGM_FI_DEV_FB_USED) / sum(DCGM_FI_DEV_FB_FREE + DCGM_FI_DEV_FB_USED)`,
	"cluster_gpu_memory_usage":           `sum(DCGM_FI_DEV_FB_USED) * 1024 * 1024`,
	"cluster_gpu_memory_available":       `sum(DCGM_FI_DEV_FB_FREE) * 1024 * 1024`,
	"cluster_gpu_memory_total":           `sum(DCGM_FI_DEV_FB_FREE + DCGM_FI_DEV_FB_USED) * 1024 * 1024`,

	"cluster_npu_utilization":        `round(avg(npu_chip_info_utilization) / 100, 0.00001)`,
	"cluster_npu_usage":              `round(sum(npu_chip_info_utilization) / 100, 0.00001)`,
	"cluster_npu_total":              `count(npu_chip_info_utilization)`,
	"cluster_npu_memory_usage":       `sum(npu_chip_info_used_memory) * 1024 * 1024`,
	"cluster_npu_memory_total":       `sum(npu_chip_info_total_memory) * 1024 * 1024`,
	"cluster_npu_memory_utilization": `round(sum(npu_chip_info_used_memory) / sum(npu_chip_info_total_memory), 0.00001)`,
	"cluster_npu_memory_available":   `(sum(npu_chip_info_total_memory- npu_chip_info_used_memory)) * 1024 * 1024`,

	// node
	"node_cpu_utilisation":        "node:node_cpu_utilisation:avg1m{$1}",
	"node_cpu_total":              "node:node_num_cpu:sum{$1}",
	"node_memory_utilisation":     "node:node_memory_utilisation:{$1}",
	"node_memory_available":       "node:node_memory_bytes_available:sum{$1}",
	"node_memory_total":           "node:node_memory_bytes_total:sum{$1}",
	"node_memory_usage_wo_cache":  "node:node_memory_bytes_total:sum{$1} - node:node_memory_bytes_available:sum{$1}",
	"node_net_utilisation":        "node:node_net_utilisation:sum_irate{$1}",
	"node_net_bytes_transmitted":  "node:node_net_bytes_transmitted:sum_irate{$1}",
	"node_net_bytes_received":     "node:node_net_bytes_received:sum_irate{$1}",
	"node_disk_read_iops":         "node:data_volume_iops_reads:sum{$1}",
	"node_disk_write_iops":        "node:data_volume_iops_writes:sum{$1}",
	"node_disk_read_throughput":   "node:data_volume_throughput_bytes_read:sum{$1}",
	"node_disk_write_throughput":  "node:data_volume_throughput_bytes_written:sum{$1}",
	"node_disk_size_capacity":     `sum(max(node_filesystem_size_bytes{device=~"/dev/.*", device!~"/dev/loop\\d+", job="node-exporter"} * on (namespace, pod) group_left(node) node_namespace_pod:kube_pod_info:{$1}) by (device, node)) by (node)`,
	"node_disk_size_available":    `node:disk_space_available:{$1}`,
	"node_disk_size_usage":        `sum(max((node_filesystem_size_bytes{device=~"/dev/.*", device!~"/dev/loop\\d+", job="node-exporter"} - node_filesystem_avail_bytes{device=~"/dev/.*", device!~"/dev/loop\\d+", job="node-exporter"}) * on (namespace, pod) group_left(node) node_namespace_pod:kube_pod_info:{$1}) by (device, node)) by (node)`,
	"node_disk_size_utilisation":  `node:disk_space_utilization:ratio{$1}`,
	"node_disk_inode_total":       `node:node_inodes_total:{$1}`,
	"node_disk_inode_usage":       `node:node_inodes_total:{$1} - node:node_inodes_free:{$1}`,
	"node_disk_inode_utilisation": `node:disk_inode_utilization:ratio{$1}`,
	"node_pod_count":              `node:pod_count:sum{$1}`,
	"node_pod_quota":              `max(kube_node_status_capacity{resource="pods",$1}) by (node) unless on (node) (kube_node_status_condition{condition="Ready",status=~"unknown|false"} > 0)`,
	"node_pod_utilisation":        `node:pod_utilization:ratio{$1}`,
	"node_pod_running_count":      `node:pod_running:count{$1}`,
	"node_pod_succeeded_count":    `node:pod_succeeded:count{$1}`,
	"node_pod_abnormal_count":     `node:pod_abnormal:count{$1}`,
	"node_cpu_usage":              `round(node:node_cpu_utilisation:avg1m{$1} * node:node_num_cpu:sum{$1}, 0.001)`,
	"node_load1":                  `node:load1:ratio{$1}`,
	"node_load5":                  `node:load5:ratio{$1}`,
	"node_load15":                 `node:load15:ratio{$1}`,
	"node_pod_abnormal_ratio":     `node:pod_abnormal:ratio{$1}`,
	"node_pleg_quantile":          `node_quantile:kubelet_pleg_relist_duration_seconds:histogram_quantile{$1}`,

	// TODO make Kubeedge to support /var/lib/kubelet/pod-resources
	//"node_gpu_utilization":        `round(avg(DCGM_FI_PROF_GR_ENGINE_ACTIVE * on (namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1}) by(node) / 100, 0.00001) or round(avg(DCGM_FI_DEV_GPU_UTIL* on (namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1}) by(node)/ 100, 0.00001)`,
	//"node_gpu_usage":              `round(sum(DCGM_FI_PROF_GR_ENGINE_ACTIVE * on (namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1}) by(node) / 100, 0.00001) or round(sum(DCGM_FI_DEV_GPU_UTIL* on (namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1}) by(node)/ 100, 0.00001)`,
	//"node_gpu_total":              `kube_node_status_capacity{resource="nvidia_com_gpu", $1}`,
	//"node_gpu_memory_utilization": `avg(DCGM_FI_DEV_FB_USED/(DCGM_FI_DEV_FB_FREE + DCGM_FI_DEV_FB_USED) * on(namespace , pod) group_left(node) (node_namespace_pod:kube_pod_info:{$1})) by(node)`,
	//"node_gpu_memory_usage":       `DCGM_FI_DEV_FB_USED * on(namespace , pod) group_left(node) (node_namespace_pod:kube_pod_info:{$1}) * 1024 * 1024`,
	//"node_gpu_memory_available":   `DCGM_FI_DEV_FB_FREE* on(namespace , pod) group_left(node) (node_namespace_pod:kube_pod_info:{$1}) * 1024 * 1024`,
	//"node_gpu_memory_total":       `sum((DCGM_FI_DEV_FB_FREE + DCGM_FI_DEV_FB_USED) * on(pod,namespace) group_left(node) node_namespace_pod:kube_pod_info:{$1}) by(node) * 1024 * 1024`,
	//"node_gpu_temp":               `round(DCGM_FI_DEV_GPU_TEMP* on(namespace , pod) group_left(node) (node_namespace_pod:kube_pod_info:{$1}), 0.001)`,
	//"node_gpu_power_usage":        `round(DCGM_FI_DEV_POWER_USAGE* on(namespace , pod) group_left(node) (node_namespace_pod:kube_pod_info:{$1}), 0.001)`,

	"node_gpu_utilization":        `round(avg(DCGM_FI_PROF_GR_ENGINE_ACTIVE{$1}) by (node) / 100, 0.00001) or round(avg(DCGM_FI_DEV_GPU_UTIL{$1}) by (node) / 100, 0.00001)`,
	"node_gpu_usage":              `round(avg(DCGM_FI_PROF_GR_ENGINE_ACTIVE{$1}) by (node) / 100, 0.00001) or round(avg(DCGM_FI_DEV_GPU_UTIL{$1}) by (node) / 100, 0.00001)`,
	"node_gpu_total":              `kube_node_status_capacity{resource="nvidia_com_gpu", $1}`,
	"node_gpu_memory_utilization": `round(sum(DCGM_FI_DEV_FB_USED{$1}) by (node) / (sum(DCGM_FI_DEV_FB_USED{$1}) by (node) + sum(DCGM_FI_DEV_FB_FREE{$1}) by (node)), 0.00001)`,
	"node_gpu_memory_usage":       `sum(DCGM_FI_DEV_FB_USED{$1}) by (node) * 1024 * 1024`,
	"node_gpu_memory_available":   `sum(DCGM_FI_DEV_FB_FREE{$1}) by (node) * 1024 * 1024`,
	"node_gpu_memory_total":       `sum(DCGM_FI_DEV_FB_FREE{$1} + DCGM_FI_DEV_FB_USED{$1}) by (node) * 1024 * 1024`,
	"node_gpu_temp":               `round(avg(DCGM_FI_DEV_GPU_TEMP{$1}) by (node), 0.001)`,
	"node_gpu_power_usage":        `round(avg(DCGM_FI_DEV_POWER_USAGE{$1}) by (node), 0.001)`,

	"node_npu_utilization":        `round(avg(npu_chip_info_utilization{$1}) by (node) / 100, 0.00001)`,
	"node_npu_usage":              `round(sum(npu_chip_info_utilization{$1}) by (node) / 100, 0.00001)`,
	"node_npu_total":              `count(npu_chip_info_utilization{$1}) by (node)`,
	"node_npu_memory_usage":       `sum(npu_chip_info_used_memory{$1}) by (node) * 1024 * 1024`,
	"node_npu_memory_total":       `sum(npu_chip_info_total_memory{$1}) by (node) * 1024 * 1024`,
	"node_npu_memory_utilization": `round(sum(npu_chip_info_used_memory{$1}) by (node) / sum(npu_chip_info_total_memory{$1}) by (node), 0.00001)`,
	"node_npu_memory_available":   `sum(npu_chip_info_total_memory{$1} - npu_chip_info_used_memory{$1}) by (node) * 1024 * 1024`,
	"node_npu_power":              `round(avg(npu_chip_info_power{$1}) by (node), 0.00001)`,
	"node_npu_temp":               `round(avg(npu_chip_info_temperature{$1}) by (node), 0.001)`,
	"node_npu_voltage":            `round(avg(npu_chip_info_voltage{$1}) by (node), 0.001)`,
	"node_npu_health_status":      `npu_chip_info_health_status{$1}`,

	"node_device_size_usage":       `sum by(device, node, host_ip, role) (node_filesystem_size_bytes{device!~"/dev/loop\\d+",device=~"/dev/.*",job="node-exporter"} * on(namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1}) - sum by(device, node, host_ip, role) (node_filesystem_avail_bytes{device!~"/dev/loop\\d+",device=~"/dev/.*",job="node-exporter"} * on(namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1})`,
	"node_device_size_utilisation": `1 - sum by(device, node, host_ip, role) (node_filesystem_avail_bytes{device!~"/dev/loop\\d+",device=~"/dev/.*",job="node-exporter"} * on(namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1}) / sum by(device, node, host_ip, role) (node_filesystem_size_bytes{device!~"/dev/loop\\d+",device=~"/dev/.*",job="node-exporter"} * on(namespace, pod) group_left(node, host_ip, role) node_namespace_pod:kube_pod_info:{$1})`,

	//edge node
	//The edge metrics with the prefix of node_filesystem is missing .

	/*"node_disk_size_capacity_edge": ``,
	"node_disk_size_available":    ``,
	"node_disk_size_usage":        ``,
	"node_disk_size_utilisation":  ``,
	"node_disk_inode_total":       ``,
	"node_disk_inode_usage":       ``,
	"node_disk_inode_utilisation": ``,
	"node_device_size_usage":       ``,
	"node_device_size_utilisation": ``,
	*/

	// workspace
	"workspace_cpu_usage":                  `round(sum by (workspace) (namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}), 0.001)`,
	"workspace_memory_usage":               `sum by (workspace) (namespace:container_memory_usage_bytes:sum{namespace!="", $1})`,
	"workspace_memory_usage_wo_cache":      `sum by (workspace) (namespace:container_memory_usage_bytes_wo_cache:sum{namespace!="", $1})`,
	"workspace_net_bytes_transmitted":      `sum by (workspace) (sum by (namespace) (irate(container_network_transmit_bytes_total{namespace!="", pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job=~"kubelet|kubeedge"}[5m])) * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(workspace) max by(workspace) (kube_namespace_labels{$1} * 0)`,
	"workspace_net_bytes_received":         `sum by (workspace) (sum by (namespace) (irate(container_network_receive_bytes_total{namespace!="", pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job=~"kubelet|kubeedge"}[5m])) * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(workspace) max by(workspace) (kube_namespace_labels{$1} * 0)`,
	"workspace_pod_count":                  `sum by (workspace) (kube_pod_status_phase{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1})) or on(workspace) max by(workspace) (kube_namespace_labels{$1} * 0)`,
	"workspace_pod_running_count":          `sum by (workspace) (kube_pod_status_phase{phase="Running", namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1})) or on(workspace) max by(workspace) (kube_namespace_labels{$1} * 0)`,
	"workspace_pod_succeeded_count":        `sum by (workspace) (kube_pod_status_phase{phase="Succeeded", namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1})) or on(workspace) max by(workspace) (kube_namespace_labels{$1} * 0)`,
	"workspace_pod_abnormal_count":         `count by (workspace) ((kube_pod_info{node!=""} unless on (pod, namespace) (kube_pod_status_phase{job="kube-state-metrics", phase="Succeeded"}>0) unless on (pod, namespace) ((kube_pod_status_ready{job="kube-state-metrics", condition="true"}>0) and on (pod, namespace) (kube_pod_status_phase{job="kube-state-metrics", phase="Running"}>0)) unless on (pod, namespace) (kube_pod_container_status_waiting_reason{job="kube-state-metrics", reason="ContainerCreating"}>0)) * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_ingresses_extensions_count": `sum by (workspace) (kube_ingress_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_cronjob_count":              `sum by (workspace) (kube_cronjob_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_pvc_count":                  `sum by (workspace) (kube_persistentvolumeclaim_info{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_pvc_bytes_used":             `sum by (workspace) (kubelet_volume_stats_used_bytes{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_daemonset_count":            `sum by (workspace) (kube_daemonset_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_deployment_count":           `sum by (workspace) (kube_deployment_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_endpoint_count":             `sum by (workspace) (kube_endpoint_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_hpa_count":                  `sum by (workspace) (kube_horizontalpodautoscaler_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_job_count":                  `sum by (workspace) (kube_job_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_statefulset_count":          `sum by (workspace) (kube_statefulset_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_replicaset_count":           `count by (workspace) (kube_replicaset_labels{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_service_count":              `sum by (workspace) (kube_service_info{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_secret_count":               `sum by (workspace) (kube_secret_info{namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_pod_abnormal_ratio":         `count by (workspace) ((kube_pod_info{node!=""} unless on (pod, namespace) (kube_pod_status_phase{job="kube-state-metrics", phase="Succeeded"}>0) unless on (pod, namespace) ((kube_pod_status_ready{job="kube-state-metrics", condition="true"}>0) and on (pod, namespace) (kube_pod_status_phase{job="kube-state-metrics", phase="Running"}>0)) unless on (pod, namespace) (kube_pod_container_status_waiting_reason{job="kube-state-metrics", reason="ContainerCreating"}>0)) * on (namespace) group_left(workspace) kube_namespace_labels{$1}) / sum by (workspace) (kube_pod_status_phase{phase!="Succeeded", namespace!=""} * on (namespace) group_left(workspace)(kube_namespace_labels{$1}))`,
	"workspace_gpu_usage":                  `round((sum by(workspace) (label_replace(DCGM_FI_PROF_GR_ENGINE_ACTIVE{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)") * on(namespace) group_left(workspace) (kube_namespace_labels{$1})) or  sum by(workspace) (label_replace(DCGM_FI_DEV_GPU_UTIL{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)") * on(namespace) group_left(workspace) (kube_namespace_labels{$1})) ) / 100,0.001)`,
	"workspace_gpu_memory_usage":           `(sum by (workspace) (label_replace(DCGM_FI_DEV_FB_USED{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)") * on(namespace) group_left(workspace) (kube_namespace_labels{$1}))) * 1024 * 1024`,

	"workspace_npu_usage":        `round((sum by(workspace) (container_npu_utilization * on(namespace) group_left(workspace) (kube_namespace_labels{$1})) ) / 100,0.001)`,
	"workspace_npu_memory_usage": `(sum by (workspace) (container_npu_used_memory * on(namespace) group_left(workspace) (kube_namespace_labels{$1}))) * 1024 * 1024`,

	// namespace
	"namespace_cpu_usage":                        `round(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="", $1}, 0.001)`,
	"namespace_cpu_requests_total":               `sum by(namespace)(kube_pod_container_resource_requests{resource="cpu", $1})`,
	"namespace_cpu_used_requests_utilisation":    `round(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="",$1} / on (namespace) sum by(namespace)(kube_pod_container_resource_requests{resource="cpu",$1}), 0.001)`,
	"namespace_cpu_used_limits_utilisation":      `round(namespace:container_cpu_usage_seconds_total:sum_rate{namespace!="",$1} / on (namespace) sum by(namespace)(kube_pod_container_resource_limits{resource="cpu",$1}), 0.001)`,
	"namespace_memory_usage":                     `namespace:container_memory_usage_bytes:sum{namespace!="", $1}`,
	"namespace_memory_usage_wo_cache":            `namespace:container_memory_usage_bytes_wo_cache:sum{namespace!="", $1}`,
	"namespace_memory_requests_total":            `sum by(namespace)(kube_pod_container_resource_requests{resource="memory",$1})`,
	"namespace_memory_used_requests_utilisation": `round(namespace:container_memory_usage_bytes_wo_cache:sum{namespace!="",$1} / on (namespace) sum by(namespace)(kube_pod_container_resource_requests{resource="memory",$1}),0.001)`,
	"namespace_memory_used_limits_utilisation":   `round(namespace:container_memory_usage_bytes_wo_cache:sum{namespace!="",$1} / on (namespace) sum by(namespace)(kube_pod_container_resource_limits{resource="memory", $1}),0.001)`,
	"namespace_net_bytes_transmitted":            `sum by (namespace) (irate(container_network_transmit_bytes_total{namespace!="", pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job=~"kubelet|kubeedge"}[5m]) * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_net_bytes_received":               `sum by (namespace) (irate(container_network_receive_bytes_total{namespace!="", pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job=~"kubelet|kubeedge"}[5m]) * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_count":                        `sum by (namespace) (kube_pod_status_phase{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_running_count":                `sum by (namespace) (kube_pod_status_phase{phase="Running", namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_succeeded_count":              `sum by (namespace) (kube_pod_status_phase{phase="Succeeded", namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_pending_count":                `sum by (namespace) (kube_pod_status_phase{phase="Pending", namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_failed_count":                 `sum by (namespace) (kube_pod_status_phase{phase="Failed", namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_unknown_count":                `sum by (namespace) (kube_pod_status_phase{phase="Unknown", namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_oomkilled_count":              `sum by (namespace) (kube_pod_container_status_last_terminated_reason{reason="OOMKilled"}  * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_evicted_count":                `sum by (namespace) ((kube_pod_status_reason{phase="Evicted"}>0) * on (namespace) group_left(workspace) kube_namespace_labels{$1}) or on(namespace) max by(namespace) (kube_namespace_labels{$1} * 0)`,
	"namespace_pod_qos_guaranteed_count":         `sum by (namespace) (kube_pod_status_phase{phase="Running"} * on(namespace,pod) qos_owner_node:kube_pod_info:{qos="guaranteed",$1})`,
	"namespace_pod_qos_burstable_count":          `sum by (namespace) (kube_pod_status_phase{phase="Running"} * on(namespace,pod) qos_owner_node:kube_pod_info:{qos="burstable",$1})`,
	"namespace_pod_qos_besteffort_count":         `sum by (namespace) (kube_pod_status_phase{phase="Running"} * on(namespace,pod) qos_owner_node:kube_pod_info:{qos="besteffort",$1})`,
	"namespace_pod_abnormal_count":               `namespace:pod_abnormal:count{namespace!="", $1}`,
	"namespace_pod_abnormal_ratio":               `namespace:pod_abnormal:ratio{namespace!="", $1}`,
	"namespace_memory_limit_hard":                `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="limits.memory"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_cpu_limit_hard":                   `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="limits.cpu"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_memory_requests_used":             `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="uesd", namespace!="", resource="requests.memory"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_memory_requests_hard":             `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="requests.memory"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_memory_limits_used":               `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="uesd", namespace!="", resource="limits.memory"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_memory_limits_hard":               `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="limits.memory"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_cpu_requests_uesd":                `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="uesd", namespace!="", resource="requests.cpu"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_cpu_requests_hard":                `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="requests.cpu"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_cpu_limits_uesd":                  `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="uesd", namespace!="", resource="limits.cpu"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_cpu_limits_hard":                  `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="limits.cpu"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_storage_requests_used":            `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="uesd", namespace!="", resource="requests.cpu"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_storage_requests_hard":            `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="requests.cpu"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_pvcs_used":                        `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="used", namespace!="", resource="persistentvolumeclaims"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_pvcs_hard":                        `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="persistentvolumeclaims"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_pod_count_hard":                   `min by (namespace) (kube_resourcequota{resourcequota!="quota", type="hard", namespace!="", resource="count/pods"} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_pvc_bytes_used":                   `sum by (namespace) (kubelet_volume_stats_used_bytes{$1})`,
	"namespace_pvc_bytes_total":                  `sum by (namespace) (kubelet_volume_stats_capacity_bytes{$1})`,
	"namespace_pvc_bytes_utilisation":            `avg by (namespace) (kubelet_volume_stats_used_bytes{$1} / kubelet_volume_stats_capacity_bytes{$1})`,
	"namespace_cronjob_count":                    `sum by (namespace) (kube_cronjob_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_pvc_count":                        `sum by (namespace) (kube_persistentvolumeclaim_info{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_daemonset_count":                  `sum by (namespace) (kube_daemonset_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_deployment_count":                 `sum by (namespace) (kube_deployment_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_endpoint_count":                   `sum by (namespace) (kube_endpoint_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_hpa_count":                        `sum by (namespace) (kube_horizontalpodautoscaler_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_job_count":                        `sum by (namespace) (kube_job_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_statefulset_count":                `sum by (namespace) (kube_statefulset_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_replicaset_count":                 `count by (namespace) (kube_replicaset_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_service_count":                    `sum by (namespace) (kube_service_info{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_secret_count":                     `sum by (namespace) (kube_secret_info{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_configmap_count":                  `sum by (namespace) (kube_configmap_info{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_ingresses_extensions_count":       `sum by (namespace) (kube_ingress_labels{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_s2ibuilder_count":                 `sum by (namespace) (s2i_s2ibuilder_created{namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_gpu_limit_hard":                   `sum by (namespace) (kube_resourcequota{resource="requests.nvidia.com/gpu", type="hard", namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_gpu_usage":                        `round((sum by(namespace) (label_replace(DCGM_FI_PROF_GR_ENGINE_ACTIVE{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)") * on(namespace) group_left(workspace) (kube_namespace_labels{$1})) or  sum by(namespace) (label_replace(DCGM_FI_DEV_GPU_UTIL{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)") * on(namespace) group_left(workspace) (kube_namespace_labels{$1})) ) / 100,0.001)`,
	"namespace_gpu_memory_usage":                 `(sum by (namespace) (label_replace(DCGM_FI_DEV_FB_USED{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)") * on(namespace) group_left(workspace) (kube_namespace_labels{$1}))) * 1024 * 1024`,
	"namespace_alerts_total":                     `count(sum without(ruler_replica) (ALERTS{rule_level="namespace", severity!="", severity!="info", $1}))`,
	"namespace_alerts_firing_total":              `count(sum without(ruler_replica) (ALERTS{rule_level="namespace", severity!="", severity!="info", alertstate="firing", $1}))`,
	"namespace_alerts_pending_total":             `count(sum without(ruler_replica) (ALERTS{rule_level="namespace", severity!="", severity!="info", alertstate="pending",$1}))`,
	"namespace_alerts_critical_total":            `count(sum without(ruler_replica) (ALERTS{rule_level="namespace", severity="critical", $1}))`,
	"namespace_alerts_error_total":               `count(sum without(ruler_replica) (ALERTS{rule_level="namespace", severity="error", $1}))`,
	"namespace_alerts_warning_total":             `count(sum without(ruler_replica) (ALERTS{rule_level="namespace", severity="warning", $1}))`,
	"namespace_alerts_info_total":                `count(sum without(ruler_replica) (ALERTS{rule_level="namespace", severity="info", $1}))`,

	"namespace_npu_limit_hard":   `sum by (namespace) (kube_resourcequota{resource="requests.huawei.com/Ascend310", type="hard", namespace!=""} * on (namespace) group_left(workspace) kube_namespace_labels{$1})`,
	"namespace_npu_usage":        `round((sum by(namespace) (container_npu_utilization * on(namespace) group_left(workspace) (kube_namespace_labels{$1})) ) / 100, 0.001)`,
	"namespace_npu_memory_usage": `(sum by (namespace) (container_npu_used_memory * on(namespace) group_left(workspace) (kube_namespace_labels{$1}))) * 1024 * 1024`,
	// ingress
	"ingress_request_count":                 `round(sum(increase(nginx_ingress_controller_requests{$1,$2}[$3])))`,
	"ingress_request_4xx_count":             `round(sum(increase(nginx_ingress_controller_requests{$1,$2,status=~"[4].*"}[$3])))`,
	"ingress_request_5xx_count":             `round(sum(increase(nginx_ingress_controller_requests{$1,$2,status=~"[5].*"}[$3])))`,
	"ingress_active_connections":            `sum(avg_over_time(nginx_ingress_controller_nginx_process_connections{$2,state="active"}[$3]))`,
	"ingress_success_rate":                  `sum(rate(nginx_ingress_controller_requests{$1,$2,status!~"[4-5].*"}[$3])) / sum(rate(nginx_ingress_controller_requests{$1,$2}[$3]))`,
	"ingress_request_duration_average":      `sum_over_time(nginx_ingress_controller_request_duration_seconds_sum{$1,$2}[$3])/sum_over_time(nginx_ingress_controller_request_duration_seconds_count{$1,$2}[$3])`,
	"ingress_request_duration_50percentage": `histogram_quantile(0.50, sum by (le) (rate(nginx_ingress_controller_request_duration_seconds_bucket{$1,$2}[$3])))`,
	"ingress_request_duration_95percentage": `histogram_quantile(0.95, sum by (le) (rate(nginx_ingress_controller_request_duration_seconds_bucket{$1,$2}[$3])))`,
	"ingress_request_duration_99percentage": `histogram_quantile(0.99, sum by (le) (rate(nginx_ingress_controller_request_duration_seconds_bucket{$1,$2}[$3])))`,
	"ingress_request_volume":                `round(sum(irate(nginx_ingress_controller_requests{$1,$2}[$3])), 0.001)`,
	"ingress_request_volume_by_ingress":     `round(sum(irate(nginx_ingress_controller_requests{$1,$2}[$3])) by (ingress), 0.001)`,
	"ingress_request_network_sent":          `sum(irate(nginx_ingress_controller_response_size_sum{$1,$2}[$3]))`,
	"ingress_request_network_received":      `sum(irate(nginx_ingress_controller_request_size_sum{$1,$2}[$3]))`,
	"ingress_request_memory_bytes":          `avg(nginx_ingress_controller_nginx_process_resident_memory_bytes{$2})`,
	"ingress_request_cpu_usage":             `avg(rate(nginx_ingress_controller_nginx_process_cpu_seconds_total{$2}[5m]))`,

	// workload
	"workload_cpu_usage":                              `round(namespace:workload_cpu_usage:sum{$1}, 0.001)`,
	"workload_memory_usage":                           `namespace:workload_memory_usage:sum{$1}`,
	"workload_memory_usage_wo_cache":                  `namespace:workload_memory_usage_wo_cache:sum{$1}`,
	"workload_net_bytes_transmitted":                  `namespace:workload_net_bytes_transmitted:sum_irate{$1}`,
	"workload_net_bytes_received":                     `namespace:workload_net_bytes_received:sum_irate{$1}`,
	"workload_deployment_replica":                     `label_join(sum (label_join(label_replace(kube_deployment_spec_replicas{$2}, "owner_kind", "Deployment", "", ""), "workload", "", "deployment")) by (namespace, owner_kind, workload), "workload", ":", "owner_kind", "workload")`,
	"workload_deployment_replica_available":           `label_join(sum (label_join(label_replace(kube_deployment_status_replicas_available{$2}, "owner_kind", "Deployment", "", ""), "workload", "", "deployment")) by (namespace, owner_kind, workload), "workload", ":", "owner_kind", "workload")`,
	"workload_statefulset_replica":                    `label_join(sum (label_join(label_replace(kube_statefulset_replicas{$2}, "owner_kind", "StatefulSet", "", ""), "workload", "", "statefulset")) by (namespace, owner_kind, workload), "workload", ":", "owner_kind", "workload")`,
	"workload_statefulset_replica_available":          `label_join(sum (label_join(label_replace(kube_statefulset_status_replicas_current{$2}, "owner_kind", "StatefulSet", "", ""), "workload", "", "statefulset")) by (namespace, owner_kind, workload), "workload", ":", "owner_kind", "workload")`,
	"workload_daemonset_replica":                      `label_join(sum (label_join(label_replace(kube_daemonset_status_desired_number_scheduled{$2}, "owner_kind", "DaemonSet", "", ""), "workload", "", "daemonset")) by (namespace, owner_kind, workload), "workload", ":", "owner_kind", "workload")`,
	"workload_daemonset_replica_available":            `label_join(sum (label_join(label_replace(kube_daemonset_status_number_available{$2}, "owner_kind", "DaemonSet", "", ""), "workload", "", "daemonset")) by (namespace, owner_kind, workload), "workload", ":", "owner_kind", "workload")`,
	"workload_deployment_unavailable_replicas_ratio":  `namespace:deployment_unavailable_replicas:ratio{$1}`,
	"workload_daemonset_unavailable_replicas_ratio":   `namespace:daemonset_unavailable_replicas:ratio{$1}`,
	"workload_statefulset_unavailable_replicas_ratio": `namespace:statefulset_unavailable_replicas:ratio{$1}`,
	"workload_gpu_usage":                              `namespace:workload_gpu_usage:sum{$1}`,
	"workload_gpu_memory_usage":                       `namespace:workload_gpu_memory_usage:sum{$1}`,

	"workload_npu_usage":        `namespace:workload_npu_usage:sum{$1}`,
	"workload_npu_memory_usage": `namespace:workload_npu_memory_usage:sum{$1}`,

	// pod
	"pod_cpu_usage":                        `round(sum by (namespace, pod) (irate(container_cpu_usage_seconds_total{job=~"kubelet|kubeedge", pod!="", image!=""}[5m])), 0.001) *  on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_cpu_used_requests_utilisation":    `round(sum by (namespace, pod) (irate(container_cpu_usage_seconds_total{job="kubelet", pod!="", image!=""}[5m])) / sum by (namespace,pod)(kube_pod_container_resource_requests{resource="cpu"}),0.001) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_cpu_used_limits_utilisation":      `round(sum by (namespace, pod) (irate(container_cpu_usage_seconds_total{job="kubelet", pod!="", image!=""}[5m])) / sum by (namespace,pod)(kube_pod_container_resource_limits{resource="cpu"}),0.001) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_memory_usage":                     `sum by (namespace, pod) (container_memory_usage_bytes{job=~"kubelet|kubeedge", pod!="", image!=""}) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_memory_usage_wo_cache":            `sum by (namespace, pod) (container_memory_working_set_bytes{job=~"kubelet|kubeedge", pod!="", image!=""}) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_memory_used_requests_utilisation": `sum by (namespace, pod) (container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}) / sum by (namespace,pod)(kube_pod_container_resource_requests{resource="memory"}) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_memory_used_limits_utilisation":   `sum by (namespace, pod) (container_memory_working_set_bytes{job="kubelet", pod!="", image!=""}) / sum by (namespace,pod)(kube_pod_container_resource_limits{resource="memory"}) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_net_bytes_transmitted":            `sum by (namespace, pod) (irate(container_network_transmit_bytes_total{pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job=~"kubelet|kubeedge"}[5m])) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_net_bytes_received":               `sum by (namespace, pod) (irate(container_network_receive_bytes_total{pod!="", interface!~"^(cali.+|tunl.+|dummy.+|kube.+|flannel.+|cni.+|docker.+|veth.+|lo.*)", job=~"kubelet|kubeedge"}[5m])) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}`,
	"pod_gpu_usage":                        `round(((sum by(namespace, pod) (label_replace(label_replace(DCGM_FI_PROF_GR_ENGINE_ACTIVE{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)"),"pod","$1","exported_pod","(.+)") * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1})) * (sum by (namespace,pod) (kube_pod_info{}))) /100, 0.001) or round(((sum by(namespace, pod) (label_replace(label_replace(DCGM_FI_DEV_GPU_UTIL{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)"),"pod","$1","exported_pod","(.+)") * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1})) * (sum by (namespace,pod) (kube_pod_info{}))) /100, 0.001)`,
	"pod_gpu_memory_usage":                 `(sum by(namespace, pod) (label_replace(label_replace(DCGM_FI_DEV_FB_USED{exported_namespace!=""},"namespace","$1","exported_namespace","(.+)"),"pod","$1","exported_pod","(.+)")) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}) * 1024 * 1024 `,
	"pod_pvc_bytes_used":                   `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_used_bytes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{} * on (namespace, persistentvolumeclaim) group_left (pod) kube_pod_spec_volumes_persistentvolumeclaims_info{}`,
	"pod_pvc_bytes_utilisation":            `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{} * on (namespace, persistentvolumeclaim) group_left (pod) kube_pod_spec_volumes_persistentvolumeclaims_info{}`,

	"pod_npu_usage":        `round(((sum by(namespace, pod) (label_replace(label_replace(container_npu_utilization, "pod", "$1", "pod_name", "(.+)"), "container", "$1", "container_name", "(.+)") * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1})) * (sum by (namespace,pod) (kube_pod_info{}))) / 100, 0.001)`,
	"pod_npu_memory_usage": `(sum by(namespace, pod) (label_replace(label_replace(container_npu_used_memory, "pod", "$1", "pod_name", "(.+)"), "container", "$1", "container_name", "(.+)")) * on (namespace, pod) group_left (qos,owner_kind,owner_name,node) qos_owner_node:kube_pod_info:{$1}) * 1024 * 1024`,

	// container
	"container_cpu_usage":             `round(sum by (namespace, pod, container) (irate(container_cpu_usage_seconds_total{job=~"kubelet|kubeedge", container!="POD", container!="", image!="", $1}[5m])), 0.001)`,
	"container_memory_usage":          `sum by (namespace, pod, container) (container_memory_usage_bytes{job=~"kubelet|kubeedge", container!="POD", container!="", image!="", $1})`,
	"container_memory_usage_wo_cache": `sum by (namespace, pod, container) (container_memory_working_set_bytes{job=~"kubelet|kubeedge", container!="POD", container!="", image!="", $1})`,
	"container_gpu_usage":             `round((sum by(namespace, pod, container, device, gpu) (label_replace(label_replace(label_replace(DCGM_FI_PROF_GR_ENGINE_ACTIVE{$1},"namespace","$1","exported_namespace","(.+)"),"pod","$1","exported_pod","(.+)"),"container","$1","exported_container","(.+)")) or sum by(namespace, pod, container, device, gpu) (label_replace(label_replace(label_replace(DCGM_FI_DEV_GPU_UTIL{$1},"namespace","$1","exported_namespace","(.+)"),"pod","$1","exported_pod","(.+)"),"container","$1","exported_container","(.+)")) )/100 , 0.001)`,
	"container_gpu_memory_usage":      `(sum by(namespace, pod, container) (label_replace(label_replace(label_replace(DCGM_FI_DEV_FB_USED{$1},"namespace","$1","exported_namespace","(.+)"),"pod","$1","exported_pod","(.+)"),"container","$1","exported_container","(.+)") )) * 1024 * 1024`,
	"container_processes_usage":       `sum by (namespace, pod, container) (container_processes{job=~"kubelet|kubeedge", container!="POD", container!="", image!="", $1})`,
	"container_threads_usage":         `sum by (namespace, pod, container) (container_threads {job=~"kubelet|kubeedge", container!="POD", container!="", image!="", $1})`,

	"container_npu_usage":        `round((sum by (id, namespace, pod, container) (label_replace(label_replace(container_npu_utilization{$1}, "pod", "$1", "pod_name", "(.+)"), "container", "$1", "container_name", "(.+)"))) / 100, 0.001)`,
	"container_npu_memory_usage": `(sum by (id, namespace, pod, container_name) (label_replace(label_replace(container_npu_used_memory{$1}, "pod", "$1", "pod_name", "(.+)"), "container", "$1", "container_name", "(.+)") )) * 1024 * 1024`,

	// pvc
	"pvc_inodes_available":   `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_inodes_free) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,
	"pvc_inodes_used":        `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_inodes_used) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,
	"pvc_inodes_total":       `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_inodes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,
	"pvc_inodes_utilisation": `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_inodes_used / kubelet_volume_stats_inodes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,
	"pvc_bytes_available":    `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_available_bytes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,
	"pvc_bytes_used":         `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_used_bytes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,
	"pvc_bytes_total":        `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_capacity_bytes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,
	"pvc_bytes_utilisation":  `max by (namespace, persistentvolumeclaim) (kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes) * on (namespace, persistentvolumeclaim) group_left (storageclass) kube_persistentvolumeclaim_info{$1}`,

	// component
	"etcd_server_list":                           `label_replace(up{job="etcd"}, "node_ip", "$1", "instance", "(.*):.*")`,
	"etcd_server_total":                          `count(up{job="etcd"})`,
	"etcd_server_up_total":                       `etcd:up:sum`,
	"etcd_server_has_leader":                     `label_replace(etcd_server_has_leader, "node_ip", "$1", "instance", "(.*):.*")`,
	"etcd_server_is_leader":                      `label_replace(etcd_server_is_leader, "node_ip", "$1", "instance", "(.*):.*")`,
	"etcd_server_leader_changes":                 `label_replace(etcd:etcd_server_leader_changes_seen:sum_changes, "node_ip", "$1", "node", "(.*)")`,
	"etcd_server_proposals_failed_rate":          `avg(etcd:etcd_server_proposals_failed:sum_irate)`,
	"etcd_server_proposals_applied_rate":         `avg(etcd:etcd_server_proposals_applied:sum_irate)`,
	"etcd_server_proposals_committed_rate":       `avg(etcd:etcd_server_proposals_committed:sum_irate)`,
	"etcd_server_proposals_pending_count":        `avg(etcd:etcd_server_proposals_pending:sum)`,
	"etcd_mvcc_db_size":                          `avg(etcd:etcd_mvcc_db_total_size:sum)`,
	"etcd_network_client_grpc_received_bytes":    `sum(etcd:etcd_network_client_grpc_received_bytes:sum_irate)`,
	"etcd_network_client_grpc_sent_bytes":        `sum(etcd:etcd_network_client_grpc_sent_bytes:sum_irate)`,
	"etcd_grpc_call_rate":                        `sum(etcd:grpc_server_started:sum_irate)`,
	"etcd_grpc_call_failed_rate":                 `sum(etcd:grpc_server_handled:sum_irate)`,
	"etcd_grpc_server_msg_received_rate":         `sum(etcd:grpc_server_msg_received:sum_irate)`,
	"etcd_grpc_server_msg_sent_rate":             `sum(etcd:grpc_server_msg_sent:sum_irate)`,
	"etcd_disk_wal_fsync_duration":               `avg(etcd:etcd_disk_wal_fsync_duration:avg)`,
	"etcd_disk_wal_fsync_duration_quantile":      `avg(etcd:etcd_disk_wal_fsync_duration:histogram_quantile) by (quantile)`,
	"etcd_disk_backend_commit_duration":          `avg(etcd:etcd_disk_backend_commit_duration:avg)`,
	"etcd_disk_backend_commit_duration_quantile": `avg(etcd:etcd_disk_backend_commit_duration:histogram_quantile) by (quantile)`,

	"apiserver_up_sum":                    `apiserver:up:sum`,
	"apiserver_request_rate":              `apiserver:apiserver_request_total:sum_irate`,
	"apiserver_request_by_verb_rate":      `apiserver:apiserver_request_total:sum_verb_irate`,
	"apiserver_request_latencies":         `apiserver:apiserver_request_duration:avg`,
	"apiserver_request_by_verb_latencies": `apiserver:apiserver_request_duration:avg_by_verb`,

	"scheduler_up_sum":                          `scheduler:up:sum`,
	"scheduler_schedule_attempts":               `scheduler:scheduler_schedule_attempts:sum`,
	"scheduler_schedule_attempt_rate":           `scheduler:scheduler_schedule_attempts:sum_rate`,
	"scheduler_e2e_scheduling_latency":          `scheduler:scheduler_e2e_scheduling_duration:avg`,
	"scheduler_e2e_scheduling_latency_quantile": `scheduler:scheduler_e2e_scheduling_duration:histogram_quantile`,
}

var protectedMetrics = map[string]bool{
	"workspace_gpu_usage":        false,
	"workspace_gpu_memory_usage": false,
	"namespace_gpu_usage":        true,
	"namespace_gpu_memory_usage": true,
	"workload_gpu_usage":         true,
	"workload_gpu_memory_usage":  true,
	"pod_gpu_usage":              true,
	"pod_gpu_memory_usage":       true,
	"container_gpu_usage":        true,
	"container_gpu_memory_usage": true,
	// npu
	"pod_npu_usage":              true,
	"pod_npu_memory_usage":       true,
	"container_npu_usage":        true,
	"container_npu_memory_usage": true,
}

var wrappedGpuQueryMetrics = map[string]bool{
	"container_gpu_usage":        true,
	"container_gpu_memory_usage": true,
}

var wrappedNpuQueryMetrics = map[string]bool{
	"container_npu_usage":        true,
	"container_npu_memory_usage": true,
}

func makeExpr(metric string, opts monitoring.QueryOptions) string {
	// Consider the "$1" in label_replace:
	// wrappedExpr converts `"$1"` to `$labelReplace`,
	// once completed, will convert back to `"$1"`.
	tmpl := promQLTemplates[metric]
	_, protected := protectedMetrics[metric]
	if protected {
		tmpl = wrappedExpr(tmpl)
	}
	tmpl = templateExpr(metric, tmpl, opts)
	if protected {
		tmpl = unWrappedExpr(tmpl)
	}
	_, wrappedQueryFlag := wrappedGpuQueryMetrics[metric]
	if wrappedQueryFlag {
		tmpl = strings.NewReplacer("namespace=", "exported_namespace=", "pod=", "exported_pod=", "container=", "exported_container=").Replace(tmpl)
	}

	_, npuContainerQueryFlag := wrappedNpuQueryMetrics[metric]
	if npuContainerQueryFlag {
		tmpl = strings.NewReplacer("pod=", "pod_name=", "container=", "container_name=").Replace(tmpl)
	}

	return tmpl
}

func templateExpr(metric string, tmpl string, opts monitoring.QueryOptions) string {
	switch opts.Level {
	case monitoring.LevelCluster:
		return tmpl
	case monitoring.LevelNode:
		return makeNodeMetricExpr(tmpl, opts)
	case monitoring.LevelWorkspace:
		return makeWorkspaceMetricExpr(tmpl, opts)
	case monitoring.LevelNamespace:
		return makeNamespaceMetricExpr(tmpl, opts)
	case monitoring.LevelWorkload:
		return makeWorkloadMetricExpr(metric, tmpl, opts)
	case monitoring.LevelPod:
		return makePodMetricExpr(tmpl, opts)
	case monitoring.LevelContainer:
		return makeContainerMetricExpr(tmpl, opts)
	case monitoring.LevelPVC:
		return makePVCMetricExpr(tmpl, opts)
	case monitoring.LevelIngress:
		return makeIngressMetricExpr(tmpl, opts)
	case monitoring.LevelComponent:
		return tmpl
	default:
		return tmpl
	}
}

func makeNodeMetricExpr(tmpl string, o monitoring.QueryOptions) string {
	var nodeSelector string
	if o.NodeName != "" {
		nodeSelector = fmt.Sprintf(`node="%s"`, o.NodeName)
	} else {
		nodeSelector = fmt.Sprintf(`node=~"%s"`, o.ResourceFilter)
	}
	return strings.Replace(tmpl, "$1", nodeSelector, -1)
}

func makeWorkspaceMetricExpr(tmpl string, o monitoring.QueryOptions) string {
	var workspaceSelector string
	if o.WorkspaceName != "" {
		workspaceSelector = fmt.Sprintf(`workspace="%s"`, o.WorkspaceName)
	} else {
		workspaceSelector = fmt.Sprintf(`workspace=~"%s", workspace!=""`, o.ResourceFilter)
	}
	return strings.Replace(tmpl, "$1", workspaceSelector, -1)
}

func makeNamespaceMetricExpr(tmpl string, o monitoring.QueryOptions) string {
	var namespaceSelector string

	// For monitoring namespaces in the specific workspace
	// GET /workspaces/{workspace}/namespaces
	if o.WorkspaceName != "" {
		namespaceSelector = fmt.Sprintf(`workspace="%s", namespace=~"%s"`, o.WorkspaceName, o.ResourceFilter)
		return strings.Replace(tmpl, "$1", namespaceSelector, -1)
	}

	// For monitoring the specific namespaces
	// GET /namespaces/{namespace} or
	// GET /namespaces
	if o.NamespaceName != "" {
		namespaceSelector = fmt.Sprintf(`namespace="%s"`, o.NamespaceName)
	} else {
		namespaceSelector = fmt.Sprintf(`namespace=~"%s"`, o.ResourceFilter)
	}
	return strings.Replace(tmpl, "$1", namespaceSelector, -1)
}

func makeWorkloadMetricExpr(metric, tmpl string, o monitoring.QueryOptions) string {
	var kindSelector, workloadSelector string

	switch o.WorkloadKind {
	case "deployment":
		o.WorkloadKind = Deployment
	case "statefulset":
		o.WorkloadKind = StatefulSet
	case "daemonset":
		o.WorkloadKind = DaemonSet
	default:
		o.WorkloadKind = "(Deployment|StatefulSet|DaemonSet|Job|CronJob)"
	}
	workloadSelector = fmt.Sprintf(`namespace="%s", workload=~"%s:(%s)"`, o.NamespaceName, o.WorkloadKind, o.ResourceFilter)

	if strings.Contains(metric, "deployment") {
		kindSelector = fmt.Sprintf(`namespace="%s", deployment!="", deployment=~"%s"`, o.NamespaceName, o.ResourceFilter)
	}
	if strings.Contains(metric, "statefulset") {
		kindSelector = fmt.Sprintf(`namespace="%s", statefulset!="", statefulset=~"%s"`, o.NamespaceName, o.ResourceFilter)
	}
	if strings.Contains(metric, "daemonset") {
		kindSelector = fmt.Sprintf(`namespace="%s", daemonset!="", daemonset=~"%s"`, o.NamespaceName, o.ResourceFilter)
	}

	return strings.NewReplacer("$1", workloadSelector, "$2", kindSelector).Replace(tmpl)
}

func makePodMetricExpr(tmpl string, o monitoring.QueryOptions) string {
	var podSelector, workloadSelector string

	// For monitoriong pods of the specific workload
	// GET /namespaces/{namespace}/workloads/{kind}/{workload}/pods
	if o.WorkloadName != "" {
		switch o.WorkloadKind {
		case "deployment":
			workloadSelector = fmt.Sprintf(`owner_kind="ReplicaSet", owner_name=~"^%s-[^-]{1,10}$"`, o.WorkloadName)
		case "statefulset":
			workloadSelector = fmt.Sprintf(`owner_kind="StatefulSet", owner_name="%s"`, o.WorkloadName)
		case "daemonset":
			workloadSelector = fmt.Sprintf(`owner_kind="DaemonSet", owner_name="%s"`, o.WorkloadName)
		}
	}

	// For monitoring pods in the specific namespace
	// GET /namespaces/{namespace}/workloads/{kind}/{workload}/pods or
	// GET /namespaces/{namespace}/pods/{pod} or
	// GET /namespaces/{namespace}/pods
	if o.NamespaceName != "" {
		if o.PodName != "" {
			podSelector = fmt.Sprintf(`pod="%s", namespace="%s"`, o.PodName, o.NamespaceName)
		} else {
			podSelector = fmt.Sprintf(`pod=~"%s", namespace="%s"`, o.ResourceFilter, o.NamespaceName)
		}
	} else {
		var namespaces, pods []string
		if o.NamespacedResourcesFilter != "" {
			for _, np := range strings.Split(o.NamespacedResourcesFilter, "|") {
				if nparr := strings.SplitN(np, "/", 2); len(nparr) > 1 {
					namespaces = append(namespaces, nparr[0])
					pods = append(pods, nparr[1])
				} else {
					pods = append(pods, np)
				}
			}
		}
		// For monitoring pods on the specific node
		// GET /nodes/{node}/pods/{pod}
		// GET /nodes/{node}/pods
		if o.NodeName != "" {
			if o.PodName != "" {
				if nparr := strings.SplitN(o.PodName, "/", 2); len(nparr) > 1 {
					podSelector = fmt.Sprintf(`namespace="%s",pod="%s", node="%s"`, nparr[0], nparr[1], o.NodeName)
				} else {
					podSelector = fmt.Sprintf(`pod="%s", node="%s"`, o.PodName, o.NodeName)
				}
			} else {
				var ps []string
				ps = append(ps, fmt.Sprintf(`node="%s"`, o.NodeName))
				if o.ResourceFilter != "" {
					ps = append(ps, fmt.Sprintf(`pod=~"%s"`, o.ResourceFilter))
				}

				if len(namespaces) > 0 {
					ps = append(ps, fmt.Sprintf(`namespace=~"%s"`, strings.Join(namespaces, "|")))
				}
				if len(pods) > 0 {
					ps = append(ps, fmt.Sprintf(`pod=~"%s"`, strings.Join(pods, "|")))
				}
				podSelector = strings.Join(ps, ",")
			}
		} else {
			// For monitoring pods in the whole cluster
			// Get /pods
			var ps []string
			if len(namespaces) > 0 {
				ps = append(ps, fmt.Sprintf(`namespace=~"%s"`, strings.Join(namespaces, "|")))
			}
			if len(pods) > 0 {
				ps = append(ps, fmt.Sprintf(`pod=~"%s"`, strings.Join(pods, "|")))
			}
			if len(ps) > 0 {
				podSelector = strings.Join(ps, ",")
			}
		}
	}

	var filetr string
	if workloadSelector == "" || podSelector == "" {
		filetr = workloadSelector + podSelector
	} else {
		filetr = strings.Join([]string{workloadSelector, podSelector}, ",")
	}

	return strings.NewReplacer("$1", filetr).Replace(tmpl)
}

func makeContainerMetricExpr(tmpl string, o monitoring.QueryOptions) string {
	var containerSelector string
	if o.ContainerName != "" {
		containerSelector = fmt.Sprintf(`pod="%s", namespace="%s", container="%s"`, o.PodName, o.NamespaceName, o.ContainerName)
	} else {
		containerSelector = fmt.Sprintf(`pod="%s", namespace="%s", container=~"%s"`, o.PodName, o.NamespaceName, o.ResourceFilter)
	}
	return strings.Replace(tmpl, "$1", containerSelector, -1)
}

func makePVCMetricExpr(tmpl string, o monitoring.QueryOptions) string {
	var pvcSelector string

	// For monitoring persistentvolumeclaims in the specific namespace
	// GET /namespaces/{namespace}/persistentvolumeclaims/{persistentvolumeclaim} or
	// GET /namespaces/{namespace}/persistentvolumeclaims
	if o.NamespaceName != "" {
		if o.PersistentVolumeClaimName != "" {
			pvcSelector = fmt.Sprintf(`namespace="%s", persistentvolumeclaim="%s"`, o.NamespaceName, o.PersistentVolumeClaimName)
		} else {
			pvcSelector = fmt.Sprintf(`namespace="%s", persistentvolumeclaim=~"%s"`, o.NamespaceName, o.ResourceFilter)
		}
		return strings.Replace(tmpl, "$1", pvcSelector, -1)
	}

	// For monitoring persistentvolumeclaims of the specific storageclass
	// GET /storageclasses/{storageclass}/persistentvolumeclaims
	if o.StorageClassName != "" {
		pvcSelector = fmt.Sprintf(`storageclass="%s", persistentvolumeclaim=~"%s"`, o.StorageClassName, o.ResourceFilter)
	}
	return strings.Replace(tmpl, "$1", pvcSelector, -1)
}

func makeIngressMetricExpr(tmpl string, o monitoring.QueryOptions) string {
	var ingressSelector string
	var jobSelector string
	duration := "5m"

	// parse Range Vector Selectors metric{key=value}[duration]
	if o.Duration != nil {
		duration = o.Duration.String()
	}

	// For monitoring ingress in the specific namespace
	// GET /namespaces/{namespace}/ingress/{ingress} or
	// GET /namespaces/{namespace}/ingress
	if o.NamespaceName != constants.KubeSphereNamespace {
		if o.Ingress != "" {
			ingressSelector = fmt.Sprintf(`exported_namespace="%s", ingress="%s"`, o.NamespaceName, o.Ingress)
		} else {
			ingressSelector = fmt.Sprintf(`exported_namespace="%s", ingress=~"%s"`, o.NamespaceName, o.ResourceFilter)
		}
	} else {
		if o.Ingress != "" {
			ingressSelector = fmt.Sprintf(`ingress="%s"`, o.Ingress)
		} else {
			ingressSelector = fmt.Sprintf(`ingress=~"%s"`, o.ResourceFilter)
		}
	}

	// job is a reqiuried filter
	// GET /namespaces/{namespace}/ingress?job=xxx&pod=xxx
	if o.Job != "" {
		jobSelector = fmt.Sprintf(`job="%s"`, o.Job)
		if o.PodName != "" {
			jobSelector = fmt.Sprintf(`%s,controller_pod="%s"`, jobSelector, o.PodName)
		}
	}

	tmpl = strings.Replace(tmpl, "$1", ingressSelector, -1)
	tmpl = strings.Replace(tmpl, "$2", jobSelector, -1)
	return strings.Replace(tmpl, "$3", duration, -1)
}

// wrappedExpr converts `"$1"`  to `$labelReplace`
func wrappedExpr(tmpl string) string {
	return strings.Replace(tmpl, "\"$1\"", "$labelReplace", -1)
}

// wrappedExpr converts `$labelReplace` back to `"$1"`
func unWrappedExpr(tmpl string) string {
	return strings.Replace(tmpl, "$labelReplace", "\"$1\"", -1)
}
