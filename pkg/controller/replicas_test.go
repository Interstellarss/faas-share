package controller

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/fake"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
	"github.com/Interstellarss/faas-share/pkg/k8s"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Replicas(t *testing.T) {
	scenarios := []struct {
		name     string
		sharepod *faasv1.SharePod
		deploy   *appsv1.Deployment
		expected *int32
	}{
		{
			"return nil replicas when label is missing and deployment does not exist",
			&faasv1.SharePod{},
			nil,
			nil,
		},
		{
			"return nil replicas when label is missing and deployment has no replicas",
			&faasv1.SharePod{},
			&appsv1.Deployment{},
			nil,
		},
		{
			"return min replicas when label is present and deployment has nil replicas",
			&faasv1.SharePod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: nil}},
			int32p(2),
		},
		{
			"return min replicas when label is present and deployment has replicas less than min",
			&faasv1.SharePod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: int32p(1)}},
			int32p(2),
		},
		{
			"return existing replicas when label is present and deployment has more replicas than min",
			&faasv1.SharePod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: int32p(3)}},
			int32p(3),
		},
		{
			"return existing replicas when label is missing and deployment has replicas set by HPA",
			&faasv1.SharePod{ObjectMeta: metav1.ObjectMeta{}, Spec: corev1.PodSpec{}},
			&appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: int32p(3)}},
			int32p(3),
		},
		{
			"return zero replicas when label is present and deployment has zero replicas",
			&faasv1.SharePod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{LabelMinReplicas: "2"}}},
			&appsv1.Deployment{Spec: appsv1.DeploymentSpec{Replicas: int32p(0)}},
			int32p(0),
		},
	}

	/*
		factory := NewFunctionFactory(fake.NewSimpleClientset(),
			k8s.DeploymentConfig{
				LivenessProbe:  &k8s.ProbeConfig{},
				ReadinessProbe: &k8s.ProbeConfig{},
			})
	*/

	factory := NewFunctionFactory(fake.NewSimpleClientset(), k8s.DeploymentConfig{
		LivenessProbe:  &k8s.ProbeConfig{},
		ReadinessProbe: &k8s.ProbeConfig{},
	})
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			deploy := newDeployment(s.sharepod, s.deploy, factory)
			value := deploy.Spec.Replicas

			if s.expected != nil && value != nil {
				if *s.expected != *value {
					t.Errorf("incorrect replica count: expected %v, got %v", *s.expected, *value)
				}
			} else if s.expected != value {
				t.Errorf("incorrect replica count: expected %v, got %v", s.expected, value)
			}
		})
	}
}
