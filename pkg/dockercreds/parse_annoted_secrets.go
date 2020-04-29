package dockercreds

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/secret"
)

func ParseMountedAnnotatedSecrets(volumeName string, secrets []string) (DockerCreds, error) {
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
