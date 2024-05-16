package blob

import "fmt"

type Keychain interface {
	Resolve(url string) (authHeader string, headers map[string]string, err error)
}

var DefaultKeychain = NewMultiKeychain(
	azKeychain{},
	gcpKeychain{},
)

type multiKeychain struct {
	keychains []Keychain
}

func NewMultiKeychain(creds ...Keychain) Keychain {
	return &multiKeychain{creds}
}

func (m *multiKeychain) Resolve(url string) (string, map[string]string, error) {
	for _, helper := range m.keychains {
		t, h, err := helper.Resolve(url)
		if t != "" {
			return t, h, err
		}
	}
	return "", nil, fmt.Errorf("no keychain matched for '%v'", url)
}
