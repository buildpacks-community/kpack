package registry_test

import (
	"testing"

	"github.com/buildpack/lifecycle/image/fakes"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"

	"github.com/pivotal/build-service-system/pkg/registry"
	"github.com/pivotal/build-service-system/pkg/registry/registryfakes"
)

func TestMetadataRetriever(t *testing.T) {
	spec.Run(t, "Metadata Retriever", testMetadataRetriever)
}

func testMetadataRetriever(t *testing.T, when spec.G, it spec.S) {
	var (
		mockFactory = &registryfakes.FakeFactory{}
	)

	when("RemoteMetadataRetriever", func() {
		when("retrieving from a builder image", func() {
			it("gets buildpacks from a local image", func() {
				fakeImage := fakes.NewImage(t, "packs/samples:v3alpha2", "topLayerSha", "digest")
				err := fakeImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`)
				assert.NoError(t, err)

				imageRef := registry.NewNoAuthImageRef("test-repo-name")
				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := registry.RemoteMetadataRetriever{LifecycleImageFactory: mockFactory}
				metadata, err := subject.GetBuilderBuildpacks(imageRef)
				assert.NoError(t, err)

				assert.Len(t, metadata, 1)
				assert.Equal(t, metadata[0].ID, "test.id")
				assert.Equal(t, metadata[0].Version, "1.2.3")
				assert.Equal(t, mockFactory.NewRemoteArgsForCall(0), imageRef)
			})
		})

		when("GetBuiltImage", func() {
			it("retrieves the metadata from the registry", func() {
				fakeImage := fakes.NewImage(t, "packs/samples:v3alpha2", "topLayerSha", "expected-image-digest")
				err := fakeImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"buildpacks": [{"key": "test.id", "version": "1.2.3"}]}`)
				assert.NoError(t, err)

				fakeImageRef := registry.NewNoAuthImageRef("packs/samples:v3alpha2")
				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := registry.RemoteMetadataRetriever{LifecycleImageFactory: mockFactory}

				result, err := subject.GetBuiltImage(fakeImageRef)
				assert.NoError(t, err)

				metadata := result.BuildpackMetadata
				assert.Len(t, metadata, 1)
				assert.Equal(t, metadata[0].ID, "test.id")
				assert.Equal(t, metadata[0].Version, "1.2.3")

				createdAtTime, err := fakeImage.CreatedAt()
				assert.NoError(t, err)

				assert.Equal(t, result.CompletedAt, createdAtTime)
				assert.Equal(t, result.SHA, "expected-image-digest")
				assert.Equal(t, mockFactory.NewRemoteArgsForCall(0), fakeImageRef)
			})
		})
	})
}
