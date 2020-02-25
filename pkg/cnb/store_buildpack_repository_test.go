package cnb

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1"
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
		engineBuildpack := v1alpha1.StoreBuildpack{
			BuildpackInfo: v1alpha1.BuildpackInfo{
				Id:      "io.buildpack.engine",
				Version: "v1",
			},
			DiffId: "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:d345d1b12ae6b3f7cfc617f7adaebe06c32ce60b1aa30bb80fb622b65523de8f",
			Size:   50,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order: nil,
			API:   "0.1",
			Stacks: []v1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.engine.works",
				},
			},
		}

		packageManagerBuildpack := v1alpha1.StoreBuildpack{
			BuildpackInfo: v1alpha1.BuildpackInfo{
				Id:      "io.buildpack.package-manager",
				Version: "v1",
			},
			DiffId: "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:7c1213a54d20137a7479e72150c058268a6604b98c011b4fc11ca45927923d7b",
			Size:   40,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order: nil,
			API:   "0.2",
			Stacks: []v1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.package.works",
				},
			},
		}

		metaBuildpack := v1alpha1.StoreBuildpack{
			BuildpackInfo: v1alpha1.BuildpackInfo{
				Id:      "io.buildpack.meta",
				Version: "v1",
			},
			DiffId: "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:07db84e57fdd7101104c2469984217696fdfe51591cb1edee2928514135920d6",
			Size:   30,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order: []v1alpha1.OrderEntry{
				{
					Group: []v1alpha1.BuildpackRef{
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.engine",
								Version: "v1",
							},
							Optional: false,
						},
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.package-manager",
								Version: "v1",
							},
							Optional: true,
						},
					},
				},
			},
			API: "0.3",
			Stacks: []v1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.meta.works",
				},
			},
		}

		v8Buildpack := v1alpha1.StoreBuildpack{
			BuildpackInfo: v1alpha1.BuildpackInfo{
				Id:      "io.buildpack.multi",
				Version: "v8",
			},
			DiffId: "sha256:8bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:fc14806eb95d01b6338ba1b9fea605e84db7c8c09561ae360bad5b80b5d0d80b",
			Size:   20,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order: nil,
			API:   "0.2",
			Stacks: []v1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.v8.works",
				},
			},
		}

		v9Buildpack := v1alpha1.StoreBuildpack{
			BuildpackInfo: v1alpha1.BuildpackInfo{
				Id:      "io.buildpack.multi",
				Version: "v9",
			},
			DiffId: "sha256:9bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
			Size:   10,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order: nil,
			API:   "0.2",
			Stacks: []v1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.v9.works",
				},
			},
		}

		storeBuildpackRepository := &StoreBuildpackRepository{
			Keychain: nil,
			Store: &v1alpha1.Store{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-store",
				},
				Status: v1alpha1.StoreStatus{
					Buildpacks: []v1alpha1.StoreBuildpack{
						engineBuildpack,
						v9Buildpack,
						v8Buildpack,
						packageManagerBuildpack,
						metaBuildpack,
					},
				},
			},
		}

		it("returns layer info from store image", func() {
			info, err := storeBuildpackRepository.FindByIdAndVersion("io.buildpack.engine", "v1")
			require.NoError(t, err)

			expectedLayer, err := layerFromStoreBuildpack(nil, engineBuildpack)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					Id:      "io.buildpack.engine",
					Version: "v1",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: expectedLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							Id:      "io.buildpack.engine",
							Version: "v1",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.1",
							LayerDiffID: diffID(t, expectedLayer),
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID: "io.custom.stack",
								},
								{
									ID: "io.stack.only.engine.works",
								},
							},
						},
					},
				},
			})
		})

		it("returns the alphabetical newest buildpack if version is unspecified", func() {
			info, err := storeBuildpackRepository.FindByIdAndVersion("io.buildpack.multi", "")
			require.NoError(t, err)

			expectedLayer, err := layerFromStoreBuildpack(nil, v9Buildpack)
			require.NoError(t, err)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					Id:      "io.buildpack.multi",
					Version: "v9",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: expectedLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							Id:      "io.buildpack.multi",
							Version: "v9",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.2",
							LayerDiffID: diffID(t, expectedLayer),
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID: "io.custom.stack",
								},
								{
									ID: "io.stack.only.v9.works",
								},
							},
						},
					},
				},
			})
		})

		it("returns all buildpack layers in a meta buildpack", func() {
			info, err := storeBuildpackRepository.FindByIdAndVersion("io.buildpack.meta", "v1")
			require.NoError(t, err)

			expectedEngineLayer, err := layerFromStoreBuildpack(nil, engineBuildpack)
			require.NoError(t, err)

			expectedPackageManagerLayer, err := layerFromStoreBuildpack(nil, packageManagerBuildpack)
			require.NoError(t, err)

			expectedMetaLayer, err := layerFromStoreBuildpack(nil, metaBuildpack)
			require.NoError(t, err)

			require.Equal(t, RemoteBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					Id:      "io.buildpack.meta",
					Version: "v1",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: expectedEngineLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							Id:      "io.buildpack.engine",
							Version: "v1",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.1",
							LayerDiffID: diffID(t, expectedEngineLayer),
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID: "io.custom.stack",
								},
								{
									ID: "io.stack.only.engine.works",
								},
							},
						},
					},
					{
						v1Layer: expectedPackageManagerLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							Id:      "io.buildpack.package-manager",
							Version: "v1",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.2",
							LayerDiffID: diffID(t, expectedPackageManagerLayer),
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID: "io.custom.stack",
								},
								{
									ID: "io.stack.only.package.works",
								},
							},
						},
					},
					{
						v1Layer: expectedMetaLayer,
						BuildpackInfo: v1alpha1.BuildpackInfo{
							Id:      "io.buildpack.meta",
							Version: "v1",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.3",
							LayerDiffID: diffID(t, expectedMetaLayer),
							Order: v1alpha1.Order{
								{
									Group: []v1alpha1.BuildpackRef{
										{
											BuildpackInfo: v1alpha1.BuildpackInfo{
												Id:      "io.buildpack.engine",
												Version: "v1",
											},
											Optional: false,
										},
										{
											BuildpackInfo: v1alpha1.BuildpackInfo{
												Id:      "io.buildpack.package-manager",
												Version: "v1",
											},
											Optional: true,
										},
									},
								},
							},
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID:     "io.custom.stack",
									Mixins: nil,
								},
								{
									ID:     "io.stack.only.meta.works",
									Mixins: nil,
								},
							},
						},
					},
				},
			}, info)
		})

	})

}

func diffID(t *testing.T, layer v1.Layer) string {
	id, err := layer.DiffID()
	require.NoError(t, err)

	return id.String()
}
