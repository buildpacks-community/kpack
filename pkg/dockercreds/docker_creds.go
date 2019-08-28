package dockercreds

import (
	"encoding/json"
	"io/ioutil"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

type DockerCreds map[string]entry

func (c DockerCreds) Resolve(reg name.Registry) (authn.Authenticator, error) {
	registryMatcher := RegistryMatcher{}

	for registry, registryAuth := range c {
		if registryMatcher.Match(reg.RegistryStr(), registry) {
			return Auth(registryAuth.Auth), nil
		}
	}

	// Fallback on anonymous.
	return authn.Anonymous, nil
}

func (c DockerCreds) AppendToDockerConfig(path string) error {
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
