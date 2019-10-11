package k8svolume

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"k8s.io/kubernetes/pkg/credentialprovider"
)

const (
	SecretFilePathEnv = "CREDENTIAL_PROVIDER_SECRET_PATH"
)

func init() {
	credentialprovider.RegisterCredentialProvider("secret-volume", &VolumeSecretProvider{})
}

type VolumeSecretProvider struct {
}

func (k VolumeSecretProvider) Enabled() bool {
	_, ok := os.LookupEnv(SecretFilePathEnv)
	return ok
}

func (k VolumeSecretProvider) Provide() credentialprovider.DockerConfig {
	secretFolder := os.Getenv(SecretFilePathEnv)

	dockerConfig, err := parseDockerConfigJson(filepath.Join(secretFolder, ".dockerconfigjson"))
	if err != nil {
		log.Printf("unable to parse .dockerconfigjson secret configuration: %s", err.Error())
		return nil
	}
	return dockerConfig
}

// Do not implement the lazy for now
func (k VolumeSecretProvider) LazyProvide() *credentialprovider.DockerConfigEntry {
	return nil
}

func parseDockerConfigJson(path string) (credentialprovider.DockerConfig, error) {
	config := credentialprovider.DockerConfigJson{}
	configjsonExists, err := fileExists(path)
	if err != nil {
		return nil, err
	}
	if !configjsonExists {
		return nil, nil
	}

	configJsonFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(configJsonFile, &config)
	if err != nil {
		return nil, err
	}

	return config.Auths, nil
}

func fileExists(file string) (bool, error) {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, nil
	}

	return true, nil
}
