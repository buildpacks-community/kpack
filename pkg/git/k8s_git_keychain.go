package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing/transport"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "k8s.io/client-go/kubernetes"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/secret"
)

type k8sGitKeychain struct {
	secretFetcher secret.Fetcher
}

var anonymousAuth transport.AuthMethod = nil

func newK8sGitKeychain(k8sClient k8sclient.Interface) *k8sGitKeychain {
	return &k8sGitKeychain{secretFetcher: secret.Fetcher{Client: k8sClient}}
}

func (k *k8sGitKeychain) Resolve(ctx context.Context, namespace, serviceAccount string, git corev1alpha1.Git) (transport.AuthMethod, error) {
	secrets, err := k.secretFetcher.SecretsForServiceAccount(ctx, serviceAccount, namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		return anonymousAuth, nil
	}

	var creds []gitCredential
	for _, s := range secrets {
		switch s.Type {
		case v1.SecretTypeBasicAuth:
			{
				creds = append(creds, gitBasicAuthCred{
					Domain:      s.Annotations[buildapi.GITSecretAnnotationPrefix],
					SecretName:  s.Name,
					fetchSecret: fetchBasicAuth(s),
				})
			}
		case v1.SecretTypeSSHAuth:
			{
				creds = append(creds, gitSshAuthCred{
					Domain:      s.Annotations[buildapi.GITSecretAnnotationPrefix],
					SecretName:  s.Name,
					fetchSecret: fetchSshAuth(s),
				})
			}
		}
	}

	return (&secretGitKeychain{creds: creds}).Resolve(git.URL)
}

func fetchBasicAuth(s *v1.Secret) func() (secret.BasicAuth, error) {
	return func() (auth secret.BasicAuth, err error) {
		return secret.BasicAuth{
			Username: string(s.Data[v1.BasicAuthUsernameKey]),
			Password: string(s.Data[v1.BasicAuthPasswordKey]),
		}, nil
	}
}

func fetchSshAuth(s *v1.Secret) func() (secret.SSH, error) {
	return func() (auth secret.SSH, err error) {
		return secret.SSH{PrivateKey: string(s.Data[v1.SSHAuthPrivateKey])}, nil
	}
}

var matchingDomains = []string{
	// Allow naked domains
	"%s",
	// Allow scheme-prefixed.
	"https://%s",
	"http://%s",
	"git@%s",
}

func gitUrlMatch(urlMatch, annotatedUrl string) bool {
	for _, format := range matchingDomains {
		if fmt.Sprintf(format, urlMatch) == annotatedUrl {
			return true
		}
	}
	return false
}
