package git

import (
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"

	"github.com/pivotal/kpack/pkg/secret"
)

type GitKeychain interface {
	Resolve(gitUrl string) (transport.AuthMethod, error)
}

type secretGitKeychain struct {
	gitCredentials []gitCredentials
	volumeName     string
}

type gitCredentials struct {
	Domain     string
	SecretName string
}

func NewMountedSecretGitKeychain(volumeName string, secrets []string) (*secretGitKeychain, error) {
	var gitCreds []gitCredentials
	for _, s := range secrets {
		splitSecret := strings.Split(s, "=")
		if len(splitSecret) != 2 {
			return nil, errors.Errorf("could not parse git secret argument %s", s)
		}

		gitCreds = append(gitCreds, gitCredentials{
			Domain:     splitSecret[1],
			SecretName: splitSecret[0],
		})
	}
	return &secretGitKeychain{
		gitCredentials: gitCreds,
		volumeName:     volumeName,
	}, nil
}

func (k *secretGitKeychain) Resolve(url string) (transport.AuthMethod, error) {
	for _, creds := range k.gitCredentials {
		if gitUrlMatch(url, creds.Domain) {
			basicAuth, err := secret.ReadSecret(k.volumeName, creds.SecretName)
			if err != nil {
				return nil, err
			}
			return &http.BasicAuth{
				Username: basicAuth.Username,
				Password: basicAuth.Password,
			}, nil
		}
	}
	return anonymousAuth, nil
}
