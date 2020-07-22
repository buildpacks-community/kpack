package s3

import (
	"github.com/pivotal/kpack/pkg/secret"
	"github.com/pkg/errors"
)

// Credentials Represents the S3 Credentials
type Credentials struct {
	AccessKey string
	SecretKey string
}

// ParseMountedCredentialsSecret Gets the S3 Credentials from secret
func ParseMountedCredentialsSecret(volumeName, secretName string) (Credentials, error) {
	var creds = Credentials{}
	auth, err := secret.ReadOpaqueSecret(volumeName, secretName, []string{"accesskey", "secretkey"})
	if err != nil {
		return creds, errors.Errorf("Error reading secret %s at %s", secretName, volumeName)
	}

	if auth.StringData["accesskey"] == "" {
		return creds, errors.Errorf("accesskey is empty")
	}

	if auth.StringData["secretkey"] == "" {
		return creds, errors.Errorf("secretkey is empty")
	}

	creds.AccessKey = auth.StringData["accesskey"]
	creds.SecretKey = auth.StringData["secretkey"]

	return creds, nil
}
