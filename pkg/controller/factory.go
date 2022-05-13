package controller

import (
	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	"github.com/Interstellarss/faas-share/pkg/k8s"
	"github.com/Interstellarss/faas-share/pkg/sharepod"
	appsv1 "k8s.io/api/apps/v1"

	"k8s.io/client-go/kubernetes"
)

// FunctionFactory wraps faas-netes factory
type FunctionFactory struct {
	Factory k8s.FunctionFactory
}

func NewFunctionFactory(clientset kubernetes.Interface, config k8s.DeploymentConfig) FunctionFactory {
	return FunctionFactory{
		k8s.FunctionFactory{
			Client: clientset,
			Config: config,
		},
	}
}

func sharepodToSharepodRequest(in *faasv1.SharePod) sharepod.SharepodDeployment {
	/*
		env := make(map[string]string)
		if in.Spec.Containers[0].Env != nil {
			env = *in.Spec.Env
		}
	*/
	res := sharepodToSharepodResources(in)
	return sharepod.SharepodDeployment{
		Annotations: in.Annotations,
		Service:     in.Name,
		Labels:      &in.Labels,
		//Constraints:            in.Spec.Constraints,
		//EnvProcess:             in.Spec.Handler,
		Resources: res,
		Namespace: in.Namespace,
		//Image:                  in.Spec.Image,
		//Limits:                 lim,
		//Requests:               req,
		//ReadOnlyRootFilesystem: in.ReadOnlyRootFilesystem,
	}
}

func sharepodToSharepodResources(in *faasv1.SharePod) (re *sharepod.SharepodResources) {
	if in.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit] != "" || in.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest] != "" ||
		in.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory] != "" {
		gpu_limit := in.ObjectMeta.Annotations[faasv1.KubeShareResourceGPULimit]
		gpu_request := in.ObjectMeta.Annotations[faasv1.KubeShareResourceGPURequest]
		gpu_mem := in.ObjectMeta.Annotations[faasv1.KubeShareResourceGPUMemory]

		re = &sharepod.SharepodResources{
			GPULimit:   gpu_limit,
			GPURequest: gpu_request,
			Memory:     gpu_mem,
		}
	}
	return
}

func (f *FunctionFactory) MakeProbes(sharepod *faasv1.SharePod) (*k8s.SharepodProbes, error) {
	req := sharepodToSharepodRequest(sharepod)
	return f.Factory.MakeProbes(req)
}

func (f *FunctionFactory) ConfigureReadOnlyRootFilesystem(sharepod *faasv1.SharePod, deployment *appsv1.Deployment) {
	req := sharepodToSharepodRequest(sharepod)
	f.Factory.ConfigureReadOnlyRootFilesystem(req, deployment)
}

func (f *FunctionFactory) ConfigureContainerUserID(deployment *appsv1.Deployment) {
	f.Factory.ConfigureContainerUserID(deployment)
}

/*
func (f *FunctionFactory) ApplyProfile(profile k8s.Profile, deployment *appsv1.Deployment) {
	f.Factory.ApplyProfile(profile, deployment)
}

func (f *FunctionFactory) RemoveProfile(profile k8s.Profile, deployment *appsv1.Deployment) {
	f.Factory.RemoveProfile(profile, deployment)
}

func (f *FunctionFactory) GetProfiles(ctx context.Context, namespace string, annotations map[string]string) ([]k8s.Profile, error) {
	return f.Factory.GetProfiles(ctx, namespace, annotations)
}

func (f *FunctionFactory) GetProfilesToRemove(ctx context.Context, namespace string, annotations, currentAnnotations map[string]string) ([]k8s.Profile, error) {
	return f.Factory.GetProfilesToRemove(ctx, namespace, annotations, currentAnnotations)
}
*/
