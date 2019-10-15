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
				fakeRunImage := registryfakes.NewFakeRemoteImage("foo.io/run", "sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504")
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}], "stack": { "runImage": { "image": "foo.io/run:basecnb" }}}`))
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stacks.bionic"))

				mockFactory.NewRemoteReturnsOnCall(0, fakeImage, nil)
				mockFactory.NewRemoteReturnsOnCall(1, fakeRunImage, nil)

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
					ImagePullSecrets: []v1.LocalObjectReference{{"Secret-1"}, {"Secret-2"}},
				})

				assert.Equal(t, "index.docker.io/builder/image@sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895", builderImage.Identifier)
				assert.Equal(t, "foo.io/run@sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504", builderImage.Stack.RunImage)
				assert.Equal(t, "io.buildpacks.stacks.bionic", builderImage.Stack.ID)
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
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.build.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`))
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"runImage":{"topLayer":"sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409","reference":"localhost:5000/node@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},"stack":{"runImage":{"image":"gcr.io:443/run:full-cnb"}}}`))
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stack.bionic"))

				mockFactory.NewRemoteReturns(fakeImage, nil)

				subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: mockFactory}

				result, err := subject.GetBuiltImage(build)
				assert.NoError(t, err)

				metadata := result.BuildpackMetadata
				require.Len(t, metadata, 1)
				assert.Equal(t, "test.id", metadata[0].ID)
				assert.Equal(t, "1.2.3", metadata[0].Version)

				createdAtTime, err := fakeImage.CreatedAt()
				assert.NoError(t, err)

				assert.Equal(t, createdAtTime, result.CompletedAt)
				assert.Equal(t, "gcr.io:443/run@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0", result.Stack.RunImage)
				assert.Equal(t, "io.buildpacks.stack.bionic", result.Stack.ID)
				assert.Equal(t, "index.docker.io/built/image@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4", result.Identifier)
				image, secretRef := mockFactory.NewRemoteArgsForCall(0)
				assert.Equal(t, "image/name", image)
				assert.Equal(t, registry.SecretRef{
					ServiceAccount: "service-account",
					Namespace:      "namespace-name",
				}, secretRef)
			})
		})
	})
}
