package cnb

import (
	"archive/tar"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestCreateBuilder(t *testing.T) {
	spec.Run(t, "Create Builder", testCreateBuilder)
}

func testCreateBuilder(t *testing.T, when spec.G, it spec.S) {
	const (
		tag             = "custom/example"
		baseBuilder     = "base/builder"
		runImageTag     = "kpack/run"
		baseImageLayers = 10
	)

	var (
		registryClient = registryfakes.NewFakeClient()

		keychain = authn.NewMultiKeychain(authn.DefaultKeychain)

		buildpackRepository = &fakeBuildpackRepository{buildpacks: map[string][]buildpackLayer{}}

		buildpack1Layer = &fakeLayer{
			digest: "sha256:1bd8899667b8d1e6b124f663faca32903b470831e5e4e99265c839ab34628838",
			diffID: "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   1,
		}
		buildpack2Layer = &fakeLayer{
			digest: "sha256:2bd8899667b8d1e6b124f663faca32903b470831e5e4e99265c839ab34628838",
			diffID: "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   100,
		}
		buildpack3Layer = &fakeLayer{
			digest: "sha256:3bd8899667b8d1e6b124f663faca32903b470831e5e4e99265c839ab34628838",
			diffID: "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			size:   100,
		}

		clusterBuilderSpec = expv1alpha1.CustomBuilderSpec{
			Tag: "custom/example",
			Stack: expv1alpha1.Stack{
				BaseBuilderImage: baseBuilder,
			},
			Store: "some-buildpackRepository",
			Order: []expv1alpha1.OrderEntry{
				{
					Group: []expv1alpha1.BuildpackRef{
						{
							BuildpackInfo: expv1alpha1.BuildpackInfo{
								ID:      "io.buildpack.1",
								Version: "v1",
							},
						},
						{
							BuildpackInfo: expv1alpha1.BuildpackInfo{
								ID:      "io.buildpack.2",
								Version: "v2",
							},
							Optional: true,
						},
					},
				},
			},
		}

		subject = RemoteBuilderCreator{
			RegistryClient: registryClient,
		}
	)

	buildpackRepository.AddBP("io.buildpack.1", "v1", []buildpackLayer{
		{
			v1Layer: buildpack1Layer,
			BuildpackInfo: expv1alpha1.BuildpackInfo{
				ID:      "io.buildpack.1",
				Version: "v1",
			},
		},
	})

	buildpackRepository.AddBP("io.buildpack.2", "v2", []buildpackLayer{
		{
			v1Layer: buildpack3Layer,
			BuildpackInfo: expv1alpha1.BuildpackInfo{
				ID:      "io.buildpack.3",
				Version: "v2",
			},
		},
		{
			v1Layer: buildpack2Layer,
			BuildpackInfo: expv1alpha1.BuildpackInfo{
				ID:      "io.buildpack.2",
				Version: "v1",
			},
			Order: expv1alpha1.Order{
				{
					Group: []expv1alpha1.BuildpackRef{
						{
							BuildpackInfo: expv1alpha1.BuildpackInfo{
								ID:      "io.buildpack.3",
								Version: "v2",
							},
							Optional: false,
						},
					},
				},
			},
		},
	})

	registryClient.AddSaveKeychain("custom/example", keychain)

	when("CreateBuilder", func() {
		var (
			baseImage      v1.Image
			runImageDigest string
		)

		it.Before(func() {
			var err error
			baseImage, err = random.Image(10, int64(baseImageLayers))
			require.NoError(t, err)

			baseImage, err := imagehelpers.SetStringLabels(baseImage, map[string]string{
				stackMetadataLabel: "io.buildpacks.stack",
			})
			require.NoError(t, err)

			baseImage, err = imagehelpers.SetLabels(baseImage, map[string]interface{}{
				buildpackMetadataLabel: BuilderImageMetadata{
					Stack: StackMetadata{
						RunImage: RunImageMetadata{
							Image: runImageTag,
						},
					},
					Lifecycle: LifecycleMetadata{
						LifecycleInfo: LifecycleInfo{
							Version: "0.5.0",
						},
						API: LifecycleAPI{
							BuildpackVersion: "0.2",
							PlatformVersion:  "0.1",
						},
					},
				},
			})
			require.NoError(t, err)

			registryClient.AddImage(baseBuilder, baseImage, "index.docker.io/base/builder@sha256:abc123", keychain)

			runImage, err := random.Image(1, int64(1))
			require.NoError(t, err)

			rawDigest, err := runImage.Digest()
			require.NoError(t, err)
			runImageDigest = rawDigest.Hex

			registryClient.AddImage(runImageTag, runImage, "index.docker.io/kpack/run@sha256:"+runImageDigest, keychain)
		})

		it("creates a custom builder", func() {
			builderRecord, err := subject.CreateBuilder(keychain, buildpackRepository, clusterBuilderSpec)
			require.NoError(t, err)

			assert.Len(t, builderRecord.Buildpacks, 3)
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.1", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.2", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.3", Version: "v2"})
			assert.Equal(t, v1alpha1.BuildStack{RunImage: "index.docker.io/kpack/run@sha256:" + runImageDigest, ID: "io.buildpacks.stack"}, builderRecord.Stack)

			assert.Len(t, registryClient.SavedImages(), 1)
			savedImage := registryClient.SavedImages()[tag]

			hash, err := savedImage.Digest()
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s@%s", tag, hash), builderRecord.Image)

			layers, err := savedImage.Layers()
			require.NoError(t, err)

			numberOfBuildpackLayers := 3
			numberOrderLayers := 1
			assert.Len(t, layers, baseImageLayers+numberOfBuildpackLayers+numberOrderLayers)
			assert.Contains(t, layers, buildpack1Layer)
			assert.Contains(t, layers, buildpack2Layer)
			assert.Contains(t, layers, buildpack3Layer)

			orderLayer := layers[len(layers)-1]
			assertLayerContents(t, orderLayer, 0644, map[string]string{
				"/cnb/order.toml": //language=toml
				`[[order]]

  [[order.group]]
    id = "io.buildpack.1"
    version = "v1"

  [[order.group]]
    id = "io.buildpack.2"
    version = "v2"
    optional = true
`})

			buildpackOrder, err := imagehelpers.GetStringLabel(savedImage, buildpackOrderLabel)
			assert.NoError(t, err)
			assert.JSONEq(t, //language=json
				`[{"group":[{"id":"io.buildpack.1","version":"v1"},{"id":"io.buildpack.2","version":"v2","optional":true}]}]`, buildpackOrder)

			buildpackMetadata, err := imagehelpers.GetStringLabel(savedImage, buildpackMetadataLabel)
			assert.NoError(t, err)
			assert.JSONEq(t, //language=json
				`{
  "description": "Custom Builder built with kpack",
  "stack": {
    "runImage": {
      "image": "kpack/run",
      "mirrors": null
    }
  },
  "lifecycle": {
    "version": "0.5.0",
    "api": {
      "buildpack": "0.2",
      "platform": "0.1"
    }
  },
  "createdBy": {
    "name": "kpack CustomBuilder",
    "version": ""
  },
  "buildpacks": [
    {
      "id": "io.buildpack.3",
      "version": "v2"
    },
    {
      "id": "io.buildpack.2",
      "version": "v1"
    },
    {
      "id": "io.buildpack.1",
      "version": "v1"
    }
  ]
}`, buildpackMetadata)

			buildpackLayers, err := imagehelpers.GetStringLabel(savedImage, buildpackLayersLabel)
			assert.NoError(t, err)
			assert.JSONEq(t, //language=json
				`{
  "io.buildpack.1": {
    "v1": {
      "layerDigest": "sha256:1bd8899667b8d1e6b124f663faca32903b470831e5e4e99265c839ab34628838",
      "layerDiffID": "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462"
    }
  },
  "io.buildpack.2": {
    "v1": {
      "layerDigest": "sha256:2bd8899667b8d1e6b124f663faca32903b470831e5e4e99265c839ab34628838",
      "layerDiffID": "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
      "order": [
        {
          "group": [
            {
              "id": "io.buildpack.3",
              "version": "v2"
            }
          ]
        }
      ]
    }
  },
  "io.buildpack.3": {
    "v2": {
      "layerDigest": "sha256:3bd8899667b8d1e6b124f663faca32903b470831e5e4e99265c839ab34628838",
      "layerDiffID": "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462"
    }
  }
}
`, buildpackLayers)

		})

		it("creates images deterministically ", func() {
			original, err := subject.CreateBuilder(keychain, buildpackRepository, clusterBuilderSpec)
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				other, err := subject.CreateBuilder(keychain, buildpackRepository, clusterBuilderSpec)
				require.NoError(t, err)

				require.Equal(t, original.Image, other.Image)
				require.Equal(t, original.Buildpacks, other.Buildpacks)
			}
		})
	})
}

