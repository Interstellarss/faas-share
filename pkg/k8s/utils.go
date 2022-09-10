// Copyright 2020 OpenFaaS Authors
// Licensed under the MIT license. See LICENSE file in the project root for full license information.

package k8s

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	glog "k8s.io/klog"
	"math/rand"
	"time"
)

// removeVolume returns a Volume slice with any volumes matching volumeName removed.
// Uses the filter without allocation technique
// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
func removeVolume(volumeName string, volumes []corev1.Volume) []corev1.Volume {
	if volumes == nil {
		return []corev1.Volume{}
	}

	newVolumes := volumes[:0]
	for _, v := range volumes {
		if v.Name != volumeName {
			newVolumes = append(newVolumes, v)
		}
	}

	return newVolumes
}

// removeVolumeMount returns a VolumeMount slice with any mounts matching volumeName removed
// Uses the filter without allocation technique
// https://github.com/golang/go/wiki/SliceTricks#filtering-without-allocating
func removeVolumeMount(volumeName string, mounts []corev1.VolumeMount) []corev1.VolumeMount {
	if mounts == nil {
		return []corev1.VolumeMount{}
	}

	newMounts := mounts[:0]
	for _, v := range mounts {
		if v.Name != volumeName {
			newMounts = append(newMounts, v)
		}
	}

	return newMounts
}

func FilterActivePods(pods []*corev1.Pod) []*corev1.Pod {
	var result []*corev1.Pod
	for _, p := range pods {
		if IsPodActive(p) {
			result = append(result, p)
		} else {
			glog.V(4).Infof("Ignoring inactive pod %v/%v in state %v, deletion time %v", p.Namespace, p.Name, p.Status.Phase, p.DeletionTimestamp)
		}
	}
	return result
}

func IsPodActive(p *corev1.Pod) bool {
	re := corev1.PodSucceeded != p.Status.Phase && corev1.PodFailed != p.Status.Phase && p.DeletionTimestamp == nil
	if len(p.Status.ContainerStatuses) > 0 {
		re = re && p.Status.ContainerStatuses[0].Ready
	} else {
		re = false
	}

	return re
}

func GenerateRangeNum(min, max int) int {
	rand.Seed(time.Now().Unix())
	randNum := rand.Intn(max - min)
	randNum = randNum + min
	fmt.Printf("rand is %v\n", randNum)
	return randNum
}
