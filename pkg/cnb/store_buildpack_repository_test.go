package cnb

import (
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

func TestBuildpackRepository(t *testing.T) {
	spec.Run(t, "TestBuildpackRepository", testBuildpackRepository)
}

func testBuildpackRepository(t *testing.T, when spec.G, it spec.S) {
	when("FindByIdAndVersion", func() {
		engineBuildpack := v1alpha1.StoreBuildpack{
			BuildpackInfo: v1alpha1.BuildpackInfo{
				Id:      "io.buildpack.engine",
				Version: "1.0.0",
			},
			DiffId: "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:d345d1b12ae6b3f7cfc617f7adaebe06c32ce60b1aa30bb80fb622b65523de8f",
			Size:   50,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.engine.com",
			API:      "0.1",
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
				Version: "1.0.0",
			},
			DiffId: "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:7c1213a54d20137a7479e72150c058268a6604b98c011b4fc11ca45927923d7b",
			Size:   40,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.package-manager.com",
			API:      "0.2",
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
				Version: "1.0.0",
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
								Version: "1.0.0",
							},
							Optional: false,
						},
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.package-manager",
								Version: "1.0.0",
							},
							Optional: true,
						},
					},
				},
			},
			Homepage: "buildpack.meta.com",
			API:      "0.3",
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
				Version: "8.0.0",
			},
			DiffId: "sha256:8bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:fc14806eb95d01b6338ba1b9fea605e84db7c8c09561ae360bad5b80b5d0d80b",
			Size:   20,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.multi.com",
			API:      "0.2",
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
				Version: "9.0.0",
			},
			DiffId: "sha256:9bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
			Size:   10,
			StoreImage: v1alpha1.StoreImage{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.multi.com",
			API:      "0.2",
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
			ClusterStore: &v1alpha1.ClusterStore{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-store",
				},
				Status: v1alpha1.ClusterStoreStatus{
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
			info, err := storeBuildpackRepository.FindByIdAndVersion("io.buildpack.engine", "1.0.0")
			require.NoError(t, err)

			expectedLayer, err := layerFromStoreBuildpack(nil, engineBuildpack)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: DescriptiveBuildpackInfo{
					BuildpackInfo: v1alpha1.BuildpackInfo{
						Id:      "io.buildpack.engine",
						Version: "1.0.0",
					},
					Homepage: "buildpack.engine.com",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: expectedLayer,
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.engine",
								Version: "1.0.0",
							},
							Homepage: "buildpack.engine.com",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.1",
							LayerDiffID: diffID(t, expectedLayer),
							Homepage:    "buildpack.engine.com",
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

		it("returns the semver newest buildpack if version is unspecified", func() {
			info, err := storeBuildpackRepository.FindByIdAndVersion("io.buildpack.multi", "")
			require.NoError(t, err)

			expectedLayer, err := layerFromStoreBuildpack(nil, v9Buildpack)
			require.NoError(t, err)

			require.Equal(t, info, RemoteBuildpackInfo{
				BuildpackInfo: DescriptiveBuildpackInfo{
					BuildpackInfo: v1alpha1.BuildpackInfo{
						Id:      "io.buildpack.multi",
						Version: "9.0.0",
					},
					Homepage: "buildpack.multi.com",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: expectedLayer,
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.multi",
								Version: "9.0.0",
							},
							Homepage: "buildpack.multi.com",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.2",
							LayerDiffID: diffID(t, expectedLayer),
							Homepage:    "buildpack.multi.com",
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

		it("fails to find the buildpack if version is unspecified and not all buildpacks are semver conformant", func() {
			storeBuildpackRepository.ClusterStore.Status.Buildpacks = append(storeBuildpackRepository.ClusterStore.Status.Buildpacks, v1alpha1.StoreBuildpack{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					Id:      "io.buildpack.multi",
					Version: "my-wacky-version",
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
			})

			_, err := storeBuildpackRepository.FindByIdAndVersion("io.buildpack.multi", "")
			require.Error(t, err, "cannot find buildpack 'io.buildpack.multi' with latest version due to invalid semver 'my-wacky-version'")
		})

		it("returns all buildpack layers in a meta buildpack", func() {
			info, err := storeBuildpackRepository.FindByIdAndVersion("io.buildpack.meta", "1.0.0")
			require.NoError(t, err)

			expectedEngineLayer, err := layerFromStoreBuildpack(nil, engineBuildpack)
			require.NoError(t, err)

			expectedPackageManagerLayer, err := layerFromStoreBuildpack(nil, packageManagerBuildpack)
			require.NoError(t, err)

			expectedMetaLayer, err := layerFromStoreBuildpack(nil, metaBuildpack)
			require.NoError(t, err)

			require.Equal(t, RemoteBuildpackInfo{
				BuildpackInfo: DescriptiveBuildpackInfo{
					BuildpackInfo: v1alpha1.BuildpackInfo{
						Id:      "io.buildpack.meta",
						Version: "1.0.0",
					},
					Homepage: "buildpack.meta.com",
				},
				Layers: []buildpackLayer{
					{
						v1Layer: expectedEngineLayer,
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.engine",
								Version: "1.0.0",
							},
							Homepage: "buildpack.engine.com",
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
							Homepage: "buildpack.engine.com",
						},
					},
					{
						v1Layer: expectedPackageManagerLayer,
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.package-manager",
								Version: "1.0.0",
							},
							Homepage: "buildpack.package-manager.com",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.2",
							LayerDiffID: diffID(t, expectedPackageManagerLayer),
							Homepage:    "buildpack.package-manager.com",
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
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.meta",
								Version: "1.0.0",
							},
							Homepage: "buildpack.meta.com",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.3",
							LayerDiffID: diffID(t, expectedMetaLayer),
							Homepage:    "buildpack.meta.com",
							Order: v1alpha1.Order{
								{
									Group: []v1alpha1.BuildpackRef{
										{
											BuildpackInfo: v1alpha1.BuildpackInfo{
												Id:      "io.buildpack.engine",
												Version: "1.0.0",
											},
											Optional: false,
										},
										{
											BuildpackInfo: v1alpha1.BuildpackInfo{
												Id:      "io.buildpack.package-manager",
												Version: "1.0.0",
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
