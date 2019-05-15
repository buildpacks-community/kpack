package registry_test

import (
	"testing"

	"github.com/buildpack/lifecycle/image/fakes"
	. "github.com/onsi/gomega"
	"github.com/sclevine/spec"

	"github.com/pivotal/build-service-system/pkg/registry"
	"github.com/pivotal/build-service-system/pkg/registry/registryfakes"
)

func TestMetadataRetriever(t *testing.T) {
	spec.Run(t, "Metadata Retriever", testMetadataRetriever)
}

func testMetadataRetriever(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect      func(interface{}, ...interface{}) GomegaAssertion
		mockFactory = &registryfakes.FakeFactory{}
	)

	it.Before(func() {
		Expect = NewGomegaWithT(t).Expect
	})

	when("RemoteMetadataRetriever", func() {
		when("retrieving from a builder image", func() {
			it("gets buildpacks from a local image", func() {
				fakeImage := fakes.NewImage(t, "packs/samples:v3alpha2", "topLayerSha", "digest")
				Expect(fakeImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`)).To(Succeed())

				imageRef := registry.NewNoAuthImageRef("test-repo-name")
				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := registry.RemoteMetadataRetriever{LifecycleImageFactory: mockFactory}
				metadata, err := subject.GetBuilderBuildpacks(imageRef)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(metadata)).To(Equal(1))
				Expect(metadata[0].ID).To(Equal("test.id"))
				Expect(metadata[0].Version).To(Equal("1.2.3"))
				Expect(mockFactory.NewRemoteArgsForCall(0)).To(Equal(imageRef))
			})
		})

		when("GetBuiltImage", func() {
			it("retrieves the metadata from the registry", func() {
				fakeImage := fakes.NewImage(t, "packs/samples:v3alpha2", "topLayerSha", "expected-image-digest")
				Expect(fakeImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"buildpacks": [{"key": "test.id", "version": "1.2.3"}]}`)).To(Succeed())

				fakeImageRef := registry.NewNoAuthImageRef("packs/samples:v3alpha2")
				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := registry.RemoteMetadataRetriever{LifecycleImageFactory: mockFactory}

				result, err := subject.GetBuiltImage(fakeImageRef)
				Expect(err).NotTo(HaveOccurred())
				metadata := result.BuildpackMetadata
				Expect(len(metadata)).To(Equal(1))
				Expect(metadata[0].ID).To(Equal("test.id"))
				Expect(metadata[0].Version).To(Equal("1.2.3"))
				createdAtTime, err := fakeImage.CreatedAt()
				Expect(err).To(BeNil())
				Expect(result.CompletedAt).To(Equal(createdAtTime))
				Expect(result.Sha).To(Equal("expected-image-digest"))
				Expect(mockFactory.NewRemoteArgsForCall(0)).To(Equal(fakeImageRef))
			})
		})
	})
}
