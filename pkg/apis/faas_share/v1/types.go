/*
Copyright 2015 The Kubernetes Authors.

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

package v1

import (
	"bytes"
	"math/rand"
	"time"
	"unsafe"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

const (
	letterIdxBits = 5                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	letterBytes   = "abcdefghijklmnopqrstuvwxyz"
	// letterIdxBits = 6                    // 6 bits to represent a letter index
	// letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	// letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	// letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// kubeshare constants
	KubeShareResourceGPURequest = "kubeshare/gpu_request"
	KubeShareResourceGPULimit   = "kubeshare/gpu_limit"
	KubeShareResourceGPUMemory  = "kubeshare/gpu_mem"
	KubeShareResourceGPUID      = "kubeshare/GPUID"
	KubeShareDummyPodName       = "kubeshare-vgpu"
	KubeShareNodeName           = "kubeshare/nodeName"
	KubeShareRole               = "kubeshare/role"
	KubeShareNodeGPUInfo        = "kubeshare/gpu_info"
	ResourceNVIDIAGPU           = "nvidia.com/gpu"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SharePod struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +optional
	Status SharePodStatus `json:"status,omitempty"`
	// +optional
	Spec SharePodSpec `json:"spec,omitempty"`
}

type SharePodSpec struct {

	// +optional
	PodSpec corev1.PodSpec `json:"spec,omitempty"`

	// Replicas is the number of desired replicas.
	// This is a pointer to distinguish between explicit zero and unspecified.
	// Defaults to 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller/#what-is-a-replicationcontroller
	// +optional
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`

	// Selector is a label query over pods that should match the replica count.
	// Label keys and values that must match in order to be controlled by this replica set.
	// It must match the pod template's labels.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector *metav1.LabelSelector `json:"selector" protobuf:"bytes,2,opt,name=selector"`
}

type SharePodStatus struct {
	/*PodPhase          corev1.PodPhase
	ConfigFilePhase   ConfigFilePhase
	BoundDeviceID     string
	StartTime         *metav1.Time
	ContainerStatuses []corev1.ContainerStatus*/
	//TODO,: make bounddeviceids to store array of ids, and a map of replicateed pod to pod status
	podlist map[string]*corev1.Pod `json:"podList, omitempty"`

	PodStatus *corev1.PodStatus

	//PodObjectMeta *metav1.ObjectMeta

	// readyReplicas is the number of pods targeted by this ReplicaSet with a Ready Condition.
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty" protobuf:"varint,4,opt,name=readyReplicas"`

	// The number of available replicas (ready for at least minReadySeconds) for this replica set.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty" protobuf:"varint,5,opt,name=availableReplicas"`

	// Replicas is the most recently oberved number of replicas.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller/#what-is-a-replicationcontroller
	Replicas int32 `json:"replicas" protobuf:"varint,1,opt,name=replicas"`

	//maping from pod 2 boundDeviceID
	// +optional
	BoundDeviceIDs *map[string]string `json:"boundDeviceIDs, omitempty"`

	//BoundDeviceID     string
	PodManagerPort int `json:"podManagerPort, omitempty"`

	//TODOs: add replicas spec for faas

	Usage *SharepodUsage `json:"usage, omitempty"`

	//TODO: adding contitions?
}

type SharepodUsage struct {
	GPU float64 `json:"gpu, omitempty"`

	TotalMemoryBytes float64 `json:"totalMemoryBytes, omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TestTypeList is a top-level list type. The client methods for lists are automatically created.
// You are not supposed to create a separated client for this one.
type SharePodList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []SharePod `json:"items"`
}

func (this SharePod) Print() {
	var buf bytes.Buffer
	buf.WriteString("\n================= SharePod ==================")
	buf.WriteString("\nname: ")
	buf.WriteString(this.ObjectMeta.Namespace)
	buf.WriteString("/")
	buf.WriteString(this.ObjectMeta.Name)
	buf.WriteString("\nannotation:\n\tkubeshare/gpu_request: ")
	buf.WriteString(this.ObjectMeta.Annotations["kubeshare/gpu_request"])
	if this.Status.PodStatus != nil {
		buf.WriteString("\nstatus:\n\tPodStatus: ")
		buf.WriteString(string(this.Status.PodStatus.Phase))
	}
	buf.WriteString("\n\tGPUID: ")
	buf.WriteString(this.ObjectMeta.Annotations["kubeshare/GPUID"])
	buf.WriteString("\n\tBoundDeviceIs: ")
	//buf.WriteByte(this.Status.pod2BoundDeviceID)
	buf.WriteString("\n=============================================")
	klog.Info(buf.String())
}

// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-go/31832326#31832326
var src = rand.NewSource(time.Now().UnixNano())

func NewGPUID(n int) string {
	b := make([]byte, n)
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return *(*string)(unsafe.Pointer(&b))
}
