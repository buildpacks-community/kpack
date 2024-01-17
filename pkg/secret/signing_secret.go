package secret

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type KeyType int

const (
	CosignKeyType KeyType = iota
	PKCS8KeyType
)

type SigningKey struct {
	SecretName string
	Key        []byte
	Password   []byte
	Type       KeyType
}

func FilterCosignSigningSecrets(secrets []*corev1.Secret) []*corev1.Secret {
	return filterCosignSecrets(secrets, "")
}

func FilterAndExtractSLSASecrets(secrets []*corev1.Secret) ([]SigningKey, error) {
	cosignSecrets := filterCosignSecrets(secrets, SLSASecretAnnotation)
	privKeySecrets := filterPrivateKeySecrets(secrets, SLSASecretAnnotation)

	return extractAttestationKeyFromSecrets(cosignSecrets, privKeySecrets)
}

func filterCosignSecrets(serviceAccountSecrets []*corev1.Secret, annotation string) []*corev1.Secret {
	cosignSecrets := make([]*corev1.Secret, 0)

	for _, secret := range serviceAccountSecrets {
		_, passwordOk := secret.Data[CosignSecretPassword]
		_, keyOk := secret.Data[CosignSecretPrivateKey]
		_, annotationOk := secret.Annotations[annotation]

		if passwordOk && keyOk && (annotation == "" || annotationOk) {
			cosignSecrets = append(cosignSecrets, secret)
		}
	}

	return cosignSecrets
}

func filterPrivateKeySecrets(serviceAccountSecrets []*corev1.Secret, annotation string) []*corev1.Secret {
	secrets := make([]*corev1.Secret, 0)

	for _, secret := range serviceAccountSecrets {
		_, keyOk := secret.Data[PKCS8SecretKey]
		_, annotationOk := secret.Annotations[annotation]

		if keyOk && (annotation == "" || annotationOk) {
			secrets = append(secrets, secret)
		}
	}

	return secrets
}

func extractAttestationKeyFromSecrets(cosignSecrets []*corev1.Secret, pkcs8Secrets []*corev1.Secret) ([]SigningKey, error) {
	cosignKeys, err := getKey(cosignSecrets, CosignSecretPrivateKey, CosignSecretPassword, CosignKeyType)
	if err != nil {
		return nil, fmt.Errorf("getting cosign keys: %v", err)
	}

	pkcs8Keys, err := getKey(pkcs8Secrets, PKCS8SecretKey, "", PKCS8KeyType)
	if err != nil {
		return nil, fmt.Errorf("getting pkcs#8 keys: %v", err)
	}

	return append(cosignKeys, pkcs8Keys...), nil
}

func getKey(secrets []*corev1.Secret, keyField, passField string, keyType KeyType) ([]SigningKey, error) {
	privKeys := make([]SigningKey, 0)
	for _, s := range secrets {
		privKey, keyOk := s.Data[keyField]
		if !keyOk {
			return nil, fmt.Errorf("missing '%v' field: %v", keyField, s.Name)
		}
		password := s.Data[passField]

		privKeys = append(privKeys, SigningKey{
			SecretName: s.Name,
			Key:        privKey,
			Password:   password,
			Type:       keyType,
		})
	}
	return privKeys, nil
}
