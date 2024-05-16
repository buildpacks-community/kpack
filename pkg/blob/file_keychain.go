package blob

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var errMultipleAuths = fmt.Errorf("only one of username/password, bearer, authorization is allowed")

type fileCredential struct {
	domain     string
	secretName string

	username      string
	password      string
	bearer        string
	authorization string
}

type fileCreds struct {
	creds []fileCredential
}

func NewMountedSecretBlobKeychain(volumeName string, secrets []string) (*fileCreds, error) {
	var creds []fileCredential
	for _, s := range secrets {
		splitSecret := strings.Split(s, "=")
		if len(splitSecret) != 2 {
			return nil, fmt.Errorf("could not parse blob secret argument %s", s)
		}

		dir := os.DirFS(filepath.Join(volumeName, splitSecret[0]))
		username, err := readFile(dir, "username")
		if err != nil {
			return nil, err
		}
		password, err := readFile(dir, "password")
		if err != nil {
			return nil, err
		}
		bearer, err := readFile(dir, "bearer")
		if err != nil {
			return nil, err
		}
		authorization, err := readFile(dir, "authorization")
		if err != nil {
			return nil, err
		}

		creds = append(creds, fileCredential{
			domain:     splitSecret[1],
			secretName: splitSecret[0],

			username:      username,
			password:      password,
			bearer:        bearer,
			authorization: authorization,
		})
	}
	return &fileCreds{creds}, nil
}

func readFile(dirFs fs.FS, filename string) (string, error) {
	_, err := fs.Stat(dirFs, filename)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		} else {
			return "", err
		}
	}

	buf, err := fs.ReadFile(dirFs, filename)
	return string(buf), err
}

func (f *fileCreds) Resolve(blobUrl string) (string, map[string]string, error) {
	u, err := url.Parse(blobUrl)
	if err != nil {
		return "", nil, fmt.Errorf("invalid url '%v': %v", blobUrl, u)
	}

	for _, cred := range f.creds {
		if u.Hostname() != cred.domain {
			continue
		}

		var authHeader []string
		if cred.username != "" || cred.password != "" {
			encoded := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%v:%v", cred.username, cred.password)))
			authHeader = append(authHeader, "Basic "+encoded)
		}

		if cred.bearer != "" {
			authHeader = append(authHeader, "Bearer "+cred.bearer)
		}

		if cred.authorization != "" {
			authHeader = append(authHeader, cred.authorization)
		}

		switch len(authHeader) {
		case 0:
			return "", nil, fmt.Errorf("no auths found for '%v'", cred.secretName)
		case 1:
			return authHeader[0], nil, nil
		default:
			return "", nil, fmt.Errorf("multiple auths found for '%v', only one of username/password, bearer, authorization is allowed", cred.secretName)
		}

	}
	return "", nil, fmt.Errorf("no secrets matched for '%v'", u.Hostname())
}
