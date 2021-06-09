package secret

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

func ReadBasicAuthSecret(secretVolume, secretName string) (BasicAuth, error) {
	secretPath := volumeName(secretVolume, secretName)
	ub, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.BasicAuthUsernameKey))
	if err != nil {
		return BasicAuth{}, err
	}
	username := string(ub)

	pb, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.BasicAuthPasswordKey))
	if err != nil {
		return BasicAuth{}, err
	}
	password := string(pb)

	return BasicAuth{
		Username: username,
		Password: password,
	}, nil
}

func ReadSshSecret(secretVolume, secretName string) (SSH, error) {
	secretPath := volumeName(secretVolume, secretName)
	privateKey, err := ioutil.ReadFile(filepath.Join(secretPath, corev1.SSHAuthPrivateKey))
	if err != nil {
		return SSH{}, err
	}

	return SSH{
		PrivateKey: string(privateKey),
	}, nil
}

func volumeName(VolumePath, secretName string) string {
	return fmt.Sprintf("%s/%s", VolumePath, secretName)
}
