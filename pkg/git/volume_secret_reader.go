package git

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

type VolumeSecretReader struct {
}

func (v VolumeSecretReader) FromSecret(secretName string) (*BasicAuth, error) {
	secretPath := VolumeName(secretName)
	ub, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.BasicAuthUsernameKey))
	if err != nil {
		return nil, err
	}
	username := string(ub)

	pb, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.BasicAuthPasswordKey))
	if err != nil {
		return nil, err
	}
	password := string(pb)

	return &BasicAuth{
		Username: username,
		Password: password,
	}, nil
}

const VolumePath = "/var/build-secrets"

// VolumeName returns the full path to the secret, inside the VolumePath.
func VolumeName(secretName string) string {
	return fmt.Sprintf("%s/%s", VolumePath, secretName)
}
