package test

import (
	"flag"
	"os/user"
	"path"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pivotal/build-service-beam/pkg/client/clientset/versioned"
)

func newClients() (*clients, error) {
	var defaultKubeconfig string
	if usr, err := user.Current(); err == nil {
		defaultKubeconfig = path.Join(usr.HomeDir, ".kube/config")
	}

	kubeconfig := flag.String("kubeconfig", defaultKubeconfig, "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL := flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	flag.Parse()

	clusterConfig, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := versioned.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}

	k8sClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		return nil, err
	}

	return &clients{
		client:    client,
		k8sClient: k8sClient,
	}, nil
}

type clients struct {
	client    versioned.Interface
	k8sClient kubernetes.Interface
}
