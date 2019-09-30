package cnb_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestMetadataRetriever(t *testing.T) {
	spec.Run(t, "Metadata Retriever", testMetadataRetriever)
}

func testMetadataRetriever(t *testing.T, when spec.G, it spec.S) {
	var (
		mockFactory = &registryfakes.FakeRemoteImageFactory{}
	)

	when("RemoteMetadataRetriever", func() {
		when("retrieving from a builder image", func() {
			var builder = &v1alpha1.Builder{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "builderNamespace",
				},
				Spec: v1alpha1.BuilderWithSecretsSpec{
					BuilderSpec: v1alpha1.BuilderSpec{
						Image: "builder/name",
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

			it("gets buildpacks from a local image", func() {
				fakeImage := registryfakes.NewFakeRemoteImage("index.docker.io/builder/image", "sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
				err := fakeImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`)
				assert.NoError(t, err)

				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: mockFactory}
				builderImage, err := subject.GetBuilderImage(builder)
				assert.NoError(t, err)

				require.Len(t, builderImage.BuilderBuildpackMetadata, 1)
				assert.Equal(t, builderImage.BuilderBuildpackMetadata[0].ID, "test.id")
				assert.Equal(t, builderImage.BuilderBuildpackMetadata[0].Version, "1.2.3")
				image, secretRef := mockFactory.NewRemoteArgsForCall(0)
				assert.Equal(t, image, "builder/name")
				assert.Equal(t, secretRef, registry.SecretRef{
					Namespace:        "builderNamespace",
					ImagePullSecrets: []string{"Secret-1", "Secret-2"},
				})

				assert.Equal(t, "index.docker.io/builder/image@sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895", builderImage.Identifier)
			})
		})

		when("GetBuiltImage", func() {
			var build = &v1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "namespace-name",
				},
				Spec: v1alpha1.BuildSpec{
					Tags:           []string{"image/name"},
					ServiceAccount: "service-account",
				},
				Status: v1alpha1.BuildStatus{},
			}

			it("retrieves the metadata from the registry", func() {
				fakeImage := registryfakes.NewFakeRemoteImage("index.docker.io/built/image", "sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4")
				err := fakeImage.SetLabel("io.buildpacks.build.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`)
				assert.NoError(t, err)

				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: mockFactory}

				result, err := subject.GetBuiltImage(build)
				assert.NoError(t, err)

				metadata := result.BuildpackMetadata
				require.Len(t, metadata, 1)
				assert.Equal(t, metadata[0].ID, "test.id")
				assert.Equal(t, metadata[0].Version, "1.2.3")

				createdAtTime, err := fakeImage.CreatedAt()
				assert.NoError(t, err)

				assert.Equal(t, result.CompletedAt, createdAtTime)
				assert.Equal(t, result.Identifier, "index.docker.io/built/image@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4")
				image, secretRef := mockFactory.NewRemoteArgsForCall(0)
				assert.Equal(t, image, "image/name")
				assert.Equal(t, secretRef, registry.SecretRef{
					ServiceAccount: "service-account",
					Namespace:      "namespace-name",
				})
			})
		})
	})
}
