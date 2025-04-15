package test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type config struct {
	builder              string
	testRegistry         string
	testRegistryUsername string
	testRegistryPassword string
	gitSourcePrivateRepo string
	gitSourceUsername    string
	gitSourcePassword    string
	gitSourcePrivateKey  string
	imageTag             string
}

type dockerCredentials map[string]authn.AuthConfig

type dockerConfigJson struct {
	Auths dockerCredentials `json:"auths"`
}

const (
	lifecycleImage = "mirror.gcr.io/buildpacksio/lifecycle"
)

func loadConfig(t *testing.T) config {
	gitPrivateRepo, _ := os.LookupEnv("GIT_PRIVATE_REPO")
	gitUsername, _ := os.LookupEnv("GIT_BASIC_USERNAME")
	gitPassword, _ := os.LookupEnv("GIT_BASIC_PASSWORD")
	gitPrivateKey, _ := os.LookupEnv("GIT_SSH_PRIVATE_KEY")

	registry, found := os.LookupEnv("IMAGE_REGISTRY")
	if !found {
		t.Fatal("IMAGE_REGISTRY env is needed for tests")
	}

	imageUsername, found := os.LookupEnv("IMAGE_REGISTRY_USERNAME")
	if !found {
		t.Fatal("IMAGE_REGISTRY_USERNAME env is needed for tests")
	}

	imagePassword, found := os.LookupEnv("IMAGE_REGISTRY_PASSWORD")
	if !found {
		t.Fatal("IMAGE_REGISTRY_PASSWORD env is needed for tests")
	}

	return config{
		testRegistry:         registry,
		testRegistryUsername: imageUsername,
		testRegistryPassword: imagePassword,

		gitSourcePrivateRepo: gitPrivateRepo,
		gitSourceUsername:    gitUsername,
		gitSourcePassword:    gitPassword,
		gitSourcePrivateKey:  gitPrivateKey,

		imageTag: registry + "/kpack-test",
	}
}

func (c *config) newImageTag() string {
	genTag := c.imageTag + "-" + strconv.Itoa(rand.Int())
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

func (c *config) makeGitBasicAuthSecret(secretName, namespace string) (*corev1.Secret, string) {
	if c.gitSourceUsername == "" || c.gitSourcePassword == "" {
		return nil, ""
	}

	// convert `github.com/org/repo` -> `https://github.com/org/repo.git`
	repo := fmt.Sprintf("https://%v", c.gitSourcePrivateRepo)
	host := strings.Split(c.gitSourcePrivateRepo, "/")[0]

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Annotations: map[string]string{
				v1alpha2.GITSecretAnnotationPrefix: fmt.Sprintf("https://%v", host),
			},
		},
		Data: map[string][]byte{
			corev1.BasicAuthUsernameKey: []byte(c.gitSourceUsername),
			corev1.BasicAuthPasswordKey: []byte(c.gitSourcePassword),
		},
		Type: corev1.SecretTypeBasicAuth,
	}, repo
}

func (c *config) makeGitSSHAuthSecret(secretName, namespace string) (*corev1.Secret, string) {
	if c.gitSourcePrivateKey == "" {
		return nil, ""
	}

	// convert `github.com/org/repo` -> `git@github.com:org/repo.git`
	repo := fmt.Sprintf("git@%v", c.gitSourcePrivateRepo)
	repo = strings.Replace(repo, "/", ":", 1)
	host := strings.Split(c.gitSourcePrivateRepo, "/")[0]

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Annotations: map[string]string{
				v1alpha2.GITSecretAnnotationPrefix: fmt.Sprintf("git@%v", host),
			},
		},
		Data: map[string][]byte{
			corev1.SSHAuthPrivateKey: []byte(c.gitSourcePrivateKey),
		},
		Type: corev1.SecretTypeSSHAuth,
	}, repo
}
