package test

import (
	"flag"
	"os/user"
	"path"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

var (
	setup             sync.Once
	defaultKubeconfig string
	client            *versioned.Clientset
	k8sClient         *kubernetes.Clientset
	clusterConfig     *rest.Config
	err               error
)

func newClients(t *testing.T) (*clients, error) {
	if usr, err := user.Current(); err == nil {
		defaultKubeconfig = path.Join(usr.HomeDir, ".kube/config")
	}

	setup.Do(func() {
		kubeconfig := flag.String("kubeconfig", defaultKubeconfig, "Path to a kubeconfig. Only required if out-of-cluster.")
		masterURL := flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

		flag.Parse()

		clusterConfig, err = clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
		if err != nil {
			return
		}

		client, err = versioned.NewForConfig(clusterConfig)
		if err != nil {
			return
		}

		k8sClient, err = kubernetes.NewForConfig(clusterConfig)
		if err != nil {
			return
		}
	})
	require.NoError(t, err)

	return &clients{
		client:    client,
		k8sClient: k8sClient,
	}, nil
}

type clients struct {
	client    versioned.Interface
	k8sClient kubernetes.Interface
}
