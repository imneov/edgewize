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

package monitoring

import (
	"fmt"
	"time"

	ksinformers "github.com/edgewize-io/edgewize/pkg/client/informers/externalversions"
	"github.com/edgewize-io/edgewize/pkg/informers"
	"github.com/edgewize-io/edgewize/pkg/models/monitoring/expressions"
	resourcev1alpha3 "github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3/resource"
	"github.com/edgewize-io/edgewize/pkg/simple/client/monitoring"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type MonitoringOperator interface {
	GetMetric(expr, namespace string, time time.Time) (monitoring.Metric, error)
	GetMetricOverTime(expr, namespace string, start, end time.Time, step time.Duration) (monitoring.Metric, error)
	GetNamedMetrics(metrics []string, time time.Time, opt monitoring.QueryOption) Metrics
	GetNamedMetricsOverTime(metrics []string, start, end time.Time, step time.Duration, opt monitoring.QueryOption) Metrics
	GetMetadata(namespace string) Metadata
	GetMetricLabelSet(metric, namespace string, start, end time.Time) MetricLabelSet
}

type monitoringOperator struct {
	prometheus     monitoring.Interface
	metricsserver  monitoring.Interface
	k8s            kubernetes.Interface
	ks             ksinformers.SharedInformerFactory
	resourceGetter *resourcev1alpha3.ResourceGetter
}

func NewMonitoringOperator(monitoringClient monitoring.Interface, metricsClient monitoring.Interface, k8s kubernetes.Interface, factory informers.InformerFactory, resourceGetter *resourcev1alpha3.ResourceGetter) MonitoringOperator {
	return &monitoringOperator{
		prometheus:     monitoringClient,
		metricsserver:  metricsClient,
		k8s:            k8s,
		ks:             factory.KubeSphereSharedInformerFactory(),
		resourceGetter: resourceGetter,
	}
}

func (mo monitoringOperator) GetMetric(expr, namespace string, time time.Time) (monitoring.Metric, error) {
	if namespace != "" {
		// Different monitoring backend implementations have different ways to enforce namespace isolation.
		// Each implementation should register itself to `ReplaceNamespaceFns` during init().
		// We hard code "prometheus" here because we only support this datasource so far.
		// In the future, maybe the value should be returned from a method like `mo.c.GetMonitoringServiceName()`.
		var err error
		expr, err = expressions.ReplaceNamespaceFns["prometheus"](expr, namespace)
		if err != nil {
			return monitoring.Metric{}, err
		}
	}
	return mo.prometheus.GetMetric(expr, time), nil
}

func (mo monitoringOperator) GetMetricOverTime(expr, namespace string, start, end time.Time, step time.Duration) (monitoring.Metric, error) {
	if namespace != "" {
		// Different monitoring backend implementations have different ways to enforce namespace isolation.
		// Each implementation should register itself to `ReplaceNamespaceFns` during init().
		// We hard code "prometheus" here because we only support this datasource so far.
		// In the future, maybe the value should be returned from a method like `mo.c.GetMonitoringServiceName()`.
		var err error
		expr, err = expressions.ReplaceNamespaceFns["prometheus"](expr, namespace)
		if err != nil {
			return monitoring.Metric{}, err
		}
	}
	return mo.prometheus.GetMetricOverTime(expr, start, end, step), nil
}

func (mo monitoringOperator) GetNamedMetrics(metrics []string, time time.Time, opt monitoring.QueryOption) Metrics {
	ress := mo.prometheus.GetNamedMetrics(metrics, time, opt)

	opts := &monitoring.QueryOptions{}
	opt.Apply(opts)

	var isNodeRankingQuery bool
	if opts.QueryType == "rank" {
		isNodeRankingQuery = true
	}

	if mo.metricsserver != nil {
		//Merge edge node metrics data
		edgeMetrics := make(map[string]monitoring.MetricData)

		for i, ressMetric := range ress {
			metricName := ressMetric.MetricName
			ressMetricValues := ressMetric.MetricData.MetricValues
			if len(ressMetricValues) == 0 || isNodeRankingQuery {
				// this metric has no prometheus metrics data or the request need to list all nodes metrics
				if len(edgeMetrics) == 0 {
					// start to request monintoring metricsApi data
					mr := mo.metricsserver.GetNamedMetrics(metrics, time, opt)
					for _, mrMetric := range mr {
						edgeMetrics[mrMetric.MetricName] = mrMetric.MetricData
					}
				}
				if val, ok := edgeMetrics[metricName]; ok {
					ress[i].MetricData.MetricValues = append(ress[i].MetricData.MetricValues, val.MetricValues...)
				}
			}

		}
	}

	return Metrics{Results: ress}
}

func (mo monitoringOperator) GetNamedMetricsOverTime(metrics []string, start, end time.Time, step time.Duration, opt monitoring.QueryOption) Metrics {
	ress := mo.prometheus.GetNamedMetricsOverTime(metrics, start, end, step, opt)

	if mo.metricsserver != nil {

		//Merge edge node metrics data
		edgeMetrics := make(map[string]monitoring.MetricData)

		for i, ressMetric := range ress {
			metricName := ressMetric.MetricName
			ressMetricValues := ressMetric.MetricData.MetricValues
			if len(ressMetricValues) == 0 {
				// this metric has no prometheus metrics data
				if len(edgeMetrics) == 0 {
					// start to request monintoring metricsApi data
					mr := mo.metricsserver.GetNamedMetricsOverTime(metrics, start, end, step, opt)
					for _, mrMetric := range mr {
						edgeMetrics[mrMetric.MetricName] = mrMetric.MetricData
					}
				}
				if val, ok := edgeMetrics[metricName]; ok {
					ress[i].MetricData.MetricValues = append(ress[i].MetricData.MetricValues, val.MetricValues...)
				}
			}
		}
	}

	return Metrics{Results: ress}
}

func (mo monitoringOperator) GetMetadata(namespace string) Metadata {
	// Change the query metric name implementation without changing the response format.
	// Use GetLabelValue() instead of GetMetadata() to query metric name.
	// The Api will be updated in kubesphere version 4.0
	var label string = "__name__"
	start, end := time.Now().Add(-1*time.Hour), time.Now()
	var matches []string

	if namespace != "" {
		matches = append(matches, fmt.Sprintf(`{namespace="%s"}`, namespace))
	}
	labelValues := mo.prometheus.GetLabelValues(label, matches, start, end)

	var data []monitoring.Metadata
	for _, labelValue := range labelValues {
		data = append(data, monitoring.Metadata{
			Metric: labelValue,
		})
	}
	return Metadata{Data: data}
}

func (mo monitoringOperator) GetMetricLabelSet(metric, namespace string, start, end time.Time) MetricLabelSet {
	var expr = metric
	var err error
	if namespace != "" {
		// Different monitoring backend implementations have different ways to enforce namespace isolation.
		// Each implementation should register itself to `ReplaceNamespaceFns` during init().
		// We hard code "prometheus" here because we only support this datasource so far.
		// In the future, maybe the value should be returned from a method like `mo.c.GetMonitoringServiceName()`.
		expr, err = expressions.ReplaceNamespaceFns["prometheus"](metric, namespace)
		if err != nil {
			klog.Error(err)
			return MetricLabelSet{}
		}
	}
	data := mo.prometheus.GetMetricLabelSet(expr, start, end)
	return MetricLabelSet{Data: data}
}
