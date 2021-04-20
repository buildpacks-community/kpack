package test

import (
	"flag"
	"os"
	"os/user"
	"path"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

var (
	setup         sync.Once
	client        *versioned.Clientset
	k8sClient     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	clusterConfig *rest.Config
	err           error
)

func newClients(t *testing.T) (*clients, error) {
	setup.Do(func() {
		kubeconfig := flag.String("kubeconfig", getKubeConfig(), "Path to a kubeconfig. Only required if out-of-cluster.")
		masterURL := flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

		flag.Parse()

		clusterConfig, err = clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
		require.NoError(t, err)

		client, err = versioned.NewForConfig(clusterConfig)
		require.NoError(t, err)

		k8sClient, err = kubernetes.NewForConfig(clusterConfig)
		require.NoError(t, err)

		dynamicClient, err = dynamic.NewForConfig(clusterConfig)
		require.NoError(t, err)
	})

	return &clients{
		client:        client,
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
	}, nil
}

func getKubeConfig() string {
	if config, found := os.LookupEnv("KUBECONFIG"); found {
		return config
	}
	if usr, err := user.Current(); err == nil {
		return path.Join(usr.HomeDir, ".kube/config")
	}
	return ""
}

type clients struct {
	client        versioned.Interface
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
}
