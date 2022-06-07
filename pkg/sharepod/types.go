package sharepod

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	//KubeshareV1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
)

//TODO: as the objectMeta did, add suport for annomations as defined in kubeshare, or not
//since the annomations is supported inside metav1.ObjectMetas
//type SharepodMeta struct {
//}

type SharepodSpec struct {
	//Name string `json:"name"`
	//this is inside ObjectMeta
	TerminationGracePeriodSeconds int `json:"terminationGracePeriodSeconds, omitempty"`

	//Image string `json:"image"`
	//use Containers instead, regarding how it was defined in KubeShare
	Containers []corev1.Container `json:"containers,omitempty"`

	RestartPolicy string `json:"restartPolicy, omitempty"`

	// +optional
	Handler string `json:"handler,omitempty"`
	// +optional
	//TODO: should we put annotations here?
	//may not be needed in kubeshare
	//Annotations *map[string]string `json:"annotations,omitempty"`

	// +optional
	Labels *map[string]string `json:"labels,omitempty"`
	// +optional
	//Environment *map[string]string `json:"environment,omitempty"`
	// +optional
	Constraints []string `json:"constraints,omitempty"`
	// +optional
	//Secrets []string `json:"secrets,omitempty"`
	// +optional
	//Limits *SharepodResources `json:"limits,omitempty"`
	// +optional
	//Requests *SharepodResources `json:"requests,omitempty"`
	// +optional
	ReadOnlyRootFilesystem bool `json:"readOnlyRootFilesystem"`
}

//SharepodResources is used to set GPU and memory limits and requests
type SharepodResources struct {
	GPULimit   string `json:"gpuLimit"`
	GPURequest string `json:"gpuRequest"`
	Memory     string `json:"memory"`
}

/*
type SharepodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Sharepod `json:"items"`
}
*/

type SharepodStatus struct {
	// Name is the name of the function deployment
	Name string `json:"name"`
	// Namespace for the function, if supported by the faas-provider
	Namespace string `json:"namespace,omitempty"`

	// Labels are metadata for functions which may be used by the
	// faas-provider or the gateway
	Labels *map[string]string `json:"labels,omitempty"`

	Containers []corev1.Container `json:"containers,omitempty"`

	// Annotations are metadata for functions which may be used by the
	// faas-provider or the gateway
	Annotations *map[string]string `json:"annotations,omitempty"`

	// Limits for function
	Limits *SharepodResources `json:"limits,omitempty"`

	// Requests of resources requested by function
	Requests *SharepodResources `json:"requests,omitempty"`

	// InvocationCount count of invocations
	InvocationCount float64 `json:"invocationCount,omitempty"`

	// Replicas desired within the cluster
	Replicas uint64 `json:"replicas,omitempty"`

	// AvailableReplicas is the count of replicas ready to receive
	// invocations as reported by the faas-provider
	AvailableReplicas uint64 `json:"availableReplicas,omitempty"`

	// CreatedAt is the time read back from the faas backend's
	// data store for when the function or its container was created.
	CreatedAt time.Time `json:"createdAt,omitempty"`

	// Usage represents CPU and RAM used by all of the
	// functions' replicas. Divide by AvailableReplicas for an
	// average value per replica.
	Usage *SharepodUsage `json:"usage,omitempty"`
}

type SharepodUsage struct {
	GPU float64 `json:"gpu, omitempty"`

	TotalMemoryBytes float64 `json:"totalMemoryBytes, omitempty"`
}

type SharepodRequirements struct {
	//Limits
	GPULimit   resource.Quantity `json:"gpuLimit"`
	GPURequest resource.Quantity `json:"gpuRequest"`
	Memory     resource.Quantity `json:"memory"`
}
