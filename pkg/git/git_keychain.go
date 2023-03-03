package git

import (
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"sort"
	"strings"

	"github.com/pkg/errors"
	giturls "github.com/whilp/git-urls"
	"golang.org/x/crypto/ssh"

	"github.com/pivotal/kpack/pkg/secret"
)

type CredentialType string

const (
	CredentialTypeUserpass  CredentialType = "userpass"
	CredentialTypeSSHKey    CredentialType = "sshkey"
	CredentialTypeSSHCustom CredentialType = "sshcustom"
	CredentialTypeDefault   CredentialType = "default"
	CredentialTypeUsername  CredentialType = "username"
	CredentialTypeSSHMemory CredentialType = "sshmemory"
)

type GoGitCredential interface {
	Cred() (transport.AuthMethod, error)
}

type GoGitSshCredential struct {
	GoGitCredential
	User       string
	Signer     ssh.Signer
	PrivateKey []byte
}

type GoGitHttpCredential struct {
	GoGitCredential
	User     string
	Password string
}

//func keychainAsCredentialsCallback(gitKeychain GitKeychain) git2go.CredentialsCallback {
//	return func(url string, usernameFromUrl string, allowedTypes git2go.CredentialType) (*git2go.Credential, error) {
//		cred, err := gitKeychain.Resolve(url, usernameFromUrl, allowedTypes)
//		if err != nil {
//			return nil, err
//		}
//		return cred.Cred()
//	}
//}

type GitKeychain interface {
	Resolve(url string, usernameFromUrl string, allowedTypes CredentialType) (GoGitCredential, error)
	//AuthForUrl(url string) (transport.AuthMethod, error)
}

func (kc *secretGitKeychain) AuthForUrl(url string) (transport.AuthMethod, error) {
	cred, err := kc.Resolve(url, "", CredentialTypeSSHKey)
	if err != nil {
		return nil, err
	}
	auth, err := cred.Cred()
	if err != nil {
		return nil, err
	}
	return auth, nil
}

func (c *GoGitHttpCredential) Cred() (transport.AuthMethod, error) {
	return &http.BasicAuth{
		Username: c.User,
		Password: c.Password,
	}, nil
}

func (c *GoGitSshCredential) Cred() (transport.AuthMethod, error) {
	signer, err := ssh.ParsePrivateKey([]byte(c.PrivateKey))
	if err != nil {
		return nil, err
	}
	return &gitssh.PublicKeys{User: c.User, Signer: signer}, nil
}

type gitCredential interface {
	match(host string, allowedTypes CredentialType) bool
	goGitCredential(username string) (GoGitCredential, error)
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

func (g gitSshAuthCred) goGitCredential(username string) (GoGitCredential, error) {
	sshSecret, err := g.fetchSecret()
	if err != nil {
		return nil, err
	}

	return &GoGitSshCredential{
		User:       username,
		PrivateKey: []byte(sshSecret.PrivateKey),
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

func (g gitSshAuthCred) match(host string, allowedTypes CredentialType) bool {
	if allowedTypes != CredentialTypeSSHKey {
		return false
	}
	//fmt.Printf("gitSshAuthCred.match: host=%s, g.Domain=%s\n", host, g.Domain)
	return gitUrlMatch(host, g.Domain)
}

func (c gitBasicAuthCred) match(host string, allowedTypes CredentialType) bool {
	if allowedTypes != CredentialTypeUserpass {
		return false
	}
	//fmt.Printf("gitSshAuthCred.match: host=%s, g.Domain=%s\n", host, c.Domain)
	return gitUrlMatch(host, c.Domain)
}

func (c gitBasicAuthCred) goGitCredential(_ string) (GoGitCredential, error) {
	basicAuthSecret, err := c.fetchSecret()
	if err != nil {
		return nil, err
	}

	return &GoGitHttpCredential{
		User:     basicAuthSecret.Username,
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

// Resolve takes in a URL, username and allowedTypes as input and returns a GoGitCredential that matches the input
func (k *secretGitKeychain) Resolve(url string, username string, allowedTypes CredentialType) (GoGitCredential, error) {
	//fmt.Printf("secretGitKeychain::Resolve url->%s, username->%s, allowedTypes->%s\n", url, username, allowedTypes)
	u, err := giturls.Parse(url)
	if err != nil {
		return nil, err
	}

	if username == "" {
		username = u.User.Username()
	}

	sort.Slice(k.creds, func(i, j int) bool { return k.creds[i].name() < k.creds[j].name() })

	//fmt.Printf("secretGitKeychain::Resolve number of creds: %d\n", len(k.creds))
	for _, cred := range k.creds {
		//fmt.Printf("secretGitKeychain::Resolve %s\n", cred.name())
		if cred.match(u.Host, allowedTypes) {
			return cred.goGitCredential(username)
		}
	}

	return nil, errors.Errorf("no credentials found for %s", url)
}
