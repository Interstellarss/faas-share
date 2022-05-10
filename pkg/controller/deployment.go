package controller

import (
	"context"
	"encoding/json"
	"strings"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	"github.com/Interstellarss/faas-share/pkg/k8s"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	glog "k8s.io/klog"
)

const (
	annotationFunctionSpec = "com.openfaas.function.spec"
)

// newDeployment creates a new Deployment for a Function resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Function resource that 'owns' it.
func newDeployment(
	sharepod *faasv1.SharePod,
	existingDeployment *appsv1.Deployment,
	existingSecrets map[string]*corev1.Secret,
	factory FunctionFactory) *appsv1.Deployment {

	ctx := context.TODO()
	envVars := makeEnvVars(sharepod)
	labels := makeLabels(sharepod)
	//KubeShare does not support NodeSelector, so we dont need it here as well
	//nodeSelector := makeNodeSelector(sharepod.)
	probes, err := factory.MakeProbes(sharepod)
	if err != nil {
		glog.Warningf("Function %s probes parsing failed: %v",
			sharepod.Name, err)
	}

	resources, err := makeResources(sharepod)
	if err != nil {
		glog.Warningf("Function %s resources parsing failed: %v",
			sharepod.Name, err)
	}

	annotations := makeAnnotations(sharepod)

	allowPrivilegeEscalation := false

	deploymentSpec := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        sharepod.Name,
			Annotations: annotations,
			Namespace:   sharepod.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(sharepod, schema.GroupVersionKind{
					Group:   faasv1.SchemeGroupVersion.Group,
					Version: faasv1.SchemeGroupVersion.Version,
					Kind:    faasKind,
				}),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: getReplicas(sharepod, existingDeployment),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(0),
					},
					MaxSurge: &intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: int32(1),
					},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        sharepod.Name,
					"controller": sharepod.Name,
				},
			},
			RevisionHistoryLimit: int32p(5),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					//NodeSelector: nodeSelector,
					Containers: []corev1.Container{
						{
							Name: sharepod.Name,
							//need to specify Image here, refer to KubeShares
							Image: sharepod.Spec["Image"],
							Ports: []corev1.ContainerPort{
								{ContainerPort: int32(functionPort), Protocol: corev1.ProtocolTCP},
							},
							ImagePullPolicy: corev1.PullPolicy(factory.Factory.Config.ImagePullPolicy),
							Env:             envVars,
							Resources:       *resources,
							LivenessProbe:   probes.Liveness,
							ReadinessProbe:  probes.Readiness,
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: &allowPrivilegeEscalation,
							},
						},
					},
				},
			},
		},
	}

	factory.ConfigureReadOnlyRootFilesystem(sharepod, deploymentSpec)
	factory.ConfigureContainerUserID(deploymentSpec)

	var currentAnnotations map[string]string
	if existingDeployment != nil {
		currentAnnotations = existingDeployment.Annotations
	}

	// compare the annotations from args to the cache copy of the deployment annotations
	// at this point we have already updated the annotations to the new value, if we
	// compare to that it will produce an empty list
	profileNamespace := factory.Factory.Config.ProfilesNamespace
	profileList, err := factory.GetProfilesToRemove(ctx, profileNamespace, annotations, currentAnnotations)
	if err != nil {
		// TODO: a simple warning doesn't seem strong enough if a profile can't be found or there is
		// some other error
		glog.Warningf("Function %s can not retrieve required Profiles in %s: %v", sharepod.Spec.Name, profileNamespace, err)
	}
	for _, profile := range profileList {
		factory.RemoveProfile(profile, deploymentSpec)
	}

	if _, exists := annotations[k8s.ProfileAnnotationKey]; !exists {
		glog.Infof("Function %s: no profiles specified", sharepod.Name)
	}

	profileList, err = factory.GetProfiles(ctx, profileNamespace, annotations)
	if err != nil {
		// TODO: a simple warning doesn't seem strong enough if a profile can't be found or there is
		// some other error
		glog.Warningf("Function %s can not retrieve required Profiles in %s: %v", sharepod.Name, profileNamespace, err)
	}
	// TODO: remove this or refactor to just print names
	glog.Infof("Function %s: Applying profiles %+v", sharepod.Name, profileList)
	for _, profile := range profileList {
		factory.ApplyProfile(profile, deploymentSpec)
	}

	if err := UpdateSecrets(sharepod, deploymentSpec, existingSecrets); err != nil {
		// TODO: a simple warning doesn't seem strong enough if we can't update the secrets
		glog.Warningf("Function %s secrets update failed: %v",
			sharepod.Name, err)
	}

	return deploymentSpec
}

func makeEnvVars(function *faasv1.SharePod) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	if len(function.Spec.Handler) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "fprocess",
			Value: function.Spec.Handler,
		})
	}

	if function.Spec.Environment != nil {
		for k, v := range *function.Spec.Environment {
			envVars = append(envVars, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}

	return envVars
}

func makeLabels(sharepod *faasv1.SharePod) map[string]string {
	labels := map[string]string{
		"faas_function": sharepod.Name,
		"app":           sharepod.Name,
		"controller":    sharepod.Name,
	}
	if sharepod.Labels != nil {
		for k, v := range *&sharepod.Labels {
			labels[k] = v
		}
	}

	return labels
}

func makeAnnotations(sharepod *faasv1.SharePod) map[string]string {
	annotations := make(map[string]string)

	// disable scraping since the watchdog doesn't expose a metrics endpoint
	annotations["prometheus.io.scrape"] = "false"

	// copy function annotations
	if sharepod.Annotations != nil {
		for k, v := range *&sharepod.Annotations {
			annotations[k] = v
		}
	}

	// save function spec in deployment annotations
	// used to detect changes in function spec
	specJSON, err := json.Marshal(sharepod.Spec)
	if err != nil {
		glog.Errorf("Failed to marshal function spec: %s", err.Error())
		return annotations
	}

	annotations[annotationFunctionSpec] = string(specJSON)
	return annotations
}

func makeNodeSelector(constraints []string) map[string]string {
	selector := make(map[string]string)

	if len(constraints) > 0 {
		for _, constraint := range constraints {
			parts := strings.Split(constraint, "=")

			if len(parts) == 2 {
				selector[parts[0]] = parts[1]
			}
		}
	}

	return selector
}

// deploymentNeedsUpdate determines if the function spec is different from the deployment spec
func deploymentNeedsUpdate(sharepod *faasv1.SharePod, deployment *appsv1.Deployment) bool {
	prevFnSpecJson := deployment.ObjectMeta.Annotations[annotationFunctionSpec]
	if prevFnSpecJson == "" {
		// is a new deployment or is an old deployment that is missing the annotation
		return true
	}

	prevFnSpec := &corev1.PodSpec{}
	err := json.Unmarshal([]byte(prevFnSpecJson), prevFnSpec)
	if err != nil {
		glog.Errorf("Failed to parse previous function spec: %s", err.Error())
		return true
	}
	prevFn := faasv1.SharePod{
		Spec: *prevFnSpec,
	}

	if diff := cmp.Diff(prevFn.Spec, sharepod.Spec); diff != "" {
		glog.V(2).Infof("Change detected for %s diff\n%s", sharepod.Name, diff)
		return true
	} else {
		glog.V(3).Infof("No changes detected for %s", sharepod.Name)
	}

	return false
}

func int32p(i int32) *int32 {
	return &i
}
