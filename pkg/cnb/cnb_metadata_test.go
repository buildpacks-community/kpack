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
		fakeFactory = registryfakes.NewFakeImageFactory()
	)

	when("RemoteMetadataRetriever", func() {
		when("retrieving from a builder image", func() {
			var (
				builder = &v1alpha1.Builder{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "builderNamespace",
					},
					Spec: v1alpha1.BuilderWithSecretsSpec{
						BuilderSpec: v1alpha1.BuilderSpec{
							Image: "builder/name",
							Stack: v1alpha1.BuilderStackSpec{
								RunImage: v1alpha1.RunImageSpec{},
							},
						},
						ImagePullSecrets: []v1.LocalObjectReference{
							{Name: "Secret-1"},
							{Name: "Secret-2"},
						},
					},
				}

				secretRef = registry.SecretRef{
					Namespace:        "builderNamespace",
					ImagePullSecrets: []v1.LocalObjectReference{{Name: "Secret-1"}, {Name: "Secret-2"}},
				}
			)

			it("gets buildpacks from a local image", func() {
				fakeBuilderImage := registryfakes.NewFakeRemoteImage("index.docker.io/builder/name", "sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
				assert.NoError(t, fakeBuilderImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}], "stack": { "runImage": { "image": "foo.io/run:basecnb" }}}`))
				assert.NoError(t, fakeBuilderImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stacks.bionic"))
				require.NoError(t, fakeFactory.AddImage(fakeBuilderImage, secretRef))

				fakeRunImage := registryfakes.NewFakeRemoteImage("foo.io/run:basecnb", "sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504")
				require.NoError(t, fakeFactory.AddImage(fakeRunImage, secretRef))

				subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: fakeFactory}
				builderImage, err := subject.GetBuilderImage(builder)
				assert.NoError(t, err)

				require.Len(t, builderImage.BuilderBuildpackMetadata, 1)
				assert.Equal(t, "test.id", builderImage.BuilderBuildpackMetadata[0].ID)
				assert.Equal(t, "1.2.3", builderImage.BuilderBuildpackMetadata[0].Version)
				assert.Equal(t, "index.docker.io/builder/name@sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895", builderImage.Identifier)
				assert.Equal(t, "foo.io/run:basecnb@sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504", builderImage.Stack.RunImage.Image)
				assert.Equal(t, "io.buildpacks.stacks.bionic", builderImage.Stack.ID)
			})

			when("there are run image mirrors", func() {
				builder.Spec.BuilderSpec.Stack.RunImage.Mirrors = []v1alpha1.Mirror{
					{Image: "gcr.io/some-example"},
					{Image: "index.docker.io/another-example"},
					{Image: "fake-registry.io/mismatched-digest"},
				}

				it("returns run image mirrors that match the run image digest", func() {
					fakeBuilderImage := registryfakes.NewFakeRemoteImage("index.docker.io/builder/name", "sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
					assert.NoError(t, fakeBuilderImage.SetLabel("io.buildpacks.builder.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}], "stack": { "runImage": { "image": "foo.io/run:basecnb" }}}`))
					assert.NoError(t, fakeBuilderImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stacks.bionic"))
					require.NoError(t, fakeFactory.AddImage(fakeBuilderImage, secretRef))

					fakeRunImage := registryfakes.NewFakeRemoteImage("foo.io/run:basecnb", "sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504")
					require.NoError(t, fakeFactory.AddImage(fakeRunImage, secretRef))

					mirrorRunImage1 := registryfakes.NewFakeRemoteImage("gcr.io/some-example", "sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504")
					require.NoError(t, fakeFactory.AddImage(mirrorRunImage1, secretRef))

					mirrorRunImage2 := registryfakes.NewFakeRemoteImage("index.docker.io/another-example", "sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504")
					require.NoError(t, fakeFactory.AddImage(mirrorRunImage2, secretRef))

					misMatchedRunImage := registryfakes.NewFakeRemoteImage("fake-registry.io/mismatched-digest", "sha256:b36ac0075433d709d538e8d138a057339b9d30211d68317ca46ed903d2027c85")
					require.NoError(t, fakeFactory.AddImage(misMatchedRunImage, secretRef))

					subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: fakeFactory}
					builder, err := subject.GetBuilderImage(builder)
					assert.NoError(t, err)

					assert.Equal(t, []v1alpha1.Mirror{
						{Image: "gcr.io/some-example@sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504"},
						{Image: "index.docker.io/another-example@sha256:c9d19ce90add2f415b859908952bc85afc0ee0aec012b3889cf5f2e9690bb504"},
					}, builder.Stack.RunImage.Mirrors)
				})
			})
		})

		when("GetBuiltImage", func() {
			var (
				build = &v1alpha1.Build{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "namespace-name",
					},
					Spec: v1alpha1.BuildSpec{
						Tags:           []string{"gcr.io/image/name"},
						ServiceAccount: "service-account",
					},
					Status: v1alpha1.BuildStatus{},
				}

				secretRef = registry.SecretRef{
					Namespace:      "namespace-name",
					ServiceAccount: "service-account",
				}
			)

			it("retrieves the metadata from the registry", func() {
				fakeImage := registryfakes.NewFakeRemoteImage("gcr.io/image/name", "sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4")
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.build.metadata", `{"buildpacks": [{"id": "test.id", "version": "1.2.3"}]}`))
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.lifecycle.metadata", `{"runImage":{"topLayer":"sha256:719f3f610dade1fdf5b4b2473aea0c6b1317497cf20691ab6d184a9b2fa5c409","reference":"localhost:5000/node@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0"},"stack":{"runImage":{"image":"gcr.io:443/run:full-cnb"}}}`))
				assert.NoError(t, fakeImage.SetLabel("io.buildpacks.stack.id", "io.buildpacks.stack.bionic"))
				require.NoError(t, fakeFactory.AddImage(fakeImage, secretRef))

				subject := cnb.RemoteMetadataRetriever{RemoteImageFactory: fakeFactory}

				result, err := subject.GetBuiltImage(build)
				assert.NoError(t, err)

				metadata := result.BuildpackMetadata
				require.Len(t, metadata, 1)
				assert.Equal(t, "test.id", metadata[0].ID)
				assert.Equal(t, "1.2.3", metadata[0].Version)

				createdAtTime, err := fakeImage.CreatedAt()
				assert.NoError(t, err)

				assert.Equal(t, createdAtTime, result.CompletedAt)
				assert.Equal(t, "gcr.io:443/run@sha256:0fd6395e4fe38a0c089665cbe10f52fb26fc64b4b15e672ada412bd7ab5499a0", result.Stack.RunImage.Image)
				assert.Equal(t, "io.buildpacks.stack.bionic", result.Stack.ID)
				assert.Equal(t, "gcr.io/image/name@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4", result.Identifier)
			})
		})
	})
}
