package cnb_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestRemoteStoreReader(t *testing.T) {
	spec.Run(t, "Test Remote Store Reader", testRemoteStoreReader)
}

func testRemoteStoreReader(t *testing.T, when spec.G, it spec.S) {
	when("Remote Store Reader", func() {
		const (
			buildpackageA = "build/packageA"
			buildpackageB = "build/packageB"
		)

		var (
			fakeClient        = registryfakes.NewFakeClient()
			remoteStoreReader = &cnb.RemoteStoreReader{
				RegistryClient: fakeClient,
			}

			expectedKeychain = authn.NewMultiKeychain(authn.DefaultKeychain)
		)

		it.Before(func() {
			buildPackageA, err := random.Image(10, int64(10))
			require.NoError(t, err)

			buildPackageA, err = imagehelpers.SetStringLabels(buildPackageA, map[string]string{
				"io.buildpacks.buildpack.layers": //language=json
				`{
  "org.buildpack.meta": {
    "0.0.2": {
      "layerDiffID": "sha256:1c6d357a885d873824545b40e1ccc9fd228c2dd38ba0acb9649955daf2941f94",
      "order": [
        {
          "group": [
            {
              "id": "org.buildpack.multi",
              "version": "0.0.1"
            }
          ]
        },
        {
          "group": [
            {
              "id": "org.buildpack.multi",
              "version": "0.0.2"
            }
          ]
        }
      ]
    }
  },
  "org.buildpack.multi": {
    "0.0.1": {
      "layerDiffID": "sha256:114e397795eceac649afc159afb229211a9ad97b908f7ace225736b8774d9b00"
    },
    "0.0.2": {
      "layerDiffID": "sha256:fcc1dd482e41209737dadce3afd276a93d10d974c174fb72adddd3925b2f31d5"
    }
  }
}
`,
			})
			require.NoError(t, err)

			fakeClient.AddImage(buildpackageA, buildPackageA, "", expectedKeychain)

			buildPackageB, err := random.Image(10, int64(10))
			require.NoError(t, err)

			buildPackageB, err = imagehelpers.SetStringLabels(buildPackageB, map[string]string{
				"io.buildpacks.buildpack.layers": //language=json
				`{
  "org.buildpack.simple": {
    "0.0.1": {
      "layerDiffID": "sha256:1fe2cf74b742ec16c76b9e996c247c78aa41905fe86b744db998094b4bcaf38a"
    }
  }
}
`,
			})
			require.NoError(t, err)

			fakeClient.AddImage(buildpackageB, buildPackageB, "", expectedKeychain)
		})

		it("returns all buildpacks from multiple images", func() {
			storeBuildpacks, err := remoteStoreReader.Read(expectedKeychain, []v1alpha1.StoreImage{
				{
					Image: buildpackageA,
				},
				{
					Image: buildpackageB,
				},
			})
			require.NoError(t, err)

			require.Len(t, storeBuildpacks, 4)
			require.Contains(t, storeBuildpacks, v1alpha1.StoreBuildpack{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					ID:      "org.buildpack.multi",
					Version: "0.0.1",
				},
				LayerDiffID: "sha256:114e397795eceac649afc159afb229211a9ad97b908f7ace225736b8774d9b00",
				StoreImage: v1alpha1.StoreImage{
					Image: buildpackageA,
				},
			})
			require.Contains(t, storeBuildpacks, v1alpha1.StoreBuildpack{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					ID:      "org.buildpack.multi",
					Version: "0.0.2",
				},
				LayerDiffID: "sha256:fcc1dd482e41209737dadce3afd276a93d10d974c174fb72adddd3925b2f31d5",
				StoreImage: v1alpha1.StoreImage{
					Image: buildpackageA,
				},
			})
			require.Contains(t, storeBuildpacks,
				v1alpha1.StoreBuildpack{
					BuildpackInfo: v1alpha1.BuildpackInfo{
						ID:      "org.buildpack.meta",
						Version: "0.0.2",
					},
					LayerDiffID: "sha256:1c6d357a885d873824545b40e1ccc9fd228c2dd38ba0acb9649955daf2941f94",
					StoreImage: v1alpha1.StoreImage{
						Image: buildpackageA,
					},
					Order: []v1alpha1.OrderEntry{
						{
							Group: []v1alpha1.BuildpackRef{
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										ID:      "org.buildpack.multi",
										Version: "0.0.1",
									},
								},
							},
						},
						{
							Group: []v1alpha1.BuildpackRef{
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										ID:      "org.buildpack.multi",
										Version: "0.0.2",
									},
								},
							},
						},
					},
				})

			require.Contains(t, storeBuildpacks, v1alpha1.StoreBuildpack{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					ID:      "org.buildpack.simple",
					Version: "0.0.1",
				},
				LayerDiffID: "sha256:1fe2cf74b742ec16c76b9e996c247c78aa41905fe86b744db998094b4bcaf38a",
				StoreImage: v1alpha1.StoreImage{
					Image: buildpackageB,
				},
			})
		})

		it("returns all buildpacks in a deterministic order", func() {
			expectedBuildpackOrder, err := remoteStoreReader.Read(expectedKeychain, []v1alpha1.StoreImage{
				{
					Image: buildpackageA,
				},
				{
					Image: buildpackageB,
				},
			})
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				subsequentOrder, err := remoteStoreReader.Read(expectedKeychain, []v1alpha1.StoreImage{
					{
						Image: buildpackageA,
					},
					{
						Image: buildpackageB,
					},
				})
				require.NoError(t, err)

				require.Equal(t, expectedBuildpackOrder, subsequentOrder)
			}
		})
	})
}
