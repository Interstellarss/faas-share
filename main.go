package meaning

import (
	"flag"
	"log"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/Interstellarss/faas-share/pkg/client/clientset/versioned"
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

	readConfig := config.ReadConfig()
	//osEnv :=

}

func runOperator() {

}
