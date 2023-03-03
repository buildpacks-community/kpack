package git

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "k8s.io/client-go/kubernetes"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/secret"
)

type k8sGitKeychainFactory struct {
	secretFetcher secret.Fetcher
}

func newK8sGitKeychainFactory(k8sClient k8sclient.Interface) *k8sGitKeychainFactory {
	return &k8sGitKeychainFactory{secretFetcher: secret.Fetcher{Client: k8sClient}}
}

func (k *k8sGitKeychainFactory) KeychainForServiceAccount(ctx context.Context, namespace, serviceAccount string) (GitKeychain, error) {
	secrets, err := k.secretFetcher.SecretsForServiceAccount(ctx, serviceAccount, namespace)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
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

	return &secretGitKeychain{creds: creds}, nil
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
	//fmt.Printf("gitUrlMatch: len(matchingDomains)->%d\n", len(matchingDomains))
	for _, format := range matchingDomains {
		//fmt.Printf("gitUrlMatch: urlMatch->%s, annotatedUrl->%s, format->%s\n", urlMatch, annotatedUrl, format)
		//fmt.Printf("gitUrlMatch: checking match for formatted urlMatch. format->%s, urlMatch->%s, annotatedUrl->%s, Sprintf(format, urlMatch)->%s\n", format, urlMatch, annotatedUrl, fmt.Sprintf(format, urlMatch))
		if fmt.Sprintf(format, urlMatch) == annotatedUrl {
			// found match for formatted urlMatch
			//fmt.Printf("gitUrlMatch: found match for formatted urlMatch. format->%s, urlMatch->%s, annotatedUrl->%s, Sprintf(format, urlMatch)->%s\n", format, urlMatch, annotatedUrl, fmt.Sprintf(format, urlMatch))
			return true
		}
	}
	// no match found for formatted urlMatch
	//fmt.Printf("gitUrlMatch: no match found for formatted urlMatch. urlMatch->%s, annotatedUrl->%s\n", urlMatch, annotatedUrl)
	return false
}
