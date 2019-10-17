package dockercreds

import (
	"strings"

	"github.com/pivotal/kpack/pkg/secret"
	"github.com/pkg/errors"
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

		auth, err := secret.ReadSecret(volumeName, secretName)
		if err != nil {
			return nil, err
		}

		dockerCreds[domain] = entry{
			Username: auth.Username,
			Password: auth.Password,
		}
	}
	return dockerCreds, nil
}
