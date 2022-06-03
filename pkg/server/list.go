package server

import (
	"encoding/json"
	"net/http"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	appsv1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	KubeshareV1 "github.com/Interstellarss/faas-share/pkg/apis/kubeshare/v1"
)

func makeListHandler(defaultNamespace string,
	client clientset.Interface,
	deploymentLister appsv1.DeploymentLister) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		if r.Body != nil {
			defer r.Body.Close()
		}

		q := r.URL.Query()

		namespace := q.Get("namespace")

		lookupNamespace := defaultNamespace

		if len(namespace) > 0 {
			lookupNamespace = namespace
		}

		if lookupNamespace == "kube-system" {
			http.Error(w, "unable to list within the kube-system", http.StatusUnauthorized)
			return
		}

		opts := metav1.ListOptions{}
		res, err := client.KubeshareV1().SharePods(lookupNamespace).List(r.Context(), opts)
		//todo here
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			klog.Errorf("Sharepod listing error: %v", err)
		}

		sharepods := []KubeshareV1.SharePodStatus{}

		for _, item := range res.Items {
			desiredReplicas, availableReplicas, err := getReplicas(item.Name, lookupNamespace, deploymentLister)

			if err != nil {
				klog.Warningf("Function listing getReplicas error: %v", err)
			}

			sharepod := item.Status
			sharepod.AvailableReplicas = availableReplicas
			sharepod.Replicas = desiredReplicas

			sharepods = append(sharepods, sharepod)
			//sharepod.PodStatus.
		}

		sharepodBytes, err := json.Marshal(sharepods)
		if err != nil {
			klog.Errorf("Failed to marshal sharepods: %s", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Failed to marshal sharepods"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(sharepodBytes)
	}
}

/*
func toSharepodStatus(item KubeshareV1.SharePod) KubeshareV1.SharePodStatus {

		status  := KubeshareV1.SharePodStatus{
			PodStatus: item.Status.PodStatus,
			PodObjectMeta: &item.ObjectMeta,
			BoundDeviceID: item.ObjectMeta.Annotations[KubeshareV1.KubeShareResourceGPUID],

		}

}
*/
