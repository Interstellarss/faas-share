package controller

import (
	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	kubesharev1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"
)

// makeResources creates deployment resource limits and requests requirements from function specs
func makeResources(sharepod *kubesharev1.SharePod) (*corev1.ResourceRequirements, error) {
	resources := &corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	//sharepod.Status.

	// Set Memory limits
	if sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPUMemory] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory])
		if err != nil {
			return resources, err
		}
		resources.Limits[corev1.ResourceMemory] = qty
		resources.Requests[corev1.ResourceMemory] = qty
	}
	if sharepod.ObjectMeta.Annotations[kubesharev1.KubeShareResourceGPULimit] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit])
		if err != nil {
			return resources, err
		}
		resources.Limits[kubesharev1.ResourceNVIDIAGPU] = qty
	}

	// Set CPU limits
	if sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest] != "" {
		qty, err := resource.ParseQuantity(sharepod.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest])
		if err != nil {
			return resources, err
		}
		resources.Requests[kubesharev1.ResourceNVIDIAGPU] = qty
		resources.Limits[kubesharev1.ResourceNVIDIAGPU] = qty
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
