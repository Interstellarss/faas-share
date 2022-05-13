package server

import (
	//"go/types"
	"io/ioutil"
	"net/http"

	"encoding/json"

	"github.com/gorilla/mux"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/klog"

	"github.com/openfaas/faas-provider/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func makeReplicaReader(defaultNamespace string, client clientset.Interface, lister v1.DeploymentLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		sharepodName := vars["name"]

		q := r.URL.Query()
		namespace := q.Get("namespace")

		lookupNamespace := defaultNamespace

		if len(namespace) > 0 {
			lookupNamespace = namespace
		}

		opts := metav1.GetOptions{}

		k8sfunc, err := client.KubeshareV1().SharePods(lookupNamespace).
			Get(r.Context(), sharepodName, opts)

		if err != nil || k8sfunc == nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(err.Error()))
			return
		}
	}
}

func getReplicas(sharepodName string, namespace string, lister v1.DeploymentLister) (uint64, uint64, error) {
	dep, err := lister.Deployments(namespace).Get(sharepodName)
	if err != nil {
		return 0, 0, err
	}

	desiredReplicas := uint64(dep.Status.Replicas)
	availableReplicas := uint64(dep.Status.AvailableReplicas)

	return desiredReplicas, availableReplicas, nil
}

func makeReplicaHandler(defaultNamespace string, kube kubernetes.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		sharepodName := vars["name"]

		q := r.URL.Query()
		namespace := q.Get("namespace")

		lookupNamespace := defaultNamespace

		if len(namespace) > 0 {
			lookupNamespace = namespace
		}

		if lookupNamespace == "kube-system" {
			http.Error(w, "unable to list within the kube-system namespace", http.StatusUnauthorized)
			return
		}

		req := types.ScaleServiceRequest{}

		if r.Body != nil {
			defer r.Body.Close()
			bytesIn, _ := ioutil.ReadAll(r.Body)

			if err := json.Unmarshal(bytesIn, &req); err != nil {
				klog.Errorf("Function %s replica invalid JSON: %v", sharepodName, err)
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
		}

		opts := metav1.GetOptions{}

		dep, err := kube.AppsV1().Deployments(lookupNamespace).Get(r.Context(), sharepodName, opts)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			klog.Errorf("Sharepod %s get error: %v", sharepodName, err)
			return
		}

		dep.Spec.Replicas = int32p(int32(req.Replicas))

		_, err = kube.AppsV1().Deployments(lookupNamespace).Update(r.Context(), dep, metav1.UpdateOptions{})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			klog.Errorf("Sharepod %s update error %v", sharepodName, err)
			return
		}

		klog.Infof("Sharepod %s replica updated to %v", sharepodName, req.Replicas)
		w.WriteHeader(http.StatusAccepted)
	}
}
