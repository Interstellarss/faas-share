package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sharepods "github.com/Interstellarss/faas-share/pkg/sharepod"
	corev1 "k8s.io/api/core/v1"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_makeApplyHandler(t *testing.T) {
	namsespace := "faas-share-fn"

	createVal := "test1"
	updateVal := "test2"

	fn := sharepods.SharepodDeployment{
		Service: "nodeinfo",
		Containers: []corev1.Container{
			{
				Name:  "info",
				Image: "function/nodeinfo",
			},
		},
		Labels: &map[string]string{
			"test": createVal,
		},
		//ReadOnlyRootFilesystem: true,
	}

	kube := clientset.NewSimpleClientset()
	applyHandler := makeApplyHandler(namsespace, kube).ServeHTTP

	fnJson, _ := json.Marshal(fn)

	req := httptest.NewRequest("POST", "http://system/sharepods", bytes.NewBuffer(fnJson))

	w := httptest.NewRecorder()

	applyHandler(w, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status code '%d', got '%d'", http.StatusAccepted, resp.StatusCode)
	}

	newSharepod, err := kube.KubeshareV1().SharePods(namsespace).Get(context.TODO(), fn.Service, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("error validating sharepod: %v", err)
	}

	if newSharepod.Labels == nil || len(newSharepod.Labels) < 1 {
		t.Fatal("expected one label got node")
	}

	newLabels := newSharepod.Labels

	if newLabels["test"] != createVal {
		t.Errorf("expected label '%s' got: '%s'", createVal, newLabels["test"])
	}

	//test update fn
	fn.Containers[0].Image += ":v1.0"

	//problem here, updated, but not success
	fn.Labels = &map[string]string{
		"test": updateVal,
	}

	updatedJson, _ := json.Marshal(fn)

	reqUp := httptest.NewRequest("POST", "http://system/sharepods", bytes.NewBuffer(updatedJson))
	wUp := httptest.NewRecorder()

	applyHandler(wUp, reqUp)

	respUp := wUp.Result()
	if respUp.StatusCode != http.StatusAccepted {
		t.Errorf("expected status code '%d', got '%d'", http.StatusAccepted, respUp.StatusCode)
	}

	updatedSharepod, err := kube.KubeshareV1().SharePods(namsespace).Get(context.TODO(), fn.Service, metav1.GetOptions{})
	if err != nil {
		t.Errorf("error validating sharepod: %v", err)
	}
	if updatedSharepod.Spec.PodSpec.Containers[0].Image != fn.Containers[0].Image {
		t.Errorf("expected image '%s' got: '%s'", fn.Containers[0].Image, updatedSharepod.Spec.PodSpec.Containers[0].Image)
	}

	updatedLabels := updatedSharepod.Labels
	if updatedLabels["test"] != updateVal {
		t.Errorf("expected label '%s' got: '%s'", updateVal, updatedLabels["test"])
	}
}
