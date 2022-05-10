package meaning

import (
	"flag"
	"log"
	"time"

	//"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	v1 "github.com/Interstellarss/faas-share/pkg/client/informers/externalversions/kubeshare/v1"
	"github.com/Interstellarss/faas-share/pkg/controller"
	"github.com/Interstellarss/faas-share/pkg/server"

	//v1 "github.com/Interstellarss/faas-share/pkg/client/informers/externealversions"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
	"github.com/Interstellarss/faas-share/pkg/config"
	"github.com/Interstellarss/faas-share/pkg/k8s"

	v1apps "k8s.io/client-go/informers/apps/v1"
	v1core "k8s.io/client-go/informers/core/v1"

	kubeinformers "k8s.io/client-go/informers"

	informers "github.com/Interstellarss/faas-share/pkg/client/informers/externalversions"

	providertypes "github.com/openfaas/faas-provider/types"
	//version "github.com/Interstellarss/faas-share/version"
)

func main() {
	var kubeconfig string
	var masterURL string
	var verbose bool

	flag.StringVar(&kubeconfig, "kubeconfig", "",
		"Path to a kubeconfig. Only required if out-of-cluster.")
	flag.BoolVar(&verbose, "verbose", false, "Print")

	flag.StringVar(&masterURL, "master", "",
		"The address of the Kubernetes API server. Overrides any value in kuebconfig. Only requeired if out-of-cluster.")

	flag.Parse()

	//sha, release := version.GetReleaseInfo()

	clientCmdConfig, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s", err.Error())

	}

	kubeconfigQPS := 100

	kubeconfigBurst := 250

	clientCmdConfig.QPS = float32(kubeconfigQPS)
	clientCmdConfig.Burst = kubeconfigBurst

	kubeclient, err := kubernetes.NewForConfig(clientCmdConfig)
	if err != nil {
		log.Fatalf("Error building Kubernetes clientset: %s", err.Error())
	}

	shareClient, err := clientset.NewForConfig(clientCmdConfig)
	if err != nil {
		log.Fatalf("Error building faas-share clientset: %s", err.Error())
	}

	readConfig := config.ReadConfig{}

	osEnv := providertypes.OsEnv{}

	config, err := readConfig.Read(osEnv)

	if err != nil {
		log.Fatalf("Error reading config: %s", err.Error())
	}

	config.Fprint(verbose)

	deployConfig := k8s.DeploymentConfig{
		RuntimeHTTPPort: 8080,
		HTTPProbe:       config.HTTPProbe,
		SetNonRootUser:  config.SetNonRootUser,
		ReadinessProbe: &k8s.ProbeConfig{
			InitialDelaySeconds: int32(config.ReadinessProbeInitialDelaySeconds),
			TimeoutSeconds:      int32(config.ReadinessProbeTimeoutSeconds),
			PeriodSeconds:       int32(config.ReadinessProbePeriodSeconds),
		},
		LivenessProbe: &k8s.ProbeConfig{
			InitialDelaySeconds: int32(config.LivenessProbeInitialDelaySeconds),
			TimeoutSeconds:      int32(config.LivenessProbeTimeoutSeconds),
			PeriodSeconds:       int32(config.LivenessProbePeriodSeconds),
		},
		ImagePullPolicy:   config.ImagePullPolicy,
		ProfilesNamespace: config.ProfilesNamespace,
	}

	//the sync interval does not affect the scale to/from

	defaultResync := time.Minute * 5

	namespaceScope := config.DefaultFunctionNamespace

	if config.ClusterRole {
		namespaceScope = ""
	}

	kubeInformerOpt := kubeinformers.WithNamespace(namespaceScope)
	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeclient, defaultResync, kubeInformerOpt)

	faasInformerOpt := informers.WithNamespace(namespaceScope)
	faasInformerFactory := informers.NewSharedInformerFactoryWithOptions(shareClient, defaultResync, faasInformerOpt)

	factory := k8s.NewFunctionFactory()

	setup := serverSetup{
		config:              config,
		functionFactory:     factory,
		kubeInformerFactory: kubeInformerFactory,
		faasInformerFactory: faasInformerFactory,
		kubeClient:          kubeclient,
		shareClient:         shareClient,
	}

	log.Println("Starting operator")
	runOperator(setup, config)

}

type customInformers struct {
	EndpointsInformer  v1core.EndpointsInformer
	DeploymentInformer v1apps.DeploymentInformer
	//TODO here, may need to change
	SharepodsInformer v1.SharePodInformer
}

//we probably will not need the operator bool, since we only have one deploy mode
func startInformers(setup serverSetup, stopCh <-chan struct{}) customInformers {
	kubeInformerFactory := setup.kubeInformerFactory
	faasInformerFactory := setup.faasInformerFactory

	var sharepods v1.SharePodInformer

	sharepods = faasInformerFactory.Kubeshare().V1().SharePods()
	go sharepods.Informer().Run(stopCh)
	if ok := cache.WaitForNamedCacheSync("faas-share:sharepods", stopCh, sharepods.Infromer().HasSynced); !ok {
		log.Fatalf("failed to wait for cache to sync")
	}

	//go kubeInformerFactory.Start(stopCh)

	deployments := kubeInformerFactory.Apps().V1().Deployments()
	go deployments.Informer().Run(stopCh)
	if ok := cache.WaitForNamedCacheSync("faas-share:deployments", stopCh, deployments.Informer().HasSynced); !ok {
		log.Fatalf("failed to wait for cache to sync")
	}

	endpoints := kubeInformerFactory.Core().V1().Endpoints()
	go endpoints.Informer().Run(stopCh)
	if ok := cache.WaitForNamedCacheSync("faas-share:endpoints", stopCh, endpoints.Informer().HasSynced); !ok {
		log.Fatalf("failed to wait for cache to sync")
	}

	//profielInformerFactory.Start()
	//we may not meed
	return customInformers{
		EndpointsInformer:  endpoints,
		DeploymentInformer: deployments,
		SharepodsInformer:  sharepods,
	}

}

func runOperator(setup serverSetup, cfg config.BootstrapConfig) {
	kubeClient := setup.kubeClient
	shareClient := setup.shareClient
	kubeInformerfacotry := setup.kubeInformerFactory
	faasInformerfactory := setup.faasInformerFactory

	//
	facory := controller.FunctionFactory{
		Factory: setup.functionFactory,
	}

	setupLooging()

	//TODO: signals need to configure
	//stopCh :=

	operator := true
	listers := startInformers(setup, stopCh)

	//TOOD: conttroller pkg for faas-share
	ctrl := controller.NewController()

	srv := server.New(shareClient, kubeClient, listers.EndpointsInformer, listers.DeploymentInformer.Lister(), cfg.ClusterRole, cfg)

	go srv.Start()
	if err := ctrl.Run(1, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}

type serverSetup struct {
	config              config.BootstrapConfig
	kubeClient          *kubernetes.Clientset
	shareClient         *clientset.Clientset
	functionFactory     k8s.FunctionFactory
	kubeInformerFactory kubeinformers.SharedInformerFactory
	faasInformerFactory informers.SharedInformerFactory
	//do we still need this part?
	//profileInformerFactory informers.SharedInformerFactory
}

func setupLogging() {
	klogFlags := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFlags)

	//Sync
	flag.CommandLine.VisitAll(func(f1 *flag.Flag) {
		f2 := klogFlags.Lookup(f1.Name)

		if f2 != nil {
			value := f1.Value.String()
			_ = f2.Value.Set(value)
		}
	})
}
