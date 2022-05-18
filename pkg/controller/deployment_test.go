package controller

import (
	"testing"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	"github.com/Interstellarss/faas-share/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	//KubeshareV1 "github"
)

func Test_newDeployment(t *testing.T) {
	sharepod := &faasv1.SharePod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubesec",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "kubesec",
					Image: "docker.io/kubesec/kubesec",
					//SecurityContext: ,
					//ReadinessProbe: &corev1.Probe{},
				},
			},
			/*
				Name:                   "kubesec",
				Image:                  "docker.io/kubesec/kubesec",
				Annotations:            &map[string]string{},
				ReadOnlyRootFilesystem: true,
			*/
		},
	}
	k8sConfig := k8s.DeploymentConfig{
		HTTPProbe:      true,
		SetNonRootUser: true,
		LivenessProbe: &k8s.ProbeConfig{
			PeriodSeconds:       1,
			TimeoutSeconds:      3,
			InitialDelaySeconds: 0,
		},
		ReadinessProbe: &k8s.ProbeConfig{
			PeriodSeconds:       1,
			TimeoutSeconds:      3,
			InitialDelaySeconds: 0,
		},
	}
	factory := NewFunctionFactory(fake.NewSimpleClientset(), k8sConfig)

	//secrets := map[string]*corev1.Secret{}

	deployment := newDeployment(sharepod, nil, factory)

	if deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.Path != "/_/health" {
		t.Errorf("Readiness probe should have HTTPGet handler set to %s", "/_/health")
		t.Fail()
	}

	if deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds != 0 {
		t.Errorf("Liveness probe should have initial delay seconds set to %s", "0")
		t.Fail()
	}
	/*
			if !*(deployment.Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem) {
				t.Errorf("ReadOnlyRootFilesystem should be true")
				t.Fail()
			}


		if *(deployment.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser) != k8s.SecurityContextUserID {
			t.Errorf("RunAsUser should be %v", k8s.SecurityContextUserID)
			t.Fail()
		}
	*/
}

func Test_newDeployment_withExecProbe(t *testing.T) {
	sharepod := &faasv1.SharePod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "kubesec",
			Annotations: map[string]string{},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Image: "docker.io/kubesec/kubesec",
				},
			},
			//ReadOnlyRootFilesystem: true,
			//Name:                   "kubesec",
			//Image:                  "docker.io/kubesec/kubesec",
			//Annotations:            &map[string]string{},
			//ReadOnlyRootFilesystem: true,
		},
	}
	k8sConfig := k8s.DeploymentConfig{
		HTTPProbe:      false,
		SetNonRootUser: true,
		LivenessProbe: &k8s.ProbeConfig{
			PeriodSeconds:       1,
			TimeoutSeconds:      3,
			InitialDelaySeconds: 0,
		},
		ReadinessProbe: &k8s.ProbeConfig{
			PeriodSeconds:       1,
			TimeoutSeconds:      3,
			InitialDelaySeconds: 0,
		},
	}

	factory := NewFunctionFactory(fake.NewSimpleClientset(), k8sConfig)

	//secrets := map[string]*corev1.Secret{}

	deployment := newDeployment(sharepod, nil, factory)

	if deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet != nil {
		t.Fatalf("ReadinessProbe's HTTPGet should be nil due to exec probe")
	}
}

func Test_newDeployment_PrometheusScrape_NotOverridden(t *testing.T) {
	function := &faasv1.SharePod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubesec",
			Annotations: map[string]string{
				"prometheus.io.scrape": "true",
			},
		},
		Spec: corev1.PodSpec{
			//Name:  "kubesec",
			Containers: []corev1.Container{
				{
					//Name: "kubespec",
					Image: "docker.io/kubesec/kubesec",
				},
			},
		},
	}

	factory := NewFunctionFactory(fake.NewSimpleClientset(),
		k8s.DeploymentConfig{
			HTTPProbe:      false,
			SetNonRootUser: true,
			LivenessProbe: &k8s.ProbeConfig{
				PeriodSeconds:       1,
				TimeoutSeconds:      3,
				InitialDelaySeconds: 0,
			},
			ReadinessProbe: &k8s.ProbeConfig{
				PeriodSeconds:       1,
				TimeoutSeconds:      3,
				InitialDelaySeconds: 0,
			},
		})

	//secrets := map[string]*corev1.Secret{}

	deployment := newDeployment(function, nil, factory)

	want := "true"

	if deployment.Spec.Template.Annotations["prometheus.io.scrape"] != want {
		t.Errorf("Annotation prometheus.io.scrape should be %s, was: %s", want, deployment.Spec.Template.Annotations["prometheus.io.scrape"])
	}
}
