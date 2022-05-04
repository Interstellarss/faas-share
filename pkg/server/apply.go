package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	"github.com/Interstellarss/faas-share/pkg/sharepod"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	kubesharev1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
)

func makeApplyHandler(defaultNamespace string, client clientset.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body != nil {
			defer r.Body.Close()
		}

		body, _ := ioutil.ReadAll(r.Body)

		req := sharepod.SharepodDeployment{}

		if err := json.Unmarshal(body, &req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		klog.Infof("Deployment request for : %s\n", req.Service)

		namespace := defaultNamespace
		if len(req.Namespace) > 0 {
			namespace = req.Namespace
		}

		opts := metav1.GetOptions{}
		got, err := client.KubeshareV1().SharePods(namespace).Get(r.Context(), req.Service, opts)
		miss := false

		if err != nil {
			if errors.IsNotFound(err) {
				miss = true
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}
		}

		if miss == false && got != nil {
			updated := got.DeepCopy()

			klog.Infof("Updating %s/n", updated.ObjectMeta.Name)

			updated.Spec = toSharepodSpec(req)

			if _, err = client.KubeshareV1().SharePods(namespace).
				Update(r.Context(), updated, metav1.UpdateOptions{}); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Error updating sharepod : %s", err.Error())))
				return
			}
		} else {

			//TODO: finishing the sharepod deployment
			newSharePod := &kubesharev1.SharePod{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       toSharepodSpec(req),
			}

			if _, err = client.KubeshareV1().SharePods(namespace).
				Create(r.Context(), newSharePod, metav1.CreateOptions{}); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("Error creating sharepod: %s", err.Error())))
				return
			}
		}
	}
}

func toSharepodSpec(req sharepod.SharepodDeployment) corev1.PodSpec {
	spec := corev1.PodSpec{}

	return spec
}

//how to deal with resources in sharepod
//func getResources(limits *sharepod.SharepodResources) *
