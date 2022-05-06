package server

import (
	"os"

	"net/http"
	_ "net/http/pprof"

	"github.com/Interstellarss/faas-share/pkg/k8s"

	"github.com/Interstellarss/faas-share/pkg/config"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"

	bootstrap "github.com/openfaas/faas-provider"

	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openfaas/faas-provider/proxy"
	"github.com/openfaas/faas-provider/types"

	v1apps "k8s.io/client-go/listers/apps/v1"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/klog"
)

//TODO: Move to config pattern used else-where across projects

const defaultHTTPPort = 8081
const defaultReadTimeout = 8
const defaultWriteTimeout = 8

// New created HTTP server struct
func New(client clientset.Interface,
	kube kubernetes.Interface,
	endpointsInformer coreinformer.EndpointsInformer,
	deploymentLister v1apps.DeploymentLister,
	clusterRole bool,
	cfg config.BootstrapConfig) *Server {

	sharepodNamespace := "faas-share"

	if namespace, exists := os.LookupEnv("sharepod_namspace"); exists {
		sharepodNamespace = namespace
	}

	pprof := "false"

	if val, exists := os.LookupEnv("pprof"); exists {
		pprof = val
	}

	lister := endpointsInformer.Lister()
	sharepodLookup := k8s.NewFunctionLookup(sharepodNamespace, lister)

	bootstrapConfig := types.FaaSConfig{
		ReadTimeout:  cfg.FaaSConfig.ReadTimeout,
		WriteTimeout: cfg.FaaSConfig.WriteTimeout,
		TCPPort:      cfg.FaaSConfig.TCPPort,
		EnableHealth: true,
	}

	bootstrapHandlers := types.FaaSHandlers{
		//TODO: amybe need tochange the proxy  newHandlerFunc?
		FunctionProxy: proxy.NewHandlerFunc(bootstrapConfig, sharepodLookup),

		DeployHandler: makeApplyHandler(sharepodNamespace, client),
	}

	if pprof == "true" {
		bootstrap.Router().PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	}

	bootstrap.Router().Path("/metrics").Handler(promhttp.Handler())

	klog.Infof("Using namespace '%s'", sharepodNamespace)

	return &Server{
		BootstrapConfig:   &bootstrapConfig,
		BootstrapHandlers: &bootstrapHandlers,
	}

}

type Server struct {
	BootstrapHandlers *types.FaaSHandlers
	BootstrapConfig   *types.FaaSConfig
}
