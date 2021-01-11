package buildpod_test

import (
	"fmt"
	"testing"

	"github.com/buildpacks/lifecycle"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestGenerator(t *testing.T) {
	spec.Run(t, "Generator", testGenerator)
}

func testGenerator(t *testing.T, when spec.G, it spec.S) {
	when("Generate", func() {
		const (
			serviceAccountName = "serviceAccountName"
			namespace          = "some-namespace"
		)

		var (
			keychainFactory = &registryfakes.FakeKeychainFactory{}
			imageFetcher    = registryfakes.NewFakeClient()
		)

		gitSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "git-secret-1",
				Namespace: namespace,
				Annotations: map[string]string{
					v1alpha1.GITSecretAnnotationPrefix: "https://github.com",
				},
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		}

		dockerSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "docker-secret-1",
				Namespace: namespace,
				Annotations: map[string]string{
					v1alpha1.DOCKERSecretAnnotationPrefix: "https://gcr.io",
				},
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		}

		ignoredSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ignored-secret",
				Namespace: namespace,
			},
			StringData: map[string]string{
				"username": "username",
				"password": "password",
			},
			Type: corev1.SecretTypeBasicAuth,
		}

		serviceAccount := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      serviceAccountName,
			},
			Secrets: []corev1.ObjectReference{
				{
					Kind: "secret",
					Name: "git-secret-1",
				},
				{
					Kind: "secret",
					Name: "docker-secret-1",
				},
			},
		}

		builderPullSecrets := []v1.LocalObjectReference{
			{
				Name: "some-builder-pull-secrets",
			},
		}

		fakeK8sClient := fake.NewSimpleClientset(serviceAccount, dockerSecret, gitSecret, ignoredSecret)

		it("returns pod config with secrets on build's service account", func() {
			secretRef := registry.SecretRef{
				ServiceAccount:   serviceAccountName,
				Namespace:        namespace,
				ImagePullSecrets: builderPullSecrets,
			}
			keychain := &registryfakes.FakeKeychain{}
			keychainFactory.AddKeychainForSecretRef(t, secretRef, keychain)

			image := randomImage(t)
			var err error

			config, err := image.ConfigFile()
			require.NoError(t, err)

			config.OS = "linux"
			image, err = mutate.ConfigFile(image, config)
			require.NoError(t, err)

			image, err = imagehelpers.SetStringLabel(image, lifecycle.StackIDLabel, "some.stack.id")
			require.NoError(t, err)

			image, err = imagehelpers.SetStringLabel(image, cnb.BuilderMetadataLabel, //language=json
				`{ "stack": { "runImage": { "image": "some-registry.io/run-image"} } }`)
			require.NoError(t, err)

			image, err = imagehelpers.SetStringLabel(image, cnb.BuilderMetadataLabel, //language=json
				`{
  "stack": {
    "runImage": {
      "image": "some-registry.io/run-image"
    }
  },
  "lifecycle": {
    "version": "0.9.0",
    "api": {
      "buildpack": "0.7",
      "platform": "0.5"
    }
  }
}`)
			require.NoError(t, err)

			image, err = imagehelpers.SetEnv(image, "CNB_USER_ID", "1234")
			require.NoError(t, err)

			image, err = imagehelpers.SetEnv(image, "CNB_GROUP_ID", "5678")
			require.NoError(t, err)

			imageFetcher.AddImage("some/builde@sha256:1b2911dd8eabb4bdb0bda6705158daa4149adb5ca59dc990146772c4c6deecb4", image, keychain)

			buildPodConfig := v1alpha1.BuildPodImages{}
			generator := &buildpod.Generator{
				BuildPodConfig:  buildPodConfig,
				K8sClient:       fakeK8sClient,
				KeychainFactory: keychainFactory,
				ImageFetcher:    imageFetcher,
			}

			var build = &testBuildPodable{
				serviceAccount: serviceAccountName,
				namespace:      namespace,
				buildBuilderSpec: v1alpha1.BuildBuilderSpec{
					Image:            "some/builde@sha256:1b2911dd8eabb4bdb0bda6705158daa4149adb5ca59dc990146772c4c6deecb4",
					ImagePullSecrets: builderPullSecrets,
				},
			}

			pod, err := generator.Generate(build)
			require.NoError(t, err)
			assert.NotNil(t, pod)

			assert.Equal(t, []buildPodCall{{
				BuildPodImages: buildPodConfig,
				Secrets: []corev1.Secret{
					*gitSecret,
					*dockerSecret,
				},
				BuildPodBuilderConfig: v1alpha1.BuildPodBuilderConfig{
					StackID:     "some.stack.id",
					RunImage:    "some-registry.io/run-image",
					Uid:         1234,
					Gid:         5678,
					PlatformAPI: "0.5",
					OS:          "linux",
				},
			}}, build.buildPodCalls)
		})

		it("rejects a build with a binding secret that is attached to a service account", func() {
			buildPodConfig := v1alpha1.BuildPodImages{}
			generator := &buildpod.Generator{
				BuildPodConfig:  buildPodConfig,
				K8sClient:       fakeK8sClient,
				KeychainFactory: keychainFactory,
				ImageFetcher:    imageFetcher,
			}

			var build = &testBuildPodable{
				namespace: namespace,
				bindings: []v1alpha1.Binding{
					{
						Name:        "naughty",
						MetadataRef: &corev1.LocalObjectReference{Name: "binding-configmap"},
						SecretRef:   &corev1.LocalObjectReference{Name: dockerSecret.Name},
					},
				},
			}

			pod, err := generator.Generate(build)
			require.EqualError(t, err, fmt.Sprintf("build rejected: binding %q uses forbidden secret %q", "naughty", dockerSecret.Name))
			require.Nil(t, pod)
		})
	})
}

func randomImage(t *testing.T) ggcrv1.Image {
	image, err := random.Image(5, 10)
	require.NoError(t, err)
	return image
}

type testBuildPodable struct {
	buildBuilderSpec v1alpha1.BuildBuilderSpec
	serviceAccount   string
	namespace        string
	buildPodCalls    []buildPodCall
	bindings         []v1alpha1.Binding
}

type buildPodCall struct {
	BuildPodImages        v1alpha1.BuildPodImages
	Secrets               []corev1.Secret
	BuildPodBuilderConfig v1alpha1.BuildPodBuilderConfig
}

func (tb *testBuildPodable) GetName() string {
	panic("should not be used in this test")
}

func (tb *testBuildPodable) GetNamespace() string {
	return tb.namespace
}

func (tb *testBuildPodable) ServiceAccount() string {
	return tb.serviceAccount
}

func (tb *testBuildPodable) BuilderSpec() v1alpha1.BuildBuilderSpec {
	return tb.buildBuilderSpec
}

func (tb *testBuildPodable) BuildPod(images v1alpha1.BuildPodImages, secrets []corev1.Secret, config v1alpha1.BuildPodBuilderConfig) (*corev1.Pod, error) {
	tb.buildPodCalls = append(tb.buildPodCalls, buildPodCall{
		BuildPodImages:        images,
		Secrets:               secrets,
		BuildPodBuilderConfig: config,
	})
	return &corev1.Pod{}, nil
}

func (tb *testBuildPodable) Bindings() []v1alpha1.Binding {
	return tb.bindings
}
