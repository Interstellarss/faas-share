package scheduler

import (
	//kubesharev1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	corev1 "k8s.io/api/core/v1"
)

var filters = []func(NodeResources, *corev1.Pod){
	GPUAffinityFilter,
	GPUAntiAffinityFilter,
	GPUExclusionFilter,
}

func GPUAffinityFilter(nodeResources NodeResources, sharepod *corev1.Pod) {
	affinityTag := ""
	if val, ok := sharepod.ObjectMeta.Annotations[KubeShareScheduleAffinity]; ok {
		affinityTag = val
	} else {
		return
	}
	for _, nodeRes := range nodeResources {
		for GPUID, gpuInfo := range nodeRes.GpuFree {
			notFound := true
			for _, gpuAffinityTag := range gpuInfo.GPUAffinityTags {
				if affinityTag == gpuAffinityTag {
					notFound = false
					break
				}
			}
			if notFound {
				delete(nodeRes.GpuFree, GPUID)
			}
		}
	}
}

func GPUExclusionFilter(nodeResources NodeResources, sharepod *corev1.Pod) {
	exclusionTag := ""
	if val, ok := sharepod.ObjectMeta.Annotations[KubeShareScheduleExclusion]; ok {
		exclusionTag = val
	}
	for _, nodeRes := range nodeResources {
		for GPUID, gpuInfo := range nodeRes.GpuFree {
			if exclusionTag != "" && len(gpuInfo.GPUExclusionTags) == 0 {
				delete(nodeRes.GpuFree, GPUID)
				break
			}
			// len(gpuInfo.GPUExclusionTags) should be only one
			for _, gpuExclusionTag := range gpuInfo.GPUExclusionTags {
				if exclusionTag != gpuExclusionTag {
					delete(nodeRes.GpuFree, GPUID)
					break
				}
			}
		}
	}
}

func GPUAntiAffinityFilter(nodeResources NodeResources, sharepod *corev1.Pod) {
	antiAffinityTag := ""
	if val, ok := sharepod.ObjectMeta.Annotations[KubeShareScheduleAntiAffinity]; ok {
		antiAffinityTag = val
	} else {
		return
	}
	for _, nodeRes := range nodeResources {
		for GPUID, gpuInfo := range nodeRes.GpuFree {
			for _, gpuAntiAffinityTag := range gpuInfo.GPUAntiAffinityTags {
				if antiAffinityTag == gpuAntiAffinityTag {
					delete(nodeRes.GpuFree, GPUID)
					break
				}
			}
		}
	}
}
