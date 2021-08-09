package cosigner

import (
	"context"
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/cmd/cosign/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
)

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
}

type ImageSigner struct {
	Logger    *log.Logger
	K8sClient k8sclient.Interface
}

var cliSignCmd = cli.SignCmd

// Other keyops support: https://github.com/sigstore/cosign/blob/143e47a120702f175e68e0a04594cb87a4ce8e02/cmd/cosign/cli/sign.go#L167
// Todo: Annotation obtained from kpack config

func NewImageSigner(logger *log.Logger, k8sClient k8sclient.Interface) *ImageSigner {
	return &ImageSigner{
		Logger:    logger,
		K8sClient: k8sClient,
	}
}

func (s *ImageSigner) Sign(refImage, namespace, serviceAccountName string) error {
	if refImage == "" {
		return fmt.Errorf("signing reference image is empty")
	}

	if namespace == "" {
		return fmt.Errorf("namespace is empty")
	}

	if serviceAccountName == "" {
		return fmt.Errorf("service account name is empty")
	}

	serviceAccount, err := s.K8sClient.CoreV1().ServiceAccounts(namespace).Get(context.Background(), serviceAccountName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get service account: %v", err)
	}

	// Todo: Iterate over secrets
	keyPath := fmt.Sprintf("%s/%s", namespace, serviceAccount.Secrets[0].Name)

	ctx := context.Background()
	ko := cli.KeyOpts{KeyRef: keyPath}

	if err = cliSignCmd(ctx, ko, nil, refImage, "", true, "", false, false); err != nil {
		return fmt.Errorf("signing: %v", err)
	}

	return nil
}
