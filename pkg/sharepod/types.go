package sharepod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
)

type Sharepod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec SharepodSpec `json:"spec"`
}

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
	Containers Container `json:"containers,omitempty"`

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
	Environment *map[string]string `json:"environment,omitempty"`
	// +optional
	Constraints []string `json:"constraints,omitempty"`
	// +optional
	Secrets []string `json:"secrets,omitempty"`
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

// FunctionList is a list of Sharepod resources
//this is already given in hte kubeshare packages

/*
type SharepodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Sharepod `json:"items"`
}
*/

type Container struct {
	Name    string   `jsom:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command, omitempty"`
}

//Proffile may noe be needed

// Profile and ProfileSpec are used to customise the Pod template for
// sahrepods
type Profile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ProfileSpec `json:"spec"`
}

// ProfileSpec is an openfaas api extensions that can be predefined and applied
// to functions by annotating them with `com.openfaas/profile: name1,name2`
type ProfileSpec struct {
	// If specified, the function's pod tolerations.
	//
	// merged into the Pod Tolerations
	//
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// RuntimeClassName refers to a RuntimeClass object in the node.k8s.io group, which should be used
	// to run this pod.  If no RuntimeClass resource matches the named class, the pod will not be run.
	// If unset or empty, the "legacy" RuntimeClass will be used, which is an implicit class with an
	// empty definition that uses the default runtime handler.
	// More info: https://git.k8s.io/enhancements/keps/sig-node/runtime-class.md
	// This is a beta feature as of Kubernetes v1.14.
	//
	// copied to the Pod RunTimeClass, this will replace any existing value or previously
	// applied Profile.
	//
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// If specified, the pod's scheduling constraints
	//
	// copied to the Pod Affinity, this will replace any existing value or previously
	// applied Profile. We use a replacement strategy because it is not clear that merging
	// affinities will actually produce a meaning Affinity definition, it would likely result in
	// an impossible to satisfy constraint
	//
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// SecurityContext holds pod-level security attributes and common container settings.
	// Optional: Defaults to empty.  See type description for default values of each field.
	//
	// each non-nil value will be merged into the function's PodSecurityContext, the value will
	// replace any existing value or previously applied Profile
	//
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProfileList is a list of Profiles
type ProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Profile `json:"items"`
}

type SharepodRequirements struct {
	//Limits
	GPULimit   resource.Quantity `json:"gpuLimit"`
	GPURequest resource.Quantity `json:"gpuRequest"`
	Memory     resource.Quantity `json:"memory"`
}
