package controller

import (
	"encoding/json"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faasshare/v1"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	glog "k8s.io/klog"
)

const (
	annotationFunctionSpec = "faas-sahre.sharepod.spec"

	//KubeShareLibraryPath = "/faas-share/library"
	//PodManagerPosrtStart = 50050
)

// newDeployment creates a new Deployment for a Function resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Function resource that 'owns' it.
/*
func newDeployment(
	sharepod *faasv1.SharePod,
	existingDeployment *appsv1.Deployment,
	//existingSecrets map[string]*corev1.Secret,
	factory FunctionFactory) *appsv1.Deployment {

	//ctx := context.TODO()
	//envVars := makeEnvVars(sharepod)
	labels := makeLabels(sharepod)
	//KubeShare does not support NodeSelector, so we dont need it here as well
	//nodeSelector := makeNodeSelector(sharepod.)
	//probes, err := factory.MakeProbes(sharepod)
	//if err != nil {
	//	glog.Warningf("Function %s probes parsing failed: %v",
	//		sharepod.Name, err)
	//}

	//resources, err := makeResources(sharepod)
	//if err != nil {
	//	glog.Warningf("Function %s resources parsing failed: %v",
	//		sharepod.Name, err)
	//}

	annotations := makeAnnotations(sharepod)

	allowPrivilegeEscalation := false

	specCopy := sharepod.Spec.DeepCopy()

	containerSpec := specCopy.PodSpec. Containers

	for i := range containerSpec {
		c := &containerSpec[i]
		//c.LivenessProbe = probes.Liveness
		//c.ReadinessProbe = probes.Readiness
		c.SecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: &allowPrivilegeEscalation,
		}
	}

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

			RevisionHistoryLimit: int32p(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					/*
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(sharepod, schema.GroupVersionKind{
								Group:   faasv1.SchemeGroupVersion.Group,
								Version: faasv1.SchemeGroupVersion.Version,
								Kind:    "SharePod",
							}),
						},

					Namespace:   sharepod.Namespace,
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: *specCopy,
			},
		},
	}

	//factory.ConfigureReadOnlyRootFilesystem(sharepod, deploymentSpec)
	factory.ConfigureContainerUserID(deploymentSpec)


		var currentAnnotations map[string]string
		if existingDeployment != nil {
			currentAnnotations = existingDeployment.Annotations
		}


	return deploymentSpec
}

func makeEnvVars(sharepod *faasv1.SharePod) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	if len(sharepod.Spec.Handler) > 0 {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "fprocess",
			Value: sharepod.Spec.Handler,
		})
	}

	if sharepod.Spec.Environment != nil {
		for k, v := range *sharepod.Spec.Environment {
			envVars = append(envVars, corev1.EnvVar{
				Name:  k,
				Value: v,
			})
		}
	}

	return envVars
}
*/

func makeLabels(sharepod *faasv1.SharePod) map[string]string {
	labels := map[string]string{
		"faas_function": sharepod.Name,
		"app":           sharepod.Name,
		"controller":    sharepod.Name,
	}
	if sharepod.Labels != nil {
		for k, v := range sharepod.Labels {
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
		for k, v := range sharepod.Annotations {
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

//NodeSelector not supported in KubeShare
/*
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
*/

// deploymentNeedsUpdate determines if the function spec is different from the deployment spec
func sharepodNeedsUpdate(sharepod *faasv1.SharePod, deployment *appsv1.Deployment) bool {
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
		Spec: faasv1.SharePodSpec{
			PodSpec: *prevFnSpec,
		},
	}

	if diff := cmp.Diff(prevFn.Spec, sharepod.Spec); diff != "" {
		glog.V(2).Infof("Change detected for %s diff\n%s", sharepod.Name, diff)
		return true
	} else {
		glog.V(3).Infof("No changes detected for %s", sharepod.Name)
	}

	//TODo check replicas:

	return false
}

func int32p(i int32) *int32 {
	return &i
}

/*
func (c *Controller) removeSharepodFromList(sharepod *faasv1.SharePod) {
	namespace := sharepod.Namespace
	name := sharepod.Name
	//dep, err := c.deploymentsLister.Deployments(namespace).
	//err := c.kubeclientset.AppsV1().Deployments(namespace).Delete(context.TODO() name)
}
*/
