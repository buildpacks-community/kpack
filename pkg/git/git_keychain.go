package git

import (
	"strings"
)

type secretGitKeychain struct {
	GitCredentials []gitCredentials
	SecretReader   SecretReader
}

type gitCredentials struct {
	Domain     string
	SecretName string
}

//go:generate counterfeiter . SecretReader
type SecretReader interface {
	FromSecret(secretName string) (*BasicAuth, error)
}

func NewGitKeychain(secrets []string, secretReader SecretReader) *secretGitKeychain {
	var gitCreds []gitCredentials
	for _, s := range secrets {
		splitSecret := strings.Split(s, "=")
		gitCreds = append(gitCreds, gitCredentials{
			Domain:     splitSecret[1],
			SecretName: splitSecret[0],
		})
	}
	return &secretGitKeychain{
		GitCredentials: gitCreds,
		SecretReader:   secretReader,
	}
}

func (k *secretGitKeychain) Resolve(gitUrl string) (Auth, error) {
	for _, creds := range k.GitCredentials {
		if gitUrlMatch(gitUrl, creds.Domain) {
			return k.SecretReader.FromSecret(creds.SecretName)
		}
	}
	return AnonymousAuth{}, nil
}
