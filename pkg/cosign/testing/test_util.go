package testing

import (
	"context"
	"crypto"
	"testing"

	cosignVerify "github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/signature"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/secret"
)

func GenerateFakeKeyPair(t *testing.T, secretName string, secretNamespace string, password string, annotations map[string]string) corev1.Secret {
	t.Helper()

	passFunc := func(_ bool) ([]byte, error) {
		return []byte(password), nil
	}

	keys, err := cosign.GenerateKeyPair(passFunc)
	require.NoError(t, err)

	data := map[string][]byte{
		secret.CosignSecretPublicKey:  keys.PublicBytes,
		secret.CosignSecretPrivateKey: keys.PrivateBytes,
		secret.CosignSecretPassword:   []byte(password),
	}

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        secretName,
			Namespace:   secretNamespace,
			Annotations: annotations,
		},
		Data: data,
	}

	return secret
}

func Verify(t *testing.T, keyRef, imageRef string, annotations map[string]interface{}) error {
	t.Helper()

	cmd := cosignVerify.VerifyCommand{
		KeyRef:        keyRef,
		Annotations:   signature.AnnotationsMap{Annotations: annotations},
		CheckClaims:   true,
		HashAlgorithm: crypto.SHA256,
		IgnoreTlog:    true,
	}

	args := []string{imageRef}

	return cmd.Exec(context.Background(), args)
}
