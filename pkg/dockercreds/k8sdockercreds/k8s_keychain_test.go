package k8sdockercreds

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestK8sSecretKeychainFactory(t *testing.T) {
	spec.Run(t, "k8sSecretKeychainFactory", testK8sSecretKeychainFactory)
}

func testK8sSecretKeychainFactory(t *testing.T, when spec.G, it spec.S) {
	const (
		serviceAccountName = "some-service-account"
		testNamespace      = "test-namespace"
	)

	when("#KeychainForSecretRef", func() {
		it("keychain provides auth from annotated basic auth secrets", func() {
			fakeClient := fake.NewSimpleClientset(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: testNamespace,
					Annotations: map[string]string{
						v1alpha1.DOCKERSecretAnnotationPrefix: "annotated.io",
					},
				},
				Type: v1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					v1.BasicAuthUsernameKey: []byte("annotated-username"),
					v1.BasicAuthPasswordKey: []byte("annotated-password"),
				},
			},
				&v1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceAccountName,
						Namespace: testNamespace,
					},
					Secrets: []v1.ObjectReference{
						{Name: "secret-1"},
					},
				})
			keychainFactory, err := NewSecretKeychainFactory(fakeClient)
			require.NoError(t, err)

			keychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{
				ServiceAccount: serviceAccountName,
				Namespace:      testNamespace,
			})

			reg, err := name.NewRegistry("annotated.io")
			require.NoError(t, err)

			authenticator, err := keychain.Resolve(reg)
			require.NoError(t, err)

			assert.Equal(t, authn.FromConfig(authn.AuthConfig{
				Username: "annotated-username",
				Password: "annotated-password",
			}), authenticator)
		})

		it("keychain provides auth from dockerconfigjson and dockercfg secrets", func() {
			fakeClient := fake.NewSimpleClientset(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: testNamespace,
				},
				Type: v1.SecretTypeDockercfg,
				Data: map[string][]byte{
					v1.DockerConfigKey: []byte("{\"imagcfg.io\": {\"username\": \"cfg-user\", \"password\":  \"pull-password\"}}"),
				},
			},
				&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: testNamespace,
					},
					Type: v1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{
						v1.DockerConfigJsonKey: []byte("{\"auths\": {\"imagecfgjson.io\": {\"username\": \"config-json-user\", \"password\":  \"pull-password\"}}}"),
					},
				},
				&v1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceAccountName,
						Namespace: testNamespace,
					},
					Secrets: []v1.ObjectReference{
						{Name: "secret-1"},
						{Name: "secret-2"},
					},
				})
			keychainFactory, err := NewSecretKeychainFactory(fakeClient)
			require.NoError(t, err)

			keychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{
				ServiceAccount: serviceAccountName,
				Namespace:      testNamespace,
			})

			dockerCfgReg, err := name.NewRegistry("imagcfg.io")
			require.NoError(t, err)

			dockerCfgAuth, err := keychain.Resolve(dockerCfgReg)
			require.NoError(t, err)

			dockerCfgAuthConfig, err := dockerCfgAuth.Authorization()
			require.NoError(t, err)

			assert.Equal(t, &authn.AuthConfig{
				Username: "cfg-user",
				Password: "pull-password",
			}, dockerCfgAuthConfig)

			dockerConfigReg, err := name.NewRegistry("imagecfgjson.io")
			require.NoError(t, err)

			dockerConfigAuth, err := keychain.Resolve(dockerConfigReg)
			require.NoError(t, err)

			dockerConfigAuthCfg, err := dockerConfigAuth.Authorization()
			require.NoError(t, err)

			assert.Equal(t, &authn.AuthConfig{
				Username: "config-json-user",
				Password: "pull-password",
			}, dockerConfigAuthCfg)
		})

		when("no service account is provided", func() {
			it("keychain provides auth from annotated basic auth secrets from the default service account", func() {
				fakeClient := fake.NewSimpleClientset(&v1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-1",
						Namespace: testNamespace,
						Annotations: map[string]string{
							v1alpha1.DOCKERSecretAnnotationPrefix: "annotated.io",
						},
					},
					Type: v1.SecretTypeBasicAuth,
					Data: map[string][]byte{
						v1.BasicAuthUsernameKey: []byte("annotated-username"),
						v1.BasicAuthPasswordKey: []byte("annotated-password"),
					},
				},
					&v1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "non-default",
							Namespace: testNamespace,
						},
					},
					&v1.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "default",
							Namespace: testNamespace,
						},
						Secrets: []v1.ObjectReference{
							{Name: "secret-1"},
						},
					})
				keychainFactory, err := NewSecretKeychainFactory(fakeClient)
				require.NoError(t, err)

				keychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{
					ServiceAccount: "",
					Namespace:      testNamespace,
				})

				reg, err := name.NewRegistry("annotated.io")
				require.NoError(t, err)

				authenticator, err := keychain.Resolve(reg)
				require.NoError(t, err)

				assert.Equal(t, authn.FromConfig(authn.AuthConfig{
					Username: "annotated-username",
					Password: "annotated-password",
				}), authenticator)
			})
		})

		it("keychain provides auth from ImagePull secrets", func() {
			fakeClient := fake.NewSimpleClientset(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "image-pull-secret",
					Namespace: testNamespace,
				},
				Type: v1.SecretTypeDockerConfigJson,
				Data: map[string][]byte{
					v1.DockerConfigJsonKey: []byte("{\"auths\": {\"imagepull.io\": {\"username\": \"image-pull-user\", \"password\":  \"image-pull-password\"}}}"),
				},
			},
				&v1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceAccountName,
						Namespace: testNamespace,
					},
				})
			keychainFactory, err := NewSecretKeychainFactory(fakeClient)
			require.NoError(t, err)

			keychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{
				ServiceAccount:   serviceAccountName,
				Namespace:        testNamespace,
				ImagePullSecrets: []v1.LocalObjectReference{{"image-pull-secret"}},
			})
			require.NoError(t, err)

			reg, err := name.NewRegistry("imagepull.io")
			require.NoError(t, err)

			authenticator, err := keychain.Resolve(reg)
			require.NoError(t, err)

			authConfig, err := authenticator.Authorization()
			require.NoError(t, err)

			assert.Equal(t, &authn.AuthConfig{
				Username: "image-pull-user",
				Password: "image-pull-password",
			}, authConfig)
		})

		it("keychain provides Anonymous auth for no matching credentials", func() {
			keychainFactory, err := NewSecretKeychainFactory(fake.NewSimpleClientset(&v1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: testNamespace,
				},
			}))
			require.NoError(t, err)

			keychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{
				ServiceAccount: serviceAccountName,
				Namespace:      testNamespace,
			})
			require.NoError(t, err)

			reg, err := name.NewRegistry("nosecret.io")
			require.NoError(t, err)
			auth, err := keychain.Resolve(reg)
			require.NoError(t, err)

			assert.Equal(t, auth, authn.Anonymous)
		})

		it("returns an empty k8schain when no namespace is provided to leverage k8s.io/kubernetes/pkg/credentialprovider", func() {
			keychainFactory, err := NewSecretKeychainFactory(fake.NewSimpleClientset())
			require.NoError(t, err)

			keychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{
				Namespace: "",
			})
			require.NoError(t, err)

			k8schain, err := k8schain.New(nil, k8schain.Options{})
			require.NoError(t, err)
			volumeKeyChain := dockercreds.DockerCreds{}
			expected := authn.NewMultiKeychain(volumeKeyChain, k8schain)
			require.NoError(t, err)
			assert.Equal(t, expected, keychain)
		})
	})
}
