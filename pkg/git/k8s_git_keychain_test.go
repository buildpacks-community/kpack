package git

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func Test(t *testing.T) {
	spec.Run(t, "Test Git Keychain", test)
}

func test(t *testing.T, when spec.G, it spec.S) {
	const serviceAccount = "some-service-account"
	const testNamespace = "test-namespace"

	var (
		fakeClient = fake.NewSimpleClientset(
			&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: testNamespace,
					Annotations: map[string]string{
						v1alpha1.GITSecretAnnotationPrefix: "https://github.com",
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
						v1alpha1.GITSecretAnnotationPrefix: "noschemegit.com",
					},
				},
				Type: v1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					v1.BasicAuthUsernameKey: []byte("noschemegit-username"),
					v1.BasicAuthPasswordKey: []byte("noschemegit-password"),
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
				},
			})
		keychain = newK8sGitKeychain(fakeClient)
	)

	when("Resolve", func() {
		it("returns git Auth for matching secrets", func() {
			auth, err := keychain.Resolve(testNamespace, serviceAccount, v1alpha1.Git{
				URL:      "https://github.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Equal(t, &http.BasicAuth{
				Username: "saved-username",
				Password: "saved-password",
			}, auth)
		})

		it("returns git Auth for matching secrets without scheme", func() {
			auth, err := keychain.Resolve(testNamespace, serviceAccount, v1alpha1.Git{
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
			auth, err := keychain.Resolve(testNamespace, serviceAccount, v1alpha1.Git{
				URL:      "https://no-creds-github.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Nil(t, auth)
		})
	})
}
