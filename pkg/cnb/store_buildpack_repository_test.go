package cnb

import (
	"errors"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

func TestBuildpackRepository(t *testing.T) {
	spec.Run(t, "TestBuildpackRepository", testBuildpackRetriever)
}

func testBuildpackRetriever(t *testing.T, when spec.G, it spec.S) {
	when("FindByIdAndVersion", func() {
		engineLayer := fakeLayer{
			diffID: "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   10,
		}

		packageManagerLayer := fakeLayer{
			diffID: "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   10,
		}

		metaLayer := fakeLayer{
			diffID: "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   10,
		}

		v8Layer := fakeLayer{
			diffID: "sha256:8bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   10,
		}

		v9Layer := fakeLayer{
			diffID: "sha256:9bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   10,
		}

		image := &fakeStoreImage{
			layersByDiffId: []v1.Layer{engineLayer, packageManagerLayer, metaLayer, v8Layer, v9Layer},
		}

		client := registryfakes.NewFakeClient()
		client.AddImage("some.registry.io/build-package", image, "", nil)

		subject := &StoreBuildpackRepository{
			Keychain:       nil,
			RegistryClient: client,
			Store: &v1alpha1.Store{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-store",
				},
				Status: v1alpha1.StoreStatus{
					Buildpacks: []v1alpha1.StoreBuildpack{
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								ID:      "io.buildpack.engine",
								Version: "v1",
							},
							LayerDiffID: "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
							StoreImage: v1alpha1.StoreImage{
								Image: "some.registry.io/build-package",
							},
							Order: nil,
						},
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								ID:      "io.buildpack.multi",
								Version: "v9",
							},
							LayerDiffID: "sha256:9bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
							StoreImage: v1alpha1.StoreImage{
								Image: "some.registry.io/build-package",
							},
							Order: nil,
						}, {
							BuildpackInfo: v1alpha1.BuildpackInfo{
								ID:      "io.buildpack.multi",
								Version: "v8",
							},
							LayerDiffID: "sha256:8bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
							StoreImage: v1alpha1.StoreImage{
								Image: "some.registry.io/build-package",
							},
							Order: nil,
						},
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								ID:      "io.buildpack.package-manager",
								Version: "v1",
							},
							LayerDiffID: "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
							StoreImage: v1alpha1.StoreImage{
								Image: "some.registry.io/build-package",
							},
							Order: nil,
						},
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								ID:      "io.buildpack.meta",
								Version: "v1",
							},
							LayerDiffID: "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
							StoreImage: v1alpha1.StoreImage{
								Image: "some.registry.io/build-package",
							},
							Order: []v1alpha1.OrderEntry{
								{
									Group: []v1alpha1.BuildpackRef{
										{
											BuildpackInfo: v1alpha1.BuildpackInfo{
												ID:      "io.buildpack.engine",
												Version: "v1",
											},
											Optional: false,
										},
										{
											BuildpackInfo: v1alpha1.BuildpackInfo{
												ID:      "io.buildpack.package-manager",
												Version: "v1",
											},
											Optional: true,
										},
									},
								},
							},
						},
					},
				},
			},
		}

		it("returns layer info from store image", func() {
			info, err := subject.FindByIdAndVersion("io.buildpack.engine", "v1")
			require.NoError(t, err)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					ID:      "io.buildpack.engine",
					Version: "v1",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: engineLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							ID:      "io.buildpack.engine",
							Version: "v1",
						},
					},
				},
			})
		})

		it("returns the alphabetical newest buildpack if version is unspecified", func() {
			info, err := subject.FindByIdAndVersion("io.buildpack.multi", "")
			require.NoError(t, err)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					ID:      "io.buildpack.multi",
					Version: "v9",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: v9Layer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							ID:      "io.buildpack.multi",
							Version: "v9",
						},
					},
				},
			})
		})

		it("returns all buildpack layers in a meta buildpack", func() {
			info, err := subject.FindByIdAndVersion("io.buildpack.meta", "v1")
			require.NoError(t, err)

			require.Equal(t, RemoteBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					ID:      "io.buildpack.meta",
					Version: "v1",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: engineLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							ID:      "io.buildpack.engine",
							Version: "v1",
						},
					},
					{
						v1Layer: packageManagerLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							ID:      "io.buildpack.package-manager",
							Version: "v1",
						},
					},
					{
						v1Layer: metaLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							ID:      "io.buildpack.meta",
							Version: "v1",
						},
						Order: v1alpha1.Order{
							{
								Group: []v1alpha1.BuildpackRef{
									{
										BuildpackInfo: v1alpha1.BuildpackInfo{
											ID:      "io.buildpack.engine",
											Version: "v1",
										},
										Optional: false,
									},
									{
										BuildpackInfo: v1alpha1.BuildpackInfo{
											ID:      "io.buildpack.package-manager",
											Version: "v1",
										},
										Optional: true,
									},
								},
							},
						},
					},
				},
			}, info)
		})

	})

}

type fakeStoreImage struct {
	layersByDiffId []v1.Layer
}

func (f fakeStoreImage) LayerByDiffID(diffId v1.Hash) (v1.Layer, error) {
	for _, layer := range f.layersByDiffId {
		hash, err := layer.DiffID()
		if err != nil {
			return nil, err
		}
		if hash == diffId {
			return layer, nil
		}
	}
	return nil, errors.New("layer not found")
}

func (f fakeStoreImage) LayerByDigest(v1.Hash) (v1.Layer, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) Layers() ([]v1.Layer, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) MediaType() (types.MediaType, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) ConfigName() (v1.Hash, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) ConfigFile() (*v1.ConfigFile, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) RawConfigFile() ([]byte, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) Digest() (v1.Hash, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) Manifest() (*v1.Manifest, error) {
	panic("not implemented in tests")
}

func (f fakeStoreImage) RawManifest() ([]byte, error) {
	panic("not implemented in tests")
}
