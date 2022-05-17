package controller

import (
	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	sharepodV1 "github.com/Interstellarss/faas-share/pkg/sharepod"
)

// makeResources creates deployment resource limits and requests requirements from function specs
func makeResources(sharepod *faasv1.SharePod) (*sharepodV1.SharepodRequirements, error) {
	//TODO here: modity in order to fit for sharepod
	resources := &sharepodV1.SharepodRequirements{}

	// Set Memory limits
	if sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory])
		if err != nil {
			return resources, err
		}
		resources.Memory = qty
	}
	if sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit])
		if err != nil {
			return resources, err
		}
		resources.GPULimit = qty
	}

	// Set CPU limits
	if sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest])
		if err != nil {
			return resources, err
		}
		resources.GPURequest = qty
	}
	return resources, nil
}

/*
func makeResource(sharepod *faasv1.SharePod) (*corev1.ResourceRequirements, error) {
	resources := &corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}
	if sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory])
		if err != nil {
			return resources, err
		}
		resources.Limits[faasv1.KubeShareResourceGPULimit] = qty
	}
	if sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit])
		if err != nil {
			return resources, err
		}
		resources.GPULimit = qty
	}

	// Set CPU limits
	if sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest])
		if err != nil {
			return resources, err
		}
		resources.GPURequest = qty
	}
	return resources, nil

}
*/
