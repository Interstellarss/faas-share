package server

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	"github.com/openfaas/faas-provider/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/klog"
)

func makeDeleteHandler(defaultNamespace string, client clientset.Interface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		q := r.URL.Query()
		namespace := q.Get("namespace")

		lookupNamespace := defaultNamespace
		if len(namespace) > 0 {
			lookupNamespace = namespace
		}

		if namespace == "kube-system" {
			http.Error(w, "unable to list within the kube-system namespace", http.StatusUnauthorized)
			return
		}

		if r.Body != nil {
			defer r.Body.Close()
		}

		body, _ := ioutil.ReadAll(r.Body)
		request := types.DeleteFunctionRequest{}

		err := json.Unmarshal(body, &request)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}

		if len(request.FunctionName) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "No sharepod provided")
			return
		}

		err = client.KubeshareV1().SharePods(lookupNamespace).
			Delete(r.Context(), request.FunctionName, metav1.DeleteOptions{})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			klog.Errorf("Sharepod %s delete error: %v", request.FunctionName, err)
		}
	}
}
