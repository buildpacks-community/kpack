package dockercreds

import (
	"encoding/json"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pivotal/kpack/pkg/secret"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ParseDockerConfigSecret(dir string) (DockerCreds, error) {
	dockerCfg, err := parseDockerCfg(filepath.Join(dir, ".dockercfg"))
	if err != nil {
		return DockerCreds{}, err
	}

	dockerJson, err := parseDockerConfigJson(filepath.Join(dir, ".dockerconfigjson"))
	if err != nil {
		return DockerCreds{}, err
	}

	return dockerCfg.Append(dockerJson)
}

func ParseBasicAuthSecrets(volumeName string, secrets []string) (DockerCreds, error) {
	lastModified := time.Now()
	fileInfo, err := os.Stat(volumeName)

	if err == nil {
		lastModified = fileInfo.ModTime()
	}
	var dockerCreds = DockerCreds{}
	dockerCreds.CredMap = map[string]authn.AuthConfig{}
	for _, s := range secrets {
		splitSecret := strings.Split(s, "=")
		if len(splitSecret) != 2 {
			return DockerCreds{}, errors.Errorf("could not parse docker secret argument %s", s)
		}
		secretName := splitSecret[0]
		domain := splitSecret[1]

		auth, err := secret.ReadBasicAuthSecret(volumeName, secretName)
		if err != nil {
			return DockerCreds{}, err
		}

		dockerCreds.CredMap[domain] = authn.AuthConfig{
			Username: auth.Username,
			Password: auth.Password,
		}
		dockerCreds.LastModified = lastModified
	}
	return dockerCreds, nil
}

func parseDockerCfg(path string) (DockerCreds, error) {

	lastModified := time.Now()
	fileInfo, err := os.Stat(path)

	if err == nil {
		lastModified = fileInfo.ModTime()
	}
	var creds DockerCreds

	cfgExists, err := fileExists(path)
	if err != nil {
		return DockerCreds{}, err
	}

	if !cfgExists {
		return creds, nil
	}
	cfgFile, err := os.ReadFile(path)
	if err != nil {
		return DockerCreds{}, err
	}
	err = json.Unmarshal(cfgFile, &creds.CredMap)
	if err != nil {
		return DockerCreds{}, err
	}
	creds.DockerCredPath = path
	creds.LastModified = lastModified

	return creds, nil
}

func parseDockerConfigJson(path string) (DockerCreds, error) {
	lastModified := time.Now()

	fileInfo, err := os.Stat(path)
	if err == nil {
		lastModified = fileInfo.ModTime()
	}

	var creds DockerCreds

	config := dockerConfigJson{
		Auths: creds.CredMap,
	}

	configjsonExists, err := fileExists(path)
	if err != nil {
		return DockerCreds{}, err
	}
	if !configjsonExists {
		return creds, nil
	}

	configJsonFile, err := os.ReadFile(path)
	if err != nil {
		return DockerCreds{}, err
	}
	err = json.Unmarshal(configJsonFile, &config)
	if err != nil {
		return DockerCreds{}, err
	}

	creds.CredMap = config.Auths
	creds.LastModified = lastModified
	creds.DockerCredPath = path

	return creds, nil
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
