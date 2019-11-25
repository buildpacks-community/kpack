package cnb

import (
	"errors"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBuildPackageStore(t *testing.T) {
	spec.Run(t, "TestBuildPackageStore", testBuildPackageStore)
}

func testBuildPackageStore(t *testing.T, when spec.G, it spec.S) {
	when("FetchBuildpack", func() {
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

		fakeStoreImage := &fakeStoreImage{
			layersByDiffId: []v1.Layer{
				engineLayer,
				packageManagerLayer,
				metaLayer,
				v8Layer,
				v9Layer,
			},
		}

		store := &BuildPackageStore{
			Image: fakeStoreImage,
			PackageMetadata: map[string]map[string]BuildpackLayerInfo{
				"io.buildpack.engine": {
					"v1": {
						LayerDiffID: "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
					},
				},
				"io.buildpack.package-manager": {
					"v1": {
						LayerDiffID: "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
					},
				},
				"io.buildpack.meta": {
					"v1": {
						LayerDiffID: "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
						Order: Order{
							{
								Group: []BuildpackRef{
									{
										BuildpackInfo: BuildpackInfo{
											ID:      "io.buildpack.engine",
											Version: "v1",
										},
										Optional: false,
									},
									{
										BuildpackInfo: BuildpackInfo{
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
				"io.buildpack.multi": {
					"v8": {
						LayerDiffID: "sha256:8bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
					},
					"v9": {
						LayerDiffID: "sha256:9bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
					},
				},
			},
		}

		it("returns layer info from store image", func() {
			info, err := store.FetchBuildpack("io.buildpack.engine", "v1")
			require.NoError(t, err)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: BuildpackInfo{
					ID:      "io.buildpack.engine",
					Version: "v1",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: engineLayer,
						BuildpackInfo: BuildpackInfo{
							ID:      "io.buildpack.engine",
							Version: "v1",
						},
					},
				},
			})
		})

		it("returns the alphabetical newest buildpack if version is unspecified", func() {
			info, err := store.FetchBuildpack("io.buildpack.multi", "")
			require.NoError(t, err)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: BuildpackInfo{
					ID:      "io.buildpack.multi",
					Version: "v9",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: v9Layer,
						BuildpackInfo: BuildpackInfo{
							ID:      "io.buildpack.multi",
							Version: "v9",
						},
					},
				},
			})
		})

		it("returns all buildpack layers in a meta buildpack", func() {
			info, err := store.FetchBuildpack("io.buildpack.meta", "v1")
			require.NoError(t, err)

			require.Equal(t, RemoteBuildpackInfo{
				BuildpackInfo: BuildpackInfo{
					ID:      "io.buildpack.meta",
					Version: "v1",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: engineLayer,
						BuildpackInfo: BuildpackInfo{
							ID:      "io.buildpack.engine",
							Version: "v1",
						},
					},
					{
						v1Layer: packageManagerLayer,
						BuildpackInfo: BuildpackInfo{
							ID:      "io.buildpack.package-manager",
							Version: "v1",
						},
					},
					{
						v1Layer: metaLayer,
						BuildpackInfo: BuildpackInfo{
							ID:      "io.buildpack.meta",
							Version: "v1",
						},
						Order: Order{
							{
								Group: []BuildpackRef{
									{
										BuildpackInfo: BuildpackInfo{
											ID:      "io.buildpack.engine",
											Version: "v1",
										},
										Optional: false,
									},
									{
										BuildpackInfo: BuildpackInfo{
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
