package apptemplateversion

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	appsv1alpha1 "github.com/edgewize-io/edgewize/pkg/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

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

func isAppTemplateVersionChanged(newVersion, oldVersion *appsv1alpha1.AppTemplateVersion) bool {
	newContainers := []corev1.Container{}
	oldContainers := []corev1.Container{}
	newContainers = append(newContainers, newVersion.Spec.DeploymentSpec.Template.Spec.Containers...)
	oldContainers = append(oldContainers, oldVersion.Spec.DeploymentSpec.Template.Spec.Containers...)
	if hashDeployments(newContainers...) != hashDeployments(oldContainers...) {
		return true
	}
	return false
}

func mergeContainers(currentContainers, newContainers []corev1.Container) []corev1.Container {
	newContainerMap := make(map[string]corev1.Container)
	for _, container := range newContainers {
		newContainerMap[container.Name] = container
	}

	// TODO: update other fields
	// for _, container := range currentContainers {
	// 	if _, ok := newContainerMap[container.Name]; !ok {
	//    ...
	// 	}
	// }

	ret := make([]corev1.Container, 0)
	for _, container := range newContainerMap {
		ret = append(ret, container)
	}
	return ret
}

func mergeDeployments(deployments ...[]corev1.Container) []corev1.Container {
	ret := make([]corev1.Container, 0)
	for _, deployment := range deployments {
		ret = append(ret, deployment...)
	}
	return ret
}

func hashDeployments(deployments ...corev1.Container) string {
	hash := sha256.New()
	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})
	for _, deployment := range deployments {
		ret, err := json.Marshal(deployment)
		if err != nil {
			return ""
		}
		hash.Write(ret)
	}
	return hex.EncodeToString(hash.Sum(nil))[:10]
}
