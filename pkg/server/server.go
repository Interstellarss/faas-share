package server

import (
	"os"

	"net/http"
	_ "net/http/pprof"

	"github.com/Interstellarss/faas-share/pkg/config"
	"github.com/Interstellarss/faas-share/pkg/k8s"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"

	bootstrap "github.com/openfaas/faas-provider"

	coreinformer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openfaas/faas-provider/logs"
	"github.com/openfaas/faas-provider/proxy"
	"github.com/openfaas/faas-provider/types"

	//listers "github.com/Interstellarss/faas-share/pkg/client/listers/kubeshare/v1"

	v1apps "k8s.io/client-go/listers/apps/v1"

	//faassharek8s "github.com/Interstellarss/faas-share/pkg/k8s"

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

	sharepodNamespace := "faas-share-fn"

	if namespace, exists := os.LookupEnv("function_namspace"); exists {
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
		FunctionProxy:  proxy.NewHandlerFunc(bootstrapConfig, sharepodLookup),
		DeleteHandler:  makeDeleteHandler(sharepodNamespace, client),
		DeployHandler:  makeApplyHandler(sharepodNamespace, client),
		FunctionReader: makeListHandler(sharepodNamespace, client, deploymentLister),
		ReplicaReader:  makeReplicaReader(sharepodNamespace, client, deploymentLister),
		ReplicaUpdater: makeReplicaHandler(sharepodNamespace, client),
		UpdateHandler:  makeHealthReader(),
		HealthHandler:  makeHealthReader(),
		InfoHandler:    makeInfoHandler(),
		//SecretHandler: ,
		LogHandler: logs.NewLogHandlerFunc(k8s.NewLogRequestor(kube, sharepodNamespace), bootstrapConfig.WriteTimeout),

		ListNamespaceHandler: MakeNamespacesLister(sharepodNamespace, clusterRole, kube),
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

func (s *Server) Start() {
	klog.Infof("Starting HTTP server on port %d", *s.BootstrapConfig.TCPPort)

	bootstrap.Serve(s.BootstrapHandlers, s.BootstrapConfig)
}

/*
func int32p(i int32) *int32 {
	return &i
}
*/