type fakeBuildpackRepository struct {
	buildpacks map[string][]buildpackLayer
}

func (f *fakeBuildpackRepository) FindByIdAndVersion(id, version string) (RemoteBuildpackInfo, error) {
	layers, ok := f.buildpacks[fmt.Sprintf("%s@%s", id, version)]
	if !ok {
		return RemoteBuildpackInfo{}, errors.New("buildpack not found")
	}

	return RemoteBuildpackInfo{
		BuildpackInfo: expv1alpha1.BuildpackInfo{
			ID:      id,
			Version: version,
		},
		Layers: layers,
	}, nil
}

func (f *fakeBuildpackRepository) AddBP(id, version string, layers []buildpackLayer) {
	f.buildpacks[fmt.Sprintf("%s@%s", id, version)] = layers
}

func assertLayerContents(t *testing.T, layer v1.Layer, expectedMode int64, expectedContents map[string]string) {
	t.Helper()
	uncompressed, err := layer.Uncompressed()
	require.NoError(t, err)
	reader := tar.NewReader(uncompressed)

	for {
		header, err := reader.Next()
		if err != nil {
			break
		}

		expectedContent, ok := expectedContents[header.Name]
		if !ok {
			t.Fatalf("unexpected file %s", header.Name)
		}

		fileContents := make([]byte, header.Size)
		_, _ = reader.Read(fileContents) //todo check error

		require.Equal(t, expectedContent, string(fileContents))
		require.Equal(t, header.Mode, expectedMode)
		require.True(t, header.ModTime.Equal(time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)))

		delete(expectedContents, header.Name)
	}

	for fileName := range expectedContents {
		t.Fatalf("file %s not in layer", fileName)
	}
}
