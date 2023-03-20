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

package cronjob

import (
	"strings"

	"github.com/edgewize-io/edgewize/pkg/api"
	"github.com/edgewize-io/edgewize/pkg/apiserver/query"
	"github.com/edgewize-io/edgewize/pkg/models/resources/v1alpha3"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
)

type cronJobGetter struct {
	informer informers.SharedInformerFactory
}

func New(informer informers.SharedInformerFactory) v1alpha3.Interface {
	return &cronJobGetter{informer: informer}
}

func (c *cronJobGetter) Get(namespace, name string) (runtime.Object, error) {
	return c.informer.Batch().V1beta1().CronJobs().Lister().CronJobs(namespace).Get(name)
}

func (c *cronJobGetter) List(namespace string, query *query.Query) (*api.ListResult, error) {
	jobs, err := c.informer.Batch().V1beta1().CronJobs().Lister().CronJobs(namespace).List(query.Selector())
	if err != nil {
		return nil, err
	}

	var result []runtime.Object
	for _, job := range jobs {
		result = append(result, job)
	}
	return v1alpha3.DefaultList(result, query, c.compare, c.filter), nil

}

func (c *cronJobGetter) compare(left runtime.Object, right runtime.Object, field query.Field) bool {
	leftJob, ok := left.(*v1beta1.CronJob)
	if !ok {
		return false
	}

	rightJob, ok := right.(*v1beta1.CronJob)
	if !ok {
		return false
	}

	switch field {
	case query.FieldLastScheduleTime:
		if leftJob.Status.LastScheduleTime == nil {
			return true
		}
		if rightJob.Status.LastScheduleTime == nil {
			return false
		}
		return leftJob.Status.LastScheduleTime.Before(rightJob.Status.LastScheduleTime)

	default:
		return v1alpha3.DefaultObjectMetaCompare(leftJob.ObjectMeta, rightJob.ObjectMeta, field)
	}
}

func (c *cronJobGetter) filter(object runtime.Object, filter query.Filter) bool {
	cronJob, ok := object.(*v1beta1.CronJob)
	if !ok {
		return false
	}

	switch filter.Field {
	case query.FieldStatus:
		return strings.Compare(cronJobStatus(cronJob), string(filter.Value)) == 0
	default:
		return v1alpha3.DefaultObjectMetaFilter(cronJob.ObjectMeta, filter)
	}
}

func cronJobStatus(item *v1beta1.CronJob) string {
	if item.Spec.Suspend != nil && *item.Spec.Suspend {
		return query.StatusPaused
	}
	return query.StatusRunning
}
