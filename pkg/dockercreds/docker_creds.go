package dockercreds

import (
	"encoding/json"
	"io/ioutil"
)

type DockerCreds map[string]entry

func (c DockerCreds) AppendCredsToDockerConfig(path string) error {
	existingCreds, err := parseDockerConfigJson(path)
	if err != nil {
		return err
	}

	appendedCreds := c.append(existingCreds)
	configJson := dockerConfigJson{
		Auths: appendedCreds,
	}
	configJsonBytes, err := json.Marshal(configJson)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, configJsonBytes, 0600)
}

func (c DockerCreds) append(a DockerCreds) DockerCreds {
	if c == nil {
		return a
	}

	for k, v := range a {
		c[k] = v
	}
	return c
}

type entry struct {
	Auth string `json:"auth"`
}

type dockerConfigJson struct {
	Auths DockerCreds `json:"auths"`
}
