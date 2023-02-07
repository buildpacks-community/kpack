package cnb

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestRemoteStoreReader(t *testing.T) {
	spec.Run(t, "Test Remote Store Reader", testRemoteStoreReader)
}

func testRemoteStoreReader(t *testing.T, when spec.G, it spec.S) {
	when("Remote Store Reader", func() {
		const (
			buildpackageA = "build/package_a"
			buildpackageB = "build/package_b"
		)

		var (
			expectedKeychain  = authn.NewMultiKeychain(authn.DefaultKeychain)
			fakeClient        = registryfakes.NewFakeClient()
			remoteStoreReader = &RemoteBuildpackReader{
				RegistryClient: fakeClient,
			}
		)

		it.Before(func() {
			buildPackageAImage, err := random.Image(0, 0)
			require.NoError(t, err)

			buildPackageAImage, err = mutate.AppendLayers(buildPackageAImage,
				fakeLayer{
					digest: "sha256:c375a5c675104fe85cbd3042f5cfa6b1e56573c6d4e5d11224a62598532f3cc1",
					diffID: "sha256:1c6d357a885d873824545b40e1ccc9fd228c2dd38ba0acb9649955daf2941f94",
					size:   10,
				},
				fakeLayer{
					digest: "sha256:52f341c7c36e21e5c344856dd61bc8c2d1188647f259eaba6d375e37c9aed08e",
					diffID: "sha256:114e397795eceac649afc159afb229211a9ad97b908f7ace225736b8774d9b00",
					size:   20,
				},
				fakeLayer{
					digest: "sha256:d345d1b12ae6b3f7cfc617f7adaebe06c32ce60b1aa30bb80fb622b65523de8f",
					diffID: "sha256:fcc1dd482e41209737dadce3afd276a93d10d974c174fb72adddd3925b2f31d5",
					size:   30,
				},
			)
			require.NoError(t, err)

			buildPackageAImage, err = imagehelpers.SetStringLabels(buildPackageAImage, map[string]string{
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
      ],
      "api": "0.2",
      "homepage": "buildpack.meta.com",
      "stacks": [
        {
          "id": "org.some.stack",
          "mixins": [
            "meta:mixin"
          ]
        },
        {
          "id": "org.meta.only.stack"
        }
      ]
    }
  },
  "org.buildpack.multi": {
    "0.0.1": {
      "layerDiffID": "sha256:114e397795eceac649afc159afb229211a9ad97b908f7ace225736b8774d9b00",
      "api": "0.2",
      "homepage": "buildpack.multi.com/v1",
      "stacks": [
        {
          "id": "org.some.stack",
          "mixins": [
            "multi:mixin"
          ]
        },
        {
          "id": "org.multi.only.stack"
        }
      ]
    },
    "0.0.2": {
      "layerDiffID": "sha256:fcc1dd482e41209737dadce3afd276a93d10d974c174fb72adddd3925b2f31d5",
      "api": "0.2",
      "homepage": "buildpack.multi.com/v2",
      "stacks": [
        {
          "id": "org.some.stack",
          "mixins": [
            "multi:mixin"
          ]
        },
        {
          "id": "org.multi.only.stack"
        }
      ]
    }
  }
}
`,
				"io.buildpacks.buildpackage.metadata": //language=json
				`{
  "id": "org.buildpack.meta",
  "version": "0.0.2",
  "homepage": "some-homepage",
  "stacks": [
    {
      "id": "org.some.stack"
    },
    {
      "id": "org.multi.only.stack"
    }
  ]
}`,
			})
			require.NoError(t, err)

			fakeClient.AddImage(buildpackageA, buildPackageAImage, expectedKeychain)

			buildPackageBImage, err := random.Image(0, 0)
			require.NoError(t, err)

			buildPackageBImage, err = mutate.AppendLayers(buildPackageBImage,
				fakeLayer{
					digest: "sha256:6aa3691a73805f608e5fce69fb6bc89aec8362f58a6b4be2682515e9cfa3cc1a",
					diffID: "sha256:1fe2cf74b742ec16c76b9e996c247c78aa41905fe86b744db998094b4bcaf38a",
					size:   40,
				},
			)

			buildPackageBImage, err = imagehelpers.SetStringLabels(buildPackageBImage, map[string]string{
				"io.buildpacks.buildpack.layers": //language=json
				`{
  "org.buildpack.simple": {
    "0.0.1": {
      "layerDiffID": "sha256:1fe2cf74b742ec16c76b9e996c247c78aa41905fe86b744db998094b4bcaf38a",
      "api": "0.2",
      "homepage": "buildpack.simple.com",
      "stacks": [
        {
          "id": "org.some.stack",
          "mixins": [
            "simple:mixin"
          ]
        },
        {
          "id": "org.simple.only.stack"
        }
      ]
    }
  }
}
`,
				"io.buildpacks.buildpackage.metadata": //language=json
				`{
  "id": "org.buildpack.simple",
  "version": "0.0.1",
  "homepage": "some-homepage",
  "stacks": [
    {
      "id": "org.some.stack"
    },
    {
      "id": "org.simple.only.stack"
    }
  ]
}`,
			})
			require.NoError(t, err)

			fakeClient.AddImage(buildpackageB, buildPackageBImage, expectedKeychain)
		})

		it("returns all buildpacks from multiple images", func() {
			storeBuildpacks, err := remoteStoreReader.Read(expectedKeychain, []corev1alpha1.ImageSource{
				{
					Image: buildpackageA,
				},
				{
					Image: buildpackageB,
				},
			})
			require.NoError(t, err)

			require.Len(t, storeBuildpacks, 4)
			require.Contains(t, storeBuildpacks, corev1alpha1.BuildpackStatus{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "org.buildpack.multi",
					Version: "0.0.1",
				},
				Buildpackage: corev1alpha1.BuildpackageInfo{
					Id:       "org.buildpack.meta",
					Version:  "0.0.2",
					Homepage: "some-homepage",
				},
				StoreImage: corev1alpha1.ImageSource{
					Image: buildpackageA,
				},
				API:      "0.2",
				Homepage: "buildpack.multi.com/v1",
				Stacks: []corev1alpha1.BuildpackStack{
					{
						ID:     "org.some.stack",
						Mixins: []string{"multi:mixin"},
					},
					{
						ID: "org.multi.only.stack",
					},
				},
				DiffId: "sha256:114e397795eceac649afc159afb229211a9ad97b908f7ace225736b8774d9b00",
				Digest: "sha256:52f341c7c36e21e5c344856dd61bc8c2d1188647f259eaba6d375e37c9aed08e",
				Size:   20,
			})
			require.Contains(t, storeBuildpacks, corev1alpha1.BuildpackStatus{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "org.buildpack.multi",
					Version: "0.0.2",
				},
				Buildpackage: corev1alpha1.BuildpackageInfo{
					Id:       "org.buildpack.meta",
					Version:  "0.0.2",
					Homepage: "some-homepage",
				},
				StoreImage: corev1alpha1.ImageSource{
					Image: buildpackageA,
				},
				API:      "0.2",
				Homepage: "buildpack.multi.com/v2",
				Stacks: []corev1alpha1.BuildpackStack{
					{
						ID:     "org.some.stack",
						Mixins: []string{"multi:mixin"},
					},
					{
						ID: "org.multi.only.stack",
					},
				},
				DiffId: "sha256:fcc1dd482e41209737dadce3afd276a93d10d974c174fb72adddd3925b2f31d5",
				Digest: "sha256:d345d1b12ae6b3f7cfc617f7adaebe06c32ce60b1aa30bb80fb622b65523de8f",
				Size:   30,
			})

			require.Contains(t, storeBuildpacks,
				corev1alpha1.BuildpackStatus{
					BuildpackInfo: corev1alpha1.BuildpackInfo{
						Id:      "org.buildpack.meta",
						Version: "0.0.2",
					},
					Buildpackage: corev1alpha1.BuildpackageInfo{
						Id:       "org.buildpack.meta",
						Version:  "0.0.2",
						Homepage: "some-homepage",
					},
					StoreImage: corev1alpha1.ImageSource{
						Image: buildpackageA,
					},
					API:      "0.2",
					Homepage: "buildpack.meta.com",
					Order: []corev1alpha1.OrderEntry{
						{
							Group: []corev1alpha1.BuildpackRef{
								{
									BuildpackInfo: corev1alpha1.BuildpackInfo{
										Id:      "org.buildpack.multi",
										Version: "0.0.1",
									},
								},
							},
						},
						{
							Group: []corev1alpha1.BuildpackRef{
								{
									BuildpackInfo: corev1alpha1.BuildpackInfo{
										Id:      "org.buildpack.multi",
										Version: "0.0.2",
									},
								},
							},
						},
					},
					Stacks: []corev1alpha1.BuildpackStack{
						{
							ID:     "org.some.stack",
							Mixins: []string{"meta:mixin"},
						},
						{
							ID: "org.meta.only.stack",
						},
					},
					DiffId: "sha256:1c6d357a885d873824545b40e1ccc9fd228c2dd38ba0acb9649955daf2941f94",
					Digest: "sha256:c375a5c675104fe85cbd3042f5cfa6b1e56573c6d4e5d11224a62598532f3cc1",
					Size:   10,
				})

			require.Contains(t, storeBuildpacks, corev1alpha1.BuildpackStatus{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "org.buildpack.simple",
					Version: "0.0.1",
				},
				Buildpackage: corev1alpha1.BuildpackageInfo{
					Id:       "org.buildpack.simple",
					Version:  "0.0.1",
					Homepage: "some-homepage",
				},
				DiffId:   "sha256:1fe2cf74b742ec16c76b9e996c247c78aa41905fe86b744db998094b4bcaf38a",
				Digest:   "sha256:6aa3691a73805f608e5fce69fb6bc89aec8362f58a6b4be2682515e9cfa3cc1a",
				Size:     40,
				API:      "0.2",
				Homepage: "buildpack.simple.com",
				Stacks: []corev1alpha1.BuildpackStack{
					{
						ID:     "org.some.stack",
						Mixins: []string{"simple:mixin"},
					},
					{
						ID: "org.simple.only.stack",
					},
				},
				StoreImage: corev1alpha1.ImageSource{
					Image: buildpackageB,
				},
			})
		})

		it("returns all buildpacks in a deterministic order", func() {
			expectedBuildpackOrder, err := remoteStoreReader.Read(expectedKeychain, []corev1alpha1.ImageSource{
				{
					Image: buildpackageA,
				},
				{
					Image: buildpackageB,
				},
			})
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				subsequentOrder, err := remoteStoreReader.Read(expectedKeychain, []corev1alpha1.ImageSource{
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

		it("returns all buildpacks in a deterministic order with buildpackages containing duplicates buildpacks", func() {
			imageWithDuplicates, err := random.Image(0, 0)
			require.NoError(t, err)

			imageWithDuplicates, err = mutate.AppendLayers(imageWithDuplicates,
				fakeLayer{
					digest: "sha256:6aa3691a73805f608e5fce69fb6bc89aec8362f58a6b4be2682515e9cfa3cc1a",
					diffID: "sha256:1fe2cf74b742ec16c76b9e996c247c78aa41905fe86b744db998094b4bcaf38a",
					size:   40,
				},
			)

			imageWithDuplicates, err = imagehelpers.SetStringLabels(imageWithDuplicates, map[string]string{
				"io.buildpacks.buildpack.layers": //language=json
				`{
 "org.buildpack.simple": {
   "0.0.1": {
     "layerDiffID": "sha256:1fe2cf74b742ec16c76b9e996c247c78aa41905fe86b744db998094b4bcaf38a",
     "api": "0.2",
	 "homepage": "buildpack.simple.com",
     "stacks": [
       {
         "id": "org.some.stack",
         "mixins": [
           "simple:mixin"
         ]
       },
       {
         "id": "org.simple.only.stack"
       }
     ]
   }
 }
}
`,
				"io.buildpacks.buildpackage.metadata": //language=json
				`{
  "id": "org.buildpack.simple",
  "version": "0.0.1",
  "stacks": [
    {
      "id": "org.some.stack"
    },
    {
      "id": "org.simple.only.stack"
    }
  ]
}`,
			})
			require.NoError(t, err)

			fakeClient.AddImage("image/with_duplicates", imageWithDuplicates, expectedKeychain)

			images := []corev1alpha1.ImageSource{
				{
					Image: buildpackageA,
				},
				{
					Image: buildpackageB,
				},
				{
					Image: "image/with_duplicates",
				},
			}
			expectedBuildpackOrder, err := remoteStoreReader.Read(expectedKeychain, images)
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				subsequentOrder, err := remoteStoreReader.Read(expectedKeychain, images)
				require.NoError(t, err)

				require.Equal(t, expectedBuildpackOrder, subsequentOrder)
			}
		})
	})
}
