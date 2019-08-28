package dockercreds

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

func ParseDockerPullSecrets(path string) (DockerCreds, error) {
	dockerCfg, err := parseDockerCfg(filepath.Join(path, ".dockercfg"))
	if err != nil {
		return nil, err
	}

	dockerJson, err := parseDockerConfigJson(filepath.Join(path, ".dockerconfigjson"))
	if err != nil {
		return nil, err
	}

	return dockerCfg.append(dockerJson), nil
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
	var config dockerConfigJson
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
