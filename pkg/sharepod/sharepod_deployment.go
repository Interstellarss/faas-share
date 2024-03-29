package sharepod

import (
	corev1 "k8s.io/api/core/v1"
)

//TODO: not yet finalized, still need to check its validation
type SharepodDeployment struct {

	// Service is the name of the function deployment
	Service string `json:"service"`

	Containers []corev1.Container `json:"containers"`

	// Namespace for the function, if supported by the faas-provider
	Namespace string `json:"namespace,omitempty"`

	// EnvProcess overrides the fprocess environment variable and can be used
	// with the watchdog
	//not sure about this now
	EnvProcess string `json:"envProcess,omitempty"`

	// EnvVars can be provided to set environment variables for the function runtime.
	//EnvVars map[string]string `json:"envVars,omitempty"`

	// Constraints are specific to the faas-provider.
	Constraints []string `json:"constraints,omitempty"`

	// Secrets list of secrets to be made available to function
	//Secrets []string `json:"secrets,omitempty"`

	// Labels are metadata for functions which may be used by the
	// faas-provider or the gateway
	Labels *map[string]string `json:"labels,omitempty"`

	// Annotations are metadata for functions which may be used by the
	// faas-provider or the gateway
	Annotations map[string]string `json:"annotations,omitempty"`

	/*
		// Limits for function
		GPULimits *SharepodResources `json:"limits,omitempty"`

		// Requests of resources requested by function
		GPURequests *SharepodResources `json:"requests,omitempty"`
	*/

	Resources *SharepodResources `json:"resources, omitempty"`

	// ReadOnlyRootFilesystem removes write-access from the root filesystem
	// mount-point.
	ReadOnlyRootFilesystem bool `json:"readOnlyRootFilesystem,omitempty"`
}

//SharepodResources may not need to be decaared here
//type SharepodResources struct {
//}
