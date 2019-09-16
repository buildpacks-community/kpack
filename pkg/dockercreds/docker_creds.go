package dockercreds

import (
	"encoding/json"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
)

type DockerCreds map[string]entry

func (c DockerCreds) Resolve(reg name.Registry) (authn.Authenticator, error) {
	for registry, entry := range c {
		if RegistryMatch(reg.RegistryStr(), registry) {
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

	appendedCreds, err := existingCreds.append(c)
	if err != nil {
		return err
	}

	configJson := dockerConfigJson{
		Auths: appendedCreds,
	}
	configJsonBytes, err := json.Marshal(configJson)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, configJsonBytes, 0600)
}

func (c DockerCreds) append(a DockerCreds) (DockerCreds, error) {
	if c == nil {
		return a, nil
	} else if a == nil {
		return c, nil
	}

	for k, v := range a {
		if contains, err := c.contains(k); err != nil {
			return nil, err
		} else if !contains {
			c[k] = v
		}
	}

	return c, nil
}

func (c DockerCreds) contains(reg string) (bool, error) {
	if !strings.HasPrefix(reg, "http://") && !strings.HasPrefix(reg, "https://") {
		reg = "//" + reg
	}

	u, err := url.Parse(reg)
	if err != nil {
		return false, err
	}

	for existingRegistry := range c {
		if RegistryMatch(u.Host, existingRegistry) {
			return true, nil
		}
	}

	return false, nil
}

type entry struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type dockerConfigJson struct {
	Auths DockerCreds `json:"auths"`
}
