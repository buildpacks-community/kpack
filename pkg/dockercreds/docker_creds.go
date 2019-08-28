package dockercreds

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

type DockerCreds map[string]entry

func (c DockerCreds) Resolve(reg name.Registry) (authn.Authenticator, error) {
	registryMatcher := RegistryMatcher{}

	for registry, entry := range c {
		if registryMatcher.Match(reg.RegistryStr(), registry) {
			if entry.Auth != "" {
				return Auth(entry.Auth), nil
			} else if entry.Username != "" {
				return &authn.Basic{Username: entry.Username, Password: entry.Password}, nil
			}

			return nil, errors.Errorf("Unsupported entry in \"auths\" for %q", reg.RegistryStr())
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
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type dockerConfigJson struct {
	Auths DockerCreds `json:"auths"`
}
