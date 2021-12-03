package git

import (
	"sort"
	"strings"

	git2go "github.com/libgit2/git2go/v31"
	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
	"golang.org/x/crypto/ssh"

	"github.com/pivotal/kpack/pkg/secret"
)

type Git2GoCredential interface {
	Cred() (*git2go.Credential, error)
}

func keychainAsCredentialsCallback(gitKeychain GitKeychain) git2go.CredentialsCallback {
	return func(url string, usernameFromUrl string, allowedTypes git2go.CredentialType) (*git2go.Credential, error) {
		cred, err := gitKeychain.Resolve(url, usernameFromUrl, allowedTypes)
		if err != nil {
			return nil, err
		}
		return cred.Cred()
	}
}

type GitKeychain interface {
	Resolve(url string, usernameFromUrl string, allowedTypes git2go.CredentialType) (Git2GoCredential, error)
}

type BasicGit2GoAuth struct {
	Username, Password string
}

func (b BasicGit2GoAuth) Cred() (*git2go.Credential, error) {
	return git2go.NewCredentialUserpassPlaintext(b.Username, b.Password)
}

type SSHGit2GoAuth struct {
	Username, PrivateKey string
}

func (s SSHGit2GoAuth) Cred() (*git2go.Credential, error) {
	signer, err := ssh.ParsePrivateKey([]byte(s.PrivateKey))
	if err != nil {
		return nil, err
	}

	return git2go.NewCredentialSSHKeyFromSigner(s.Username, signer)
}

type gitCredential interface {
	match(host string, allowedTypes git2go.CredentialType) bool
	git2goCredential(username string) (Git2GoCredential, error)
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

func (g gitSshAuthCred) match(host string, allowedTypes git2go.CredentialType) bool {
	if allowedTypes&(git2go.CredentialTypeSSHKey) == 0 {
		return false
	}

	return gitUrlMatch(host, g.Domain)
}

func (g gitSshAuthCred) git2goCredential(username string) (Git2GoCredential, error) {
	sshSecret, err := g.fetchSecret()
	if err != nil {
		return nil, err
	}

	return SSHGit2GoAuth{
		Username:   username,
		PrivateKey: sshSecret.PrivateKey,
	}, nil
}

func (g gitSshAuthCred) name() string {
	return g.SecretName
}

type gitBasicAuthCred struct {
	fetchSecret func() (secret.BasicAuth, error)
	Domain      string
	SecretName  string
}

func (c gitBasicAuthCred) match(host string, allowedTypes git2go.CredentialType) bool {
	if allowedTypes&(git2go.CredentialTypeUserpassPlaintext) == 0 {
		return false
	}

	return gitUrlMatch(host, c.Domain)
}

func (c gitBasicAuthCred) git2goCredential(_ string) (Git2GoCredential, error) {
	basicAuthSecret, err := c.fetchSecret()
	if err != nil {
		return nil, err
	}

	return BasicGit2GoAuth{Username: basicAuthSecret.Username, Password: basicAuthSecret.Password}, nil
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

func (k *secretGitKeychain) Resolve(url string, username string, allowedTypes git2go.CredentialType) (Git2GoCredential, error) {
	u, err := giturls.Parse(url)
	if err != nil {
		return nil, err
	}

	sort.Slice(k.creds, func(i, j int) bool { return k.creds[i].name() < k.creds[j].name() })

	for _, cred := range k.creds {
		if cred.match(u.Host, allowedTypes) {
			return cred.git2goCredential(username)
		}
	}
	return nil, errors.Errorf("no credentials found for %s", url)
}
