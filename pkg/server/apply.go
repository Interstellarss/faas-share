package server

import (
	"net/http"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
)

func makeApplyHandler(defaultNamespace string, client clientset.Interface) http.HandlerFunc {

}
