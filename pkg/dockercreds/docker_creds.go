package dockercreds

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"
)

type DockerAuthConfig map[string]authn.AuthConfig

type DockerCreds struct {
	CredMap        DockerAuthConfig
	LastModified   time.Time
	DockerCredPath string
}

func (c DockerCreds) Resolve(reg authn.Resource) (authn.Authenticator, error) {

	fileInfo, err := os.Stat(c.DockerCredPath)

	if err == nil && fileInfo.ModTime().After(c.LastModified) {
		newDockerCreds, _ := ParseDockerConfigSecret(c.DockerCredPath)

		c.CredMap = newDockerCreds.CredMap
		c.LastModified = newDockerCreds.LastModified
	}

	for registry, entry := range c.CredMap {
		matcher := RegistryMatcher{Registry: registry}
		if matcher.Match(reg.RegistryStr()) {
			return authn.FromConfig(entry), nil
		}
	}

	// Fallback on anonymous.
	return authn.Anonymous, nil
}

func (c DockerCreds) Save(path string) error {
	err := os.MkdirAll(filepath.Dir(path), 0777)
	if err != nil {
		return errors.Wrapf(err, "error creating %s", filepath.Dir(path))
	}

	configJson := dockerConfigJson{
		Auths: c.CredMap,
	}

	fh, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fh.Close()
	return json.NewEncoder(fh).Encode(configJson)
}

func (c DockerCreds) Append(a DockerCreds) (DockerCreds, error) {
	if c.CredMap == nil {
		return a, nil
	} else if a.CredMap == nil {
		return c, nil
	}

	for k, v := range a.CredMap {
		if contains, err := c.contains(k); err != nil {
			return DockerCreds{}, err
		} else if !contains {
			c.CredMap[k] = v
		}
	}

	lastModified := a.LastModified
	dockerCredPath := a.DockerCredPath
	if c.LastModified.After(a.LastModified) {
		lastModified = c.LastModified
		dockerCredPath = c.DockerCredPath
	}

	creds := DockerCreds{c.CredMap, lastModified, dockerCredPath}

	return creds, nil
}

func (c DockerCreds) contains(reg string) (bool, error) {
	if !strings.HasPrefix(reg, "http://") && !strings.HasPrefix(reg, "https://") {
		reg = "//" + reg
	}

	u, err := url.Parse(reg)
	if err != nil {
		return false, err
	}

	for existingRegistry := range c.CredMap {
		matcher := RegistryMatcher{Registry: existingRegistry}
		if matcher.Match(u.Host) {
			return true, nil
		}
	}

	return false, nil
}

type dockerConfigJson struct {
	Auths DockerAuthConfig `json:"auths"`
}
