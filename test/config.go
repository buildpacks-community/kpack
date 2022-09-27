package test

import (
	"encoding/json"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type config struct {
	builder              string
	testRegistry         string
	testRegistryUsername string
	testRegistryPassword string
	imageTag             string
	istioEnabled         bool

	generatedImageNames []string
}

type dockerCredentials map[string]authn.AuthConfig

type dockerConfigJson struct {
	Auths dockerCredentials `json:"auths"`
}

func loadConfig(t *testing.T) config {
	registry, found := os.LookupEnv("IMAGE_REGISTRY")
	if !found {
		t.Fatal("IMAGE_REGISTRY env is needed for tests")
	}

	username, found := os.LookupEnv("IMAGE_REGISTRY_USERNAME")
	if !found {
		t.Fatal("IMAGE_REGISTRY_USERNAME env is needed for tests")
	}

	password, found := os.LookupEnv("IMAGE_REGISTRY_PASSWORD")
	if !found {
		t.Fatal("IMAGE_REGISTRY_PASSWORD env is needed for tests")
	}

	istioEnabled := false
	value, _ := os.LookupEnv("ISTIO_ENABLED")
	if value != "" {
		istioEnabled, err = strconv.ParseBool(value)
		if err != nil {
			t.Fatal("ISTIO_ENABLED must be a valid boolean")
		}
	}

	return config{
		testRegistry:         registry,
		testRegistryUsername: username,
		testRegistryPassword: password,
		imageTag:             registry + "/kpack-test",
		istioEnabled:         istioEnabled,
	}
}

func (c *config) newImageTag() string {
	genTag := c.imageTag + "-" + strconv.Itoa(rand.Int())
	c.generatedImageNames = append(c.generatedImageNames, genTag)
	return genTag
}

func (c *config) makeRegistrySecret(secretName string, namespace string) (*corev1.Secret, error) {
	reg := c.testRegistry
	// Handle path in registry
	if strings.ContainsRune(reg, '/') {
		r, err := name.NewRepository(reg, name.WeakValidation)
		if err != nil {
			return nil, err
		}
		reg = r.RegistryStr()
	}

	configJson := dockerConfigJson{Auths: dockerCredentials{
		reg: authn.AuthConfig{
			Username: c.testRegistryUsername,
			Password: c.testRegistryPassword,
		},
	}}
	dockerCfgJson, err := json.Marshal(configJson)
	if err != nil {
		return nil, err
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			corev1.DockerConfigJsonKey: dockerCfgJson,
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}, nil
}
