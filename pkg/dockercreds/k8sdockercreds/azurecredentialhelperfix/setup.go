package azurecredentialhelperfix

import (
	_ "github.com/vdemeester/k8s-pkg-credentialprovider/azure"

	"os"

	"github.com/spf13/pflag"
)

func init() {
	pflag.Set("azure-container-registry-config", os.Getenv("AZURE_CONTAINER_REGISTRY_CONFIG"))
}
