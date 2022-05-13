package controller

import (
	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// makeResources creates deployment resource limits and requests requirements from function specs
func makeResources(sharepod *faasv1.SharePod) (*corev1.ResourceRequirements, error) {
	//TODO here: modity in order to fit for sharepod
	resources := &corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	// Set Memory limits
	if sharepod.ObjectMeta.Annotations[] != nil && len(sharepod.Spec.Limits.Memory) > 0 {
		qty, err := resource.ParseQuantity(sharepod.Spec.Limits.Memory)
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceMemory] = qty
	}
	if sharepod.Spec.Requests != nil && len(sharepod.Spec.Requests.Memory) > 0 {
		qty, err := resource.ParseQuantity(sharepod.Spec.Requests.Memory)
		if err != nil {
			return resources, err
		}
		resources.Requests[corev1.ResourceMemory] = qty
	}

	// Set CPU limits
	if sharepod.Spec.Limits != nil && len(sharepod.Spec.Limits.CPU) > 0 {
		qty, err := resource.ParseQuantity(sharepod.Spec.Limits.CPU)
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceCPU] = qty
	}
	if sharepod.Spec.Requests != nil && len(sharepod.Spec.Requests.CPU) > 0 {
		qty, err := resource.ParseQuantity(sharepod.Spec.Requests.CPU)
		if err != nil {
			return resources, err
		}
		resources.Requests[corev1.ResourceCPU] = qty
	}

	return resources, nil
}
