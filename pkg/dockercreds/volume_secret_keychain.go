package dockercreds

import (
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
)

const (
	SecretFilePathEnv = "CREDENTIAL_PROVIDER_SECRET_PATH"
)

func NewVolumeSecretKeychain() (authn.Keychain, error) {
	secretFolder, ok := os.LookupEnv(SecretFilePathEnv)
	if !ok {
		return DockerCreds{}, nil
	}

	secrets, err := ParseDockerPullSecrets(secretFolder)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}
