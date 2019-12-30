package cnb_test

import (
	"testing"

	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestMetadataRetriever(t *testing.T) {
	spec.Run(t, "Metadata Retriever", testMetadataRetriever)
}

func testMetadataRetriever(t *testing.T, when spec.G, it spec.S) {
	var (
		keychainFactory = &registryfakes.FakeKeychainFactory{}
		imageFetcher    = registryfakes.NewFakeClient()
	)

	when("RemoteMetadataRetriever", func() {
		when("retrieving from a builder baseImage", func() {
			var builder = &v1alpha1.Builder{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "builderNamespace",
				},
				Spec: v1alpha1.BuilderWithSecretsSpec{
					BuilderSpec: v1alpha1.BuilderSpec{
						Image: "builder/image",
					},
					ImagePullSecrets: []v1.LocalObjectReference{
						{
							Name: "Secret-1",
						},
						{
							Name: "Secret-2",
						},
					},
				},
			}

			it("gets buildpacks from a local baseImage", func() {
				builderSecretRef := registry.SecretRef{
					Namespace:        builder.Namespace,
					ImagePullSecrets: builder.Spec.ImagePullSecrets,
				}
				builderKeychain := &registryfakes.FakeKeychain{}
				keychainFactory.AddKeychainForSecretRef(t, builderSecretRef, builderKeychain)

				builderImage := randomImage(t)
				builderImage, _ = imagehelpers.SetStringLabel(builderImage, "io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}], "stack": { "runImage": { "image": "foo.io/run:basecnb" }}}`)
				builderImage, _ = imagehelpers.SetStringLabel(builderImage, "io.buildpacks.stack.id", "io.buildpacks.stacks.bionic")
				imageFetcher.AddImage("builder/image", builderImage, builderKeychain)

				runImage := randomImage(t)
				imageFetcher.AddImage("foo.io/run:basecnb", runImage, builderKeychain)

				subject := cnb.RemoteMetadataRetriever{
					KeychainFactory: keychainFactory,
					ImageFetcher:    imageFetcher,
				}

				builderRecord, err := subject.GetBuilderImage(builder)
				assert.NoError(t, err)

				require.Len(t, builderRecord.Buildpacks, 1)
				assert.Equal(t, builderRecord.Buildpacks[0].ID, "test.id")
				assert.Equal(t, builderRecord.Buildpacks[0].Version, "1.2.3")

				digest, err := builderImage.Digest()
				require.NoError(t, err)
				assert.Equal(t, "index.docker.io/builder/image@"+digest.String(), builderRecord.Image)

				digest, err = runImage.Digest()
				require.NoError(t, err)
				assert.Equal(t, "foo.io/run@"+digest.String(), builderRecord.Stack.RunImage)

				assert.Equal(t, "io.buildpacks.stacks.bionic", builderRecord.Stack.ID)
			})
		})

		when("GetBuiltImage", func() {
			var build = &v1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-name",
				},
				Spec: v1alpha1.BuildSpec{
					Tags:           []string{"reg.io/appimage/name"},
					ServiceAccount: "service-account",
				},
				Status: v1alpha1.BuildStatus{},
			}

			it("retrieves the metadata from the registry", func() {
				appImageSecretRef := registry.SecretRef{
					ServiceAccount: build.Spec.ServiceAccount,
					Namespace:      build.Namespace,
				}
				appImageKeychain := &registryfakes.FakeKeychain{}
				keychainFactory.AddKeychainForSecretRef(t, appImageSecretRef, appImageKeychain)

				appImage := randomImage(t)
				appImage, _ = imagehelpers.SetStringLabel(appImage, "io.buildpacks.build.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`)
				appImage, _ = imagehelpers.SetStringLabel(appImage, "io.buildpacks.lifecycle.metadata", `{"runImage":{"topLayer":"sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409","reference":"localhost:5000/node@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},"stack":{"runImage":{"image":"gcr.io:443/run:full-cnb"}}}`)
				appImage, _ = imagehelpers.SetStringLabel(appImage, "io.buildpacks.stack.id", "io.buildpacks.stack.bionic")
				imageFetcher.AddImage("reg.io/appimage/name", appImage, appImageKeychain)

				subject := cnb.RemoteMetadataRetriever{
					KeychainFactory: keychainFactory,
					ImageFetcher:    imageFetcher,
				}

				result, err := subject.GetBuiltImage(build)
				assert.NoError(t, err)

				metadata := result.BuildpackMetadata
				require.Len(t, metadata, 1)
				assert.Equal(t, "test.id", metadata[0].ID)
				assert.Equal(t, "1.2.3", metadata[0].Version)

				createdAtTime, err := imagehelpers.GetCreatedAt(appImage)
				assert.NoError(t, err)

				assert.Equal(t, createdAtTime, result.CompletedAt)
				assert.Equal(t, "gcr.io:443/run@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0", result.Stack.RunImage)
				assert.Equal(t, "io.buildpacks.stack.bionic", result.Stack.ID)

				digest, err := appImage.Digest()
				require.NoError(t, err)
				assert.Equal(t, "reg.io/appimage/name@"+digest.String(), result.Identifier)
			})
		})
	})
}

func randomImage(t *testing.T) ggcrv1.Image {
	image, err := random.Image(5, 10)
	require.NoError(t, err)
	return image
}
