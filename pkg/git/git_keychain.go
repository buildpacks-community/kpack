package git

import (
	"net"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	gitSsh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"

	"github.com/pivotal/kpack/pkg/secret"
)

type GitKeychain interface {
	Resolve(gitUrl string) (transport.AuthMethod, error)
}

type gitCredential interface {
	match(endpoint *transport.Endpoint) bool
	auth() (transport.AuthMethod, error)
	name() string
}

type secretGitKeychain struct {
	creds []gitCredential
}

type gitSshAuthCred struct {
	fetchSecret func() (secret.SSH, error)
	Domain      string
	SecretName  string
}

func (g gitSshAuthCred) match(endpoint *transport.Endpoint) bool {
	return endpoint.Protocol == "ssh" && gitUrlMatch(endpoint.Host, g.Domain)
}

func (g gitSshAuthCred) auth() (transport.AuthMethod, error) {
	sshSecret, err := g.fetchSecret()
	if err != nil {
		return nil, err
	}
	keys, err := gitSsh.NewPublicKeys("git", sshSecret.PrivateKey, "")
	if err != nil {
		return nil, err
	}
	keys.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }
	return keys, err
}

func (g gitSshAuthCred) name() string {
	return g.SecretName
}

type gitBasicAuthCred struct {
	fetchSecret func() (secret.BasicAuth, error)
	Domain      string
	SecretName  string
}

func (c gitBasicAuthCred) match(endpoint *transport.Endpoint) bool {
	if endpoint.Protocol != "http" && endpoint.Protocol != "https" {
		return false
	}
	return gitUrlMatch(endpoint.Host, c.Domain)
}

func (c gitBasicAuthCred) auth() (transport.AuthMethod, error) {
	basicAuthSecret, err := c.fetchSecret()
	if err != nil {
		return nil, err
	}

	return &http.BasicAuth{
		Username: basicAuthSecret.Username,
		Password: basicAuthSecret.Password,
	}, nil
}

func (c gitBasicAuthCred) name() string {
	return c.SecretName
}

func NewMountedSecretGitKeychain(volumeName string, basicAuthSecrets, sshAuthSecrets []string) (*secretGitKeychain, error) {
	var creds []gitCredential

	for _, s := range basicAuthSecrets {
		splitSecret := strings.Split(s, "=")
		if len(splitSecret) != 2 {
			return nil, errors.Errorf("could not parse git secret argument %s", s)
		}

		creds = append(creds, gitBasicAuthCred{
			Domain:     splitSecret[1],
			SecretName: splitSecret[0],
			fetchSecret: func() (secret.BasicAuth, error) {
				return secret.ReadBasicAuthSecret(volumeName, splitSecret[0])
			},
		})
	}
	for _, s := range sshAuthSecrets {
		splitSecret := strings.Split(s, "=")
		if len(splitSecret) != 2 {
			return nil, errors.Errorf("could not parse git secret argument %s", s)
		}

		creds = append(creds, gitSshAuthCred{
			Domain:     splitSecret[1],
			SecretName: splitSecret[0],
			fetchSecret: func() (secret.SSH, error) {
				return secret.ReadSshSecret(volumeName, splitSecret[0])
			},
		})
	}

	return &secretGitKeychain{
		creds: creds,
	}, nil
}

func (k *secretGitKeychain) Resolve(url string) (transport.AuthMethod, error) {
	endpoint, err := transport.NewEndpoint(url)
	if err != nil {
		return nil, err
	}
	sort.Slice(k.creds, func(i, j int) bool { return k.creds[i].name() < k.creds[j].name() })

	for _, cred := range k.creds {
		if cred.match(endpoint) {
			return cred.auth()
		}
	}
	return anonymousAuth, nil
}
