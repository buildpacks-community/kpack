package secret_test

import (
	"strings"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/build-service-system/pkg/secret"
)

func TestSecretManagerFactory(t *testing.T) {
	spec.Run(t, "SecretManager", testSecretManager)
}
func testSecretManager(t *testing.T, when spec.G, it spec.S) {
	const (
		namespace  = "some-namespace"
		secretName = "some-secret-name"
	)
	var (
		fakeClient = fake.NewSimpleClientset(&v1.Secret{})

		subject = secret.SecretManager{
			Client:  fakeClient,
			Matcher: fakeMatcher{},
		}
	)

	when("ImagePull Secret", func() {
		it("retrieves the secret from dockerconfigjson", func() {
			_, err := fakeClient.CoreV1().Secrets(namespace).Create(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					v1.DockerConfigJsonKey: []byte(`{ "auths": { "some-registry": { "auth": "some-base64-secret" }, "some-other-registry": { "auth": "some-base64-secret" } } }`),
				},
				Type: v1.SecretTypeDockerConfigJson,
			})
			require.NoError(t, err)

			auth, err := subject.SecretForImagePull(namespace, secretName, "some-registry")
			require.NoError(t, err)
			assert.Equal(t, "some-base64-secret", auth)
		})

		it("retrieves the secret from dockercfg", func() {
			_, err := fakeClient.CoreV1().Secrets(namespace).Create(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					v1.DockerConfigKey: []byte(`{ "some-registry": { "auth": "some-base64-secret" }, "some-other-registry": { "auth": "some-base64-secret" } }`),
				},
				Type: v1.SecretTypeDockercfg,
			})
			require.NoError(t, err)

			auth, err := subject.SecretForImagePull(namespace, secretName, "some-registry")
			require.NoError(t, err)
			assert.Equal(t, "some-base64-secret", auth)
		})

		it("errors when registry secret is not available from dockerconfigjson", func() {
			_, err := fakeClient.CoreV1().Secrets(namespace).Create(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					v1.DockerConfigJsonKey: []byte(`{ "auths": { "some-registry": { "auth": "some-base64-secret" } } }`),
				},
				Type: v1.SecretTypeDockerConfigJson,
			})
			require.NoError(t, err)

			_, err = subject.SecretForImagePull(namespace, secretName, "some-other-registry")
			assert.EqualError(t, err, "no secret configuration for registry: some-other-registry")
		})

		it("errors when registry secret is not available from dockercfg", func() {
			_, err := fakeClient.CoreV1().Secrets(namespace).Create(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: secretName,
				},
				Data: map[string][]byte{
					v1.DockerConfigKey: []byte(`{ "some-registry": { "auth": "some-base64-secret" } }`),
				},
				Type: v1.SecretTypeDockercfg,
			})
			require.NoError(t, err)

			_, err = subject.SecretForImagePull(namespace, secretName, "some-other-registry")
			assert.EqualError(t, err, "no secret configuration for registry: some-other-registry")
		})
	})
}

type fakeMatcher struct {
}

func (fakeMatcher) Match(url, annotatedUrl string) bool {
	return strings.Contains(annotatedUrl, url)
}
