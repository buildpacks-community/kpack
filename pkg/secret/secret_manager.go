package secret

import (
	"encoding/json"
	"fmt"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sclient "k8s.io/client-go/kubernetes"
)

type SecretManager struct {
	Client        k8sclient.Interface
	AnnotationKey string
	Matcher       Matcher
}

type Matcher interface {
	Match(url, annotatedUrl string) bool
}

func (m *SecretManager) SecretForServiceAccountAndURL(serviceAccount, namespace string, url string) (*URLAndUser, error) {
	sa, err := m.Client.CoreV1().ServiceAccounts(namespace).Get(serviceAccount, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	secret, err := m.secretForServiceAccount(sa, url, namespace)
	if err != nil {
		return nil, err
	}

	registryUser := NewURLAndUser(url, string(secret.Data["username"]), string(secret.Data["password"]))
	return &registryUser, nil
}

func (m *SecretManager) secretForServiceAccount(account *v1.ServiceAccount, url string, namespace string) (*v1.Secret, error) {
	for _, secretRef := range account.Secrets {
		secret, err := m.Client.CoreV1().Secrets(namespace).Get(secretRef.Name, meta_v1.GetOptions{})
		if err != nil {
			return nil, err
		}

		if m.Matcher.Match(url, secret.ObjectMeta.Annotations[m.AnnotationKey]) {
			return secret, nil
		}

	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{Group: "", Resource: "Secret"}, fmt.Sprintf("secret for %s", url))
}

type dockerConfigJson struct {
	Auths dockerConfig `json:"auths"`
}

type dockerConfig map[string]dockerConfigEntry

type dockerConfigEntry struct {
	Auth string
}

func (m *SecretManager) SecretForImagePull(namespace, secretName, registryName string) (string, error) {
	secret, err := m.Client.CoreV1().Secrets(namespace).Get(secretName, meta_v1.GetOptions{})
	if err != nil {
		return "", err
	}

	var config dockerConfigJson
	switch secret.Type {
	case v1.SecretTypeDockercfg:
		var auths dockerConfig
		err = json.Unmarshal(secret.Data[v1.DockerConfigKey], &auths)
		config.Auths = auths
	case v1.SecretTypeDockerConfigJson:
		err = json.Unmarshal(secret.Data[v1.DockerConfigJsonKey], &config)
	}
	if err != nil {
		return "", err
	}

	for registry, registryAuth := range config.Auths {
		if m.Matcher.Match(registryName, registry) {
			return registryAuth.Auth, nil
		}
	}
	return "", fmt.Errorf("no secret configuration for registry: %s", registryName)
}
