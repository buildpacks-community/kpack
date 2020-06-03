package main

import (
	"context"
	"flag"
	"log"
	"os"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/pivotal/kpack/pkg/logs"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig.")
	image      = flag.String("image", "", "The image name to tail logs")
	build      = flag.String("build", "", "The build number to tail logs")
	namespace  = flag.String("namespace", "default", "The namespace of the image")
)

func main() {
	flag.Parse()

	clusterConfig, err := BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	k8sClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err.Error())
	}

	if (*build) == "" {
		err = logs.NewBuildLogsClient(k8sClient).TailImage(context.Background(), os.Stdout, *image, *namespace)
	} else {
		err = logs.NewBuildLogsClient(k8sClient).Tail(context.Background(), os.Stdout, *image, *build, *namespace)
	}

	if err != nil {
		log.Fatalf("error tailing logs %s", err)
	}

}

func BuildConfigFromFlags(masterURL, kubeconfigPath string) (*rest.Config, error) {

	var clientConfigLoader clientcmd.ClientConfigLoader

	if kubeconfigPath == "" {
		clientConfigLoader = clientcmd.NewDefaultClientConfigLoadingRules()
	} else {
		clientConfigLoader = &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientConfigLoader,
		&clientcmd.ConfigOverrides{ClusterInfo: api.Cluster{Server: masterURL}}).ClientConfig()

}
