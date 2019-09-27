package git

import (
	"fmt"
	"net/url"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/secret"
)

type k8sGitKeychain struct {
	secretManager secret.SecretManager
}

func newK8sGitKeychain(k8sClient k8sclient.Interface) *k8sGitKeychain {
	return &k8sGitKeychain{secretManager: secret.SecretManager{
		Client:        k8sClient,
		AnnotationKey: v1alpha1.GITSecretAnnotationPrefix,
		Matcher:       gitUrlMatch,
	}}
}

func (k *k8sGitKeychain) Resolve(namespace, serviceAccount string, git v1alpha1.Git) (Auth, error) {
	if serviceAccount == "" {
		return AnonymousAuth{}, nil
	}

	creds, err := k.secretManager.SecretForServiceAccountAndURL(serviceAccount, namespace, git.URL)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}
	if k8serrors.IsNotFound(err) {
		return AnonymousAuth{}, nil
	}

	return BasicAuth{Username: creds.Username, Password: creds.Password}, nil
}

var matchingDomains = []string{
	// Allow naked domains
	"%s",
	// Allow scheme-prefixed.
	"https://%s",
	"http://%s",
}

func gitUrlMatch(urlMatch, annotatedUrl string) bool {
	parseURL, err := url.Parse(urlMatch)
	if err != nil {
		return false
	}

	for _, format := range matchingDomains {
		if fmt.Sprintf(format, parseURL.Hostname()) == annotatedUrl {
			return true
		}
	}

	return false
}
