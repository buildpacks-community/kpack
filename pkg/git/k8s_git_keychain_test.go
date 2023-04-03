package git

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	ssh2 "golang.org/x/crypto/ssh"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestK8sGitKeychain(t *testing.T) {
	privateKeyBytes := gitTest{key1: generateRandomPrivateKey(t), key2: generateRandomPrivateKey(t)}
	spec.Run(t, "Test Git Keychain", privateKeyBytes.testK8sGitKeychain)
}

type gitTest struct {
	key1 []byte
	key2 []byte
}

func (keys gitTest) testK8sGitKeychain(t *testing.T, when spec.G, it spec.S) {
	const serviceAccount = "some-service-account"
	const testNamespace = "test-namespace"

	var (
		fakeClient = fake.NewSimpleClientset(
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: testNamespace,
					Annotations: map[string]string{
						buildapi.GITSecretAnnotationPrefix: "https://github.com",
					},
				},
				Type: v1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					v1.BasicAuthUsernameKey: []byte("saved-username"),
					v1.BasicAuthPasswordKey: []byte("saved-password"),
				},
			},
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-2",
					Namespace: testNamespace,
					Annotations: map[string]string{
						buildapi.GITSecretAnnotationPrefix: "noschemegit.com",
					},
				},
				Type: v1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					v1.BasicAuthUsernameKey: []byte("noschemegit-username"),
					v1.BasicAuthPasswordKey: []byte("noschemegit-password"),
				},
			},
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-3",
					Namespace: testNamespace,
					Annotations: map[string]string{
						buildapi.GITSecretAnnotationPrefix: "https://bitbucket.com",
					},
				},
				Type: v1.SecretTypeSSHAuth,
				Data: map[string][]byte{
					v1.SSHAuthPrivateKey: keys.key1,
				},
			},
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-5",
					Namespace: testNamespace,
					Annotations: map[string]string{
						buildapi.GITSecretAnnotationPrefix: "https://gitlab.com",
					},
				},
				Type: v1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					v1.BasicAuthUsernameKey: []byte("gitlab-username"),
					v1.BasicAuthPasswordKey: []byte("gitlab-password"),
				},
			},
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-6",
					Namespace: testNamespace,
					Annotations: map[string]string{
						buildapi.GITSecretAnnotationPrefix: "https://gitlab.com",
					},
				},
				Type: v1.SecretTypeSSHAuth,
				Data: map[string][]byte{
					v1.SSHAuthPrivateKey: keys.key2,
				},
			},
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-4",
					Namespace: testNamespace,
					Annotations: map[string]string{
						buildapi.GITSecretAnnotationPrefix: "https://gitlab.com",
					},
				},
				Type: v1.SecretTypeSSHAuth,
				Data: map[string][]byte{
					v1.SSHAuthPrivateKey: keys.key1,
				},
			},
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-7",
					Namespace: testNamespace,
					Annotations: map[string]string{
						buildapi.GITSecretAnnotationPrefix: "https://github.com",
					},
				},
				Type: v1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					v1.BasicAuthUsernameKey: []byte("other-username"),
					v1.BasicAuthPasswordKey: []byte("other-password"),
				},
			},
			&v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccount,
					Namespace: testNamespace,
				},
				Secrets: []v1.ObjectReference{
					{Name: "secret-1"},
					{Name: "secret-2"},
					{Name: "secret-4"},
					{Name: "secret-5"},
					{Name: "secret-3"},
					{Name: "secret-6"},
					{Name: "secret-7"},
				},
			})
		keychain = newK8sGitKeychain(fakeClient)
	)

	when("K8s Keychain resolves", func() {

		it("returns alphabetical first git Auth for matching secrets with basic auth", func() {
			auth, err := keychain.Resolve(context.Background(), testNamespace, serviceAccount, v1alpha1.Git{
				URL:      "https://github.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Equal(t, &http.BasicAuth{
				Username: "saved-username",
				Password: "saved-password",
			}, auth)
		})

		it("returns the alphabetical first secretRef for ssh auth", func() {
			auth, err := keychain.Resolve(context.Background(), testNamespace, serviceAccount, v1alpha1.Git{
				URL:      "git@gitlab.com:org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			publicKeys, ok := auth.(*ssh.PublicKeys)
			require.True(t, ok)

			require.Equal(t, "git", publicKeys.User)

			expectedSigner, err := ssh2.ParsePrivateKey(keys.key1)
			require.NoError(t, err)
			require.Equal(t, expectedSigner, publicKeys.Signer)
		})

		it("returns git Auth for matching secrets with ssh auth", func() {
			auth, err := keychain.Resolve(context.Background(), testNamespace, serviceAccount, v1alpha1.Git{
				URL:      "git@bitbucket.com:org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			publicKeys, ok := auth.(*ssh.PublicKeys)
			require.True(t, ok)

			require.Equal(t, "git", publicKeys.User)

			signer, err := ssh2.ParsePrivateKey(keys.key1)
			require.NoError(t, err)
			require.Equal(t, signer, publicKeys.Signer)
		})

		it("returns git Auth for matching secrets without scheme", func() {
			auth, err := keychain.Resolve(context.Background(), testNamespace, serviceAccount, v1alpha1.Git{
				URL:      "https://noschemegit.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Equal(t, &http.BasicAuth{
				Username: "noschemegit-username",
				Password: "noschemegit-password",
			}, auth)
		})

		it("returns anonymous Auth for no matching secret", func() {
			auth, err := keychain.Resolve(context.Background(), testNamespace, serviceAccount, v1alpha1.Git{
				URL:      "https://no-creds-github.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)
			require.Nil(t, auth)
		})
	})
}

func generateRandomPrivateKey(t *testing.T) []byte {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	require.NoError(t, err)
	var pemBlock = &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(pemBlock)
}
