package dockercreds

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pivotal/kpack/pkg/secret"
	"github.com/pkg/errors"
)

func ParseDockerConfigSecret(dir string) (DockerCreds, error) {
	dockerCfg, err := parseDockerCfg(filepath.Join(dir, ".dockercfg"))
	if err != nil {
		return nil, err
	}

	dockerJson, err := parseDockerConfigJson(filepath.Join(dir, ".dockerconfigjson"))
	if err != nil {
		return nil, err
	}

	return dockerCfg.Append(dockerJson)
}

func ParseBasicAuthSecrets(volumeName string, secrets []string) (DockerCreds, error) {
	var dockerCreds = DockerCreds{}
	for _, s := range secrets {
		splitSecret := strings.Split(s, "=")
		if len(splitSecret) != 2 {
			return nil, errors.Errorf("could not parse docker secret argument %s", s)
		}
		secretName := splitSecret[0]
		domain := splitSecret[1]

		auth, err := secret.ReadBasicAuthSecret(volumeName, secretName)
		if err != nil {
			return nil, err
		}

		dockerCreds[domain] = authn.AuthConfig{
			Username: auth.Username,
			Password: auth.Password,
		}
	}
	return dockerCreds, nil
}

func parseDockerCfg(path string) (DockerCreds, error) {
	var creds DockerCreds
	cfgExists, err := fileExists(path)
	if err != nil {
		return nil, err
	}

	if !cfgExists {
		return creds, nil
	}
	cfgFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(cfgFile, &creds)
	if err != nil {
		return nil, err
	}

	return creds, nil
}

func parseDockerConfigJson(path string) (DockerCreds, error) {
	config := dockerConfigJson{
		Auths: map[string]authn.AuthConfig{},
	}

	configjsonExists, err := fileExists(path)
	if err != nil {
		return nil, err
	}
	if !configjsonExists {
		return config.Auths, nil
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
