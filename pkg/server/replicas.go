package server

import (
	"net/http"

	"github.com/gorilla/mux"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	v1 "k8s.io/client-go/listers/apps/v1"

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
