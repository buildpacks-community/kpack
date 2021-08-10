package cosigner

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/cmd/cosign/cli"
)

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
}

type ImageSigner struct {
	Logger *log.Logger
}

var cliSignCmd = cli.SignCmd
var secretLocation = "/var/build-secrets"

// Other keyops support: https://github.com/sigstore/cosign/blob/143e47a120702f175e68e0a04594cb87a4ce8e02/cmd/cosign/cli/sign.go#L167
// Todo: Annotation obtained from kpack config

func NewImageSigner(logger *log.Logger) *ImageSigner {
	return &ImageSigner{
		Logger: logger,
	}
}

// signCmd will just use the mounted file instead of trying to access kuberenets for the secret
func (s *ImageSigner) Sign(reportFilePath string) error {
	// Read Report File
	// Obtain first item from Tags (cosign will sign based on digest)
	// Go to the "secretLocation" and look for all cosign.key files
	// Loop through cosign.key files
	// signCmd with image and cosign.key path
	keyPath := ""
	refImage := ""

	ctx := context.Background()
	ko := cli.KeyOpts{KeyRef: keyPath}

	if err := cliSignCmd(ctx, ko, nil, refImage, "", true, "", false, false); err != nil {
		return fmt.Errorf("signing: %v", err)
	}
	return nil
}
