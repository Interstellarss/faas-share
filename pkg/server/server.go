package server

import (
	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"

	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openfaas/faas-provider/types"
)

//TODO: Move to config pattern used else-where across projects

const defaultHTTPPort = 8081
const defaultReadTimeout = 8
const defaultWriteTimeout = 8

// New created HTTP server struct
func New(client clientset.Interface,
	kube kubernetes.Interface,
	endpointsInformer coreinformer.EndpointsInformer,
	deploymentLister v1apps.deploymentLister,
	clusterRole bool,
	//cfg config.BootstrapConfig

) *Server {

}

type Server struct {
	BootstrapHandlers *types.FaaSHandlers
	BootstrapConfig   *types.FaaSConfig
}
