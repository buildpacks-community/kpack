package dockercreds

import (
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
)

const (
	SecretFilePathEnv  = "CREDENTIAL_PROVIDER_SECRET_PATH" // historically only support one path
	SecretFilePathsEnv = "CREDENTIAL_PROVIDER_SECRET_PATHS"
)

func NewVolumeSecretsKeychain() (authn.Keychain, error) {
	secretFolders := getSecretFolders()
	secrets := DockerCreds{}
	for _, secretFolder := range secretFolders {
		newSecrets, err := ParseDockerPullSecrets(secretFolder)
		if err != nil {
			return nil, err
		}
		secrets.Append(newSecrets)
	}

	return secrets, nil
}

func getSecretFolders() []string {
	secretFoldersEnvVal, ok := os.LookupEnv(SecretFilePathsEnv)
	if !ok {
		secretFolderEnvVal, ok := os.LookupEnv(SecretFilePathEnv)
		if !ok {
			return []string{}
		}
		return []string{secretFolderEnvVal}
	}

	return strings.Split(secretFoldersEnvVal, ",")
}
