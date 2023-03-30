// Copyright 2022 The KubeSphere Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package monitoring

import (
	"fmt"
	"math/big"

	"k8s.io/klog/v2"

	"github.com/edgewize-io/edgewize/pkg/simple/client/monitoring"
)

const (
	METER_RESOURCE_TYPE_CPU = iota
	METER_RESOURCE_TYPE_MEM
	METER_RESOURCE_TYPE_NET_INGRESS
	METER_RESOURCE_TYPE_NET_EGRESS
	METER_RESOURCE_TYPE_PVC

	meteringDefaultPrecision = 10
	meteringFeePrecision     = 3
)

var meterResourceUnitMap = map[int]string{
	METER_RESOURCE_TYPE_CPU:         "cores",
	METER_RESOURCE_TYPE_MEM:         "bytes",
	METER_RESOURCE_TYPE_NET_INGRESS: "bytes",
	METER_RESOURCE_TYPE_NET_EGRESS:  "bytes",
	METER_RESOURCE_TYPE_PVC:         "bytes",
}

var MeterResourceMap = map[string]int{
	"meter_cluster_cpu_usage":                 METER_RESOURCE_TYPE_CPU,
	"meter_cluster_memory_usage":              METER_RESOURCE_TYPE_MEM,
	"meter_cluster_net_bytes_transmitted":     METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_cluster_net_bytes_received":        METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_cluster_pvc_bytes_total":           METER_RESOURCE_TYPE_PVC,
	"meter_node_cpu_usage":                    METER_RESOURCE_TYPE_CPU,
	"meter_node_memory_usage_wo_cache":        METER_RESOURCE_TYPE_MEM,
	"meter_node_net_bytes_transmitted":        METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_node_net_bytes_received":           METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_node_pvc_bytes_total":              METER_RESOURCE_TYPE_PVC,
	"meter_workspace_cpu_usage":               METER_RESOURCE_TYPE_CPU,
	"meter_workspace_memory_usage":            METER_RESOURCE_TYPE_MEM,
	"meter_workspace_net_bytes_transmitted":   METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_workspace_net_bytes_received":      METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_workspace_pvc_bytes_total":         METER_RESOURCE_TYPE_PVC,
	"meter_namespace_cpu_usage":               METER_RESOURCE_TYPE_CPU,
	"meter_namespace_memory_usage_wo_cache":   METER_RESOURCE_TYPE_MEM,
	"meter_namespace_net_bytes_transmitted":   METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_namespace_net_bytes_received":      METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_namespace_pvc_bytes_total":         METER_RESOURCE_TYPE_PVC,
	"meter_application_cpu_usage":             METER_RESOURCE_TYPE_CPU,
	"meter_application_memory_usage_wo_cache": METER_RESOURCE_TYPE_MEM,
	"meter_application_net_bytes_transmitted": METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_application_net_bytes_received":    METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_application_pvc_bytes_total":       METER_RESOURCE_TYPE_PVC,
	"meter_workload_cpu_usage":                METER_RESOURCE_TYPE_CPU,
	"meter_workload_memory_usage_wo_cache":    METER_RESOURCE_TYPE_MEM,
	"meter_workload_net_bytes_transmitted":    METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_workload_net_bytes_received":       METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_workload_pvc_bytes_total":          METER_RESOURCE_TYPE_PVC,
	"meter_service_cpu_usage":                 METER_RESOURCE_TYPE_CPU,
	"meter_service_memory_usage_wo_cache":     METER_RESOURCE_TYPE_MEM,
	"meter_service_net_bytes_transmitted":     METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_service_net_bytes_received":        METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_pod_cpu_usage":                     METER_RESOURCE_TYPE_CPU,
	"meter_pod_memory_usage_wo_cache":         METER_RESOURCE_TYPE_MEM,
	"meter_pod_net_bytes_transmitted":         METER_RESOURCE_TYPE_NET_EGRESS,
	"meter_pod_net_bytes_received":            METER_RESOURCE_TYPE_NET_INGRESS,
	"meter_pod_pvc_bytes_total":               METER_RESOURCE_TYPE_PVC,
}

func getMaxPointValue(points []monitoring.Point) string {
	var max *big.Float
	for i, p := range points {
		if i == 0 {
			max = new(big.Float).SetFloat64(p.Value())
		}

		pf := new(big.Float).SetFloat64(p.Value())
		if pf.Cmp(max) == 1 {
			max = pf
		}
	}

	return fmt.Sprintf(generateFloatFormat(meteringDefaultPrecision), max)
}

func getMinPointValue(points []monitoring.Point) string {
	var min *big.Float
	for i, p := range points {
		if i == 0 {
			min = new(big.Float).SetFloat64(p.Value())
		}

		pf := new(big.Float).SetFloat64(p.Value())
		if min.Cmp(pf) == 1 {
			min = pf
		}
	}

	return fmt.Sprintf(generateFloatFormat(meteringDefaultPrecision), min)
}

func getSumPointValue(points []monitoring.Point) string {
	sum := new(big.Float).SetFloat64(0)

	for _, p := range points {
		pf := new(big.Float).SetFloat64(p.Value())
		sum.Add(sum, pf)
	}

	return fmt.Sprintf(generateFloatFormat(meteringDefaultPrecision), sum)
}

func getAvgPointValue(points []monitoring.Point) string {
	sum, ok := new(big.Float).SetString(getSumPointValue(points))
	if !ok {
		klog.Error("failed to parse big.Float")
		return ""
	}

	length := new(big.Float).SetFloat64(float64(len(points)))

	return fmt.Sprintf(generateFloatFormat(meteringDefaultPrecision), sum.Quo(sum, length))
}

func generateFloatFormat(precision int) string {
	return "%." + fmt.Sprintf("%d", precision) + "f"
}

func getResourceUnit(meterName string) string {
	if resourceType, ok := MeterResourceMap[meterName]; !ok {
		klog.Errorf("invlaid meter %v", meterName)
		return ""
	} else {
		return meterResourceUnitMap[resourceType]
	}
}

func squashPoints(input []monitoring.Point, factor int) (output []monitoring.Point) {

	if factor <= 0 {
		klog.Errorln("factor should be positive")
		return nil
	}

	for i := 0; i < len(input); i++ {

		if i%factor == 0 {
			output = append([]monitoring.Point{input[len(input)-1-i]}, output...)
		} else {
			output[0] = output[0].Add(input[len(input)-1-i])
		}
	}

	return output
}
