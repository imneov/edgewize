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

package apptemplateversion

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	"k8s.io/klog/v2"
)

func getAppTemplateSelector(appTemplateVersion *appsv1alpha1.AppTemplateVersion) labels.Selector {
	if appTemplateVersion == nil {
		klog.Errorf("appTemplateVersion is nil")
		return nil
	}

	lbs := appTemplateVersion.Labels
	if lbs == nil {
		klog.Errorf("appTemplateVersion is nil")
		return nil
	}

	appName, ok := lbs[appsv1alpha1.LabelAppTemplate]
	if !ok {
		klog.Errorf("appTemplateName is not found")
		return nil
	}

	versionName, ok := lbs[appsv1alpha1.LabelAppTemplateVersion]
	if !ok {
		klog.Errorf("appTemplateVersionName is not found")
		return nil
	}

	return labels.SelectorFromSet(map[string]string{
		appsv1alpha1.LabelAppTemplate:        appName,
		appsv1alpha1.LabelAppTemplateVersion: versionName,
	})
}

// UpdateInstanceAndDeployments Update the Workload when the AppTemplateVersion change
func (r *Reconciler) UpdateInstanceAndDeployments(ctx context.Context, new *appsv1alpha1.AppTemplateVersion) error {
	err := r.UpdateDeployments(ctx, new)
	if err != nil {
		return err
	}
	return r.UpdateEdgeAppSets(ctx, new)
}

func (r *Reconciler) UpdateEdgeAppSets(ctx context.Context, new *appsv1alpha1.AppTemplateVersion) error {
	appsets := &appsv1alpha1.EdgeAppSetList{}
	selector := getAppTemplateSelector(new)

	err := r.List(ctx, appsets, &client.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}

	for _, appset := range appsets.Items {
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
			nn := types.NamespacedName{
				Name:      appset.Name,
				Namespace: appset.Namespace,
			}
			instance := &appsv1alpha1.EdgeAppSet{}
			if err = r.Get(ctx, nn, instance); err != nil {
				return client.IgnoreNotFound(err)
			}
			// only update containers
			instance.Spec.DeploymentTemplate.Spec.Template.Spec.Containers = mergeContainers(
				instance.Spec.DeploymentTemplate.Spec.Template.Spec.Containers,
				new.Spec.DeploymentSpec.Template.Spec.Containers,
			)
			instance.Spec.DeploymentTemplate.Spec.Template.Spec.InitContainers = mergeContainers(
				instance.Spec.DeploymentTemplate.Spec.Template.Spec.InitContainers,
				new.Spec.DeploymentSpec.Template.Spec.InitContainers,
			)
			containers := mergeDeployments(
				instance.Spec.DeploymentTemplate.Spec.Template.Spec.Containers,
				instance.Spec.DeploymentTemplate.Spec.Template.Spec.InitContainers,
			)
			hash := hashDeployments(containers...)
			labels := instance.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			labels[appsv1alpha1.LabelAppTemplateHash] = hash
			instance.Labels = labels

			labels = instance.Spec.DeploymentTemplate.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			labels[appsv1alpha1.LabelAppTemplateHash] = hash
			instance.Spec.DeploymentTemplate.Labels = labels

			return r.Update(ctx, instance)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) UpdateDeployments(ctx context.Context, new *appsv1alpha1.AppTemplateVersion) error {
	deployments := &appsv1.DeploymentList{}
	selector := getAppTemplateSelector(new)
	err := r.List(ctx, deployments, &client.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}

	for _, deployment := range deployments.Items {
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
			nn := types.NamespacedName{
				Name:      deployment.Name,
				Namespace: deployment.Namespace,
			}
			instance := &appsv1.Deployment{}
			if err = r.Get(ctx, nn, instance); err != nil {
				return client.IgnoreNotFound(err)
			}
			// only update containers
			instance.Spec.Template.Spec.Containers = mergeContainers(
				instance.Spec.Template.Spec.Containers,
				new.Spec.DeploymentSpec.Template.Spec.Containers,
			)
			instance.Spec.Template.Spec.InitContainers = mergeContainers(
				instance.Spec.Template.Spec.InitContainers,
				new.Spec.DeploymentSpec.Template.Spec.InitContainers,
			)
			containers := mergeDeployments(
				instance.Spec.Template.Spec.Containers,
				instance.Spec.Template.Spec.InitContainers,
			)
			hash := hashDeployments(containers...)
			labels := instance.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			labels[appsv1alpha1.LabelAppTemplateHash] = hash
			instance.Labels = labels

			labels = instance.Spec.Template.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			labels[appsv1alpha1.LabelAppTemplateHash] = hash
			instance.Spec.Template.Labels = labels

			err = r.Update(ctx, instance)
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}
