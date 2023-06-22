package git

import (
	"net/url"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
	"golang.org/x/crypto/ssh"

	"github.com/pivotal/kpack/pkg/secret"
)

type GitKeychain interface {
	Resolve(url string) (transport.AuthMethod, error)
}

type gitCredential interface {
	match(url *url.URL) bool
	auth() (transport.AuthMethod, error)
	name() string
}

type secretGitKeychain struct {
	creds []gitCredential
}

type gitSshAuthCred struct {
	fetchSecret          func() (secret.SSH, error)
	Domain               string
	SecretName           string
	sshTrustUnknownHosts bool
}

func (g gitSshAuthCred) auth() (transport.AuthMethod, error) {
	sshSecret, err := g.fetchSecret()
	if err != nil {
		return nil, err
	}

	keys, err := gitssh.NewPublicKeys("git", []byte(sshSecret.PrivateKey), "")
	if err != nil {
		return nil, err
	}

	if g.sshTrustUnknownHosts {
		keys.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		knownHosts, err := gitssh.NewKnownHostsCallback(sshSecret.KnownHostsFile...)
		if err != nil {
			return nil, err
		}
		keys.HostKeyCallback = knownHosts
	}

	return keys, nil
}

func (g gitSshAuthCred) match(url *url.URL) bool {
	return url.Scheme == "ssh" && gitUrlMatch(url.Host, g.Domain)
}

func (g gitSshAuthCred) name() string {
	return g.SecretName
}

type gitBasicAuthCred struct {
	fetchSecret func() (secret.BasicAuth, error)
	Domain      string
	SecretName  string
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

func (c gitBasicAuthCred) match(url *url.URL) bool {
	return (url.Scheme == "http" || url.Scheme == "https") && gitUrlMatch(url.Host, c.Domain)
}

func (c gitBasicAuthCred) name() string {
	return c.SecretName
}

func NewMountedSecretGitKeychain(volumeName string, basicAuthSecrets, sshAuthSecrets []string, sshTrustUnknownHosts bool) (*secretGitKeychain, error) {
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
			sshTrustUnknownHosts: sshTrustUnknownHosts,
		})
	}

	return &secretGitKeychain{
		creds: creds,
	}, nil
}

func (k *secretGitKeychain) Resolve(rawUrl string) (transport.AuthMethod, error) {
	parsedUrl, err := giturls.Parse(rawUrl)
	if err != nil {
		return nil, err
	}

	sort.Slice(k.creds, func(i, j int) bool { return k.creds[i].name() < k.creds[j].name() })

	for _, cred := range k.creds {
		if cred.match(parsedUrl) {
			return cred.auth()
		}
	}

	return anonymousAuth, nil
}
