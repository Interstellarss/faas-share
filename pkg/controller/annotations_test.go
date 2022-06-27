package controller

import (
	"testing"

	faasv1 "github.com/Interstellarss/faas-share/pkg/apis/faas_share/v1"

	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeAnnotations_NoKeys(t *testing.T) {
	annotationVal := `{"containers":[{"name":"kubesec","image":"docker.io/kubesec/kubesec","resources":{}}]}`

	spec := faasv1.SharePod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Annotations: map[string]string{"kubeshare/gpu_request": "0.5"},
		},
		Spec: faasv1.SharePodSpec{
			PodSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "kubesec",
						Image: "docker.io/kubesec/kubesec",
					},
				},
			},
		},
	}

	annotations := makeAnnotations(&spec)

	if _, ok := annotations["prometheus.io.scrape"]; !ok {
		t.Errorf("wanted annotation " + "prometheus.io.scrape" + " to be added")
		t.Fail()
	}
	if val, _ := annotations["prometheus.io.scrape"]; val != "false" {
		t.Errorf("wanted annotation " + "prometheus.io.scrape" + ` to equal "false"`)
		t.Fail()
	}

	if _, ok := annotations[annotationFunctionSpec]; !ok {
		t.Errorf("wanted annotation " + annotationFunctionSpec)
		t.Fail()
	}

	if val, _ := annotations[annotationFunctionSpec]; val != annotationVal {
		t.Errorf("Annotation " + annotationFunctionSpec + "\nwant: '" + annotationVal + "'\ngot: '" + val + "'")
		t.Fail()
	}
}

func Test_makeAnnotations_WithKeyAndValue(t *testing.T) {
	//annotationVal := `{"annotations":{"key":"value","key2":"value2"}}`

	spec := faasv1.SharePod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"key":  "value",
				"key2": "value2",
			},
		},
		Spec: faasv1.SharePodSpec{},
	}

	annotations := makeAnnotations(&spec)

	if _, ok := annotations["prometheus.io.scrape"]; !ok {
		t.Errorf("wanted annotation " + "prometheus.io.scrape" + " to be added")
		t.Fail()
	}
	if val, _ := annotations["prometheus.io.scrape"]; val != "false" {
		t.Errorf("wanted annotation " + "prometheus.io.scrape" + ` to equal "false"`)
		t.Fail()
	}

	if val, _ := annotations["key2"]; val != "value2" {
		t.Errorf("Annotation " + "of key2" + "\nwant: '" + "value2" + "'\ngot: '" + val + "'")
		t.Fail()
	}

	if val, _ := annotations["key"]; val != "value" {
		t.Errorf("Annotation " + "of key" + "\nwant: '" + "value" + "'\ngot: '" + val + "'")
		t.Fail()
	}
}

/*
func Test_makeAnnotationsDoesNotModifyOriginalSpec(t *testing.T) {
	specAnnotations := map[string]string{
		"test.foo": "bar",
	}
	function := &faasv1.Function{
		Spec: faasv1.FunctionSpec{
			Name:        "testfunc",
			Annotations: &specAnnotations,
		},
	}

	expectedAnnotations := map[string]string{
		"prometheus.io.scrape": "false",
		"test.foo":             "bar",
		annotationFunctionSpec: `{"name":"testfunc","image":"","annotations":{"test.foo":"bar"},"readOnlyRootFilesystem":false}`,
	}

	makeAnnotations(function)
	annotations := makeAnnotations(function)

	if len(specAnnotations) != 1 {
		t.Errorf("length of original spec annotations has changed, expected 1, got %d", len(specAnnotations))
	}

	if specAnnotations["test.foo"] != "bar" {
		t.Errorf("original spec annotation has changed")
	}

	for name, expectedValue := range expectedAnnotations {
		actualValue := annotations[name]
		if actualValue != expectedValue {
			t.Fatalf("incorrect annotation for '%s': \nwant %q,\ngot %q", name, expectedValue, actualValue)
		}
	}
}
*/
