package secret_test

import (
	"testing"

	"github.com/pivotal/kpack/pkg/secret"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSigingSecret(t *testing.T) {
	spec.Run(t, "Test signing secrets", testSigingSecret)
}

func testSigingSecret(t *testing.T, when spec.G, it spec.S) {
	var (
		genericSecret1 = &corev1.Secret{
			TypeMeta:   metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{Name: "some-generic-secret-1"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"some-field": []byte("some-value"),
			},
		}
		genericSecret2 = &corev1.Secret{
			TypeMeta:   metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{Name: "some-generic-secret-2"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"some-other-field": []byte("some-other-value"),
			},
		}
		cosignSecret1 = &corev1.Secret{
			TypeMeta:   metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{Name: "some-cosign-secret-1"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"cosign.key":      []byte("some-key"),
				"cosign.pub":      []byte("some-key"),
				"cosign.password": []byte("some-password"),
			},
		}
		cosignSecret2 = &corev1.Secret{
			TypeMeta:   metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{Name: "some-cosign-secret-2"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"cosign.key":      []byte("some-other-key"),
				"cosign.pub":      []byte("some-other-key"),
				"cosign.password": []byte("some-other-password"),
			},
		}
		sshSecret = &corev1.Secret{
			TypeMeta:   metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{Name: "some-private-key-1"},
			Type:       corev1.SecretTypeSSHAuth,
			Data: map[string][]byte{
				"ssh-privatekey": []byte("some-private-key"),
			},
		}
		slsaSecret1 = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "some-private-key-3",
				Annotations: map[string]string{
					"kpack.io/slsa": "",
				},
			},
			Type: corev1.SecretTypeSSHAuth,
			Data: map[string][]byte{
				"ssh-privatekey": []byte("some-slsa-private-key"),
			},
		}
		slsaSecret2 = &corev1.Secret{
			TypeMeta: metav1.TypeMeta{Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{
				Name: "some-cosign-secret-3",
				Annotations: map[string]string{
					"kpack.io/slsa": "",
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"cosign.key":      []byte("some-slsa-cosign-key"),
				"cosign.pub":      []byte("some-slsa-cosign-key"),
				"cosign.password": []byte("some-slsa-cosign-password"),
			},
		}

		secrets = []*corev1.Secret{
			genericSecret1,
			cosignSecret1,
			slsaSecret1,
			sshSecret,
			slsaSecret2,
			cosignSecret2,
			genericSecret2,
		}
	)

	it("filters slsa secrets", func() {
		keys, err := secret.FilterAndExtractSLSASecrets(secrets)
		require.NoError(t, err)

		expected := []secret.SigningKey{
			{
				SecretName: "some-cosign-secret-3",
				Key:        []byte("some-slsa-cosign-key"),
				Password:   []byte("some-slsa-cosign-password"),
				Type:       secret.CosignKeyType,
			},
			{
				SecretName: "some-private-key-3",
				Key:        []byte("some-slsa-private-key"),
				Type:       secret.PKCS8KeyType,
			},
		}

		require.Equal(t, keys, expected)
	})

	it("filters cosign secrets", func() {
		actual := secret.FilterCosignSigningSecrets(secrets)

		require.Len(t, actual, 3)
		require.Contains(t, actual, cosignSecret1)
		require.Contains(t, actual, cosignSecret2)
		require.Contains(t, actual, slsaSecret2)
	})

}
