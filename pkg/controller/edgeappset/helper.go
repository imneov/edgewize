package edgeappset

import (
	"fmt"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

// build deployment with appsv1alpha1.NodeSelector
func buildDeployment(instance *appsv1alpha1.EdgeAppSet, selector appsv1alpha1.NodeSelector) *appsv1.Deployment {
	name := fmt.Sprintf("%s-%s", instance.Name, rand.String(5))
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: selector.Project,
			Labels: map[string]string{
				"app":                                name,
				appsv1alpha1.LabelEdgeAppSet:         instance.Name,
				appsv1alpha1.LabelNodeGroup:          selector.NodeGroup,
				appsv1alpha1.LabelNode:               selector.NodeName,
				appsv1alpha1.LabelAppTemplate:        instance.Spec.AppTemplateName,
				appsv1alpha1.LabelAppTemplateVersion: instance.Spec.Version,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: instance.Spec.DeploymentTemplate.Spec.Replicas,
			Template: instance.Spec.DeploymentTemplate.Spec.Template,
			Strategy: instance.Spec.DeploymentTemplate.Spec.Strategy,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                        name,
					appsv1alpha1.LabelEdgeAppSet: instance.Name,
				},
			},
		},
	}
	deployment.Spec.Template.Labels = map[string]string{
		"app":                                name,
		appsv1alpha1.LabelEdgeAppSet:         instance.Name,
		appsv1alpha1.LabelNodeGroup:          selector.NodeGroup,
		appsv1alpha1.LabelNode:               selector.NodeName,
		appsv1alpha1.LabelAppTemplate:        instance.Spec.AppTemplateName,
		appsv1alpha1.LabelAppTemplateVersion: instance.Spec.Version,
	}

	// Fix https://github.com/kubeedge/kubeedge/issues/3736
	deployment.Spec.Template.Spec.Tolerations = []corev1.Toleration{{
		Key:      corev1.TaintNodeUnreachable,
		Operator: corev1.TolerationOpExists,
	}}

	// 部署到指定节点
	if selector.NodeName != "" {
		deployment.Spec.Template.Spec.NodeName = selector.NodeName
	}
	return deployment
}

func readyWorkloadCounts(deployments []appsv1.Deployment) int {
	count := 0
	for _, deploy := range deployments {
		if deploy.Status.ReadyReplicas == deploy.Status.Replicas {
			count++
		}
	}
	return count
}

func unavailableWorkloadCounts(deployments []appsv1.Deployment) int {
	count := 0
	for _, deploy := range deployments {
		if deploy.Status.UnavailableReplicas > 0 {
			count++
		}
	}
	return count
}
