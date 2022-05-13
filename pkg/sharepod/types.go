package sharepod

import (
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

type SharepodRequirements struct {
	//Limits
	GPULimit   resource.Quantity `json:"gpuLimit"`
	GPURequest resource.Quantity `json:"gpuRequest"`
	Memory     resource.Quantity `json:"memory"`
}
