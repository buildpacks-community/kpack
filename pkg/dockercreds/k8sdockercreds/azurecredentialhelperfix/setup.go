package setup

import (
	`github.com/spf13/pflag`
	//_ "k8s.io/kubernetes/pkg/credentialprovider/azure"

	"os"

)

func init() {
	//allow azure credential helper to be proccess its config before k8schain loads
	//https://github.com/google/go-containerregistry/pull/652
	pflag.Set("azure-container-registry-config", os.Getenv("AZURE_CONTAINER_REGISTRY_CONFIG"))
}
