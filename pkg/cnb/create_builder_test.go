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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	eV1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
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
		fakeClient = registryfakes.NewFakeClient()

		fakeStoreFactory = &fakeStoreFactory{
			image:    "store/image",
			keychain: authn.NewMultiKeychain(authn.DefaultKeychain),
			store:    &fakeStore{buildpacks: map[string][]buildpackLayer{}},
		}

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

		clusterBuilder = &eV1alpha1.CustomBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster-name",
			},
			Spec: eV1alpha1.CustomBuilderSpec{
				Tag: "custom/example",
				Stack: eV1alpha1.Stack{
					BaseBuilderImage: baseBuilder,
				},
				Store: eV1alpha1.Store{
					Image: fakeStoreFactory.image,
				},
				Order: []eV1alpha1.Group{
					{
						Group: []eV1alpha1.Buildpack{
							{
								ID:      "io.buildpack.1",
								Version: "v1",
							},
							{
								ID:       "io.buildpack.2",
								Version:  "v2",
								Optional: true,
							},
						},
					},
				},
			},
		}

		subject = RemoteBuilderCreator{
			RemoteImageClient: fakeClient,
			StoreFactory:      fakeStoreFactory,
		}
	)

	fakeClient.ExpectedKeychain(fakeStoreFactory.keychain)

	fakeStoreFactory.store.AddBP("io.buildpack.1", "v1", []buildpackLayer{
		{
			v1Layer: buildpack1Layer,
			BuildpackInfo: BuildpackInfo{
				ID:      "io.buildpack.1",
				Version: "v1",
			},
		},
	})

	fakeStoreFactory.store.AddBP("io.buildpack.2", "v2", []buildpackLayer{
		{
			v1Layer: buildpack3Layer,
			BuildpackInfo: BuildpackInfo{
				ID:      "io.buildpack.3",
				Version: "v2",
			},
		},
		{
			v1Layer: buildpack2Layer,
			BuildpackInfo: BuildpackInfo{
				ID:      "io.buildpack.2",
				Version: "v1",
			},
			Order: Order{
				{
					Group: []BuildpackRef{
						{
							BuildpackInfo: BuildpackInfo{
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

	when("CreateBuilder", func() {
		var (
			baseImage      v1.Image
			runImageDigest string
		)

		it.Before(func() {
			var err error
			baseImage, err = random.Image(10, int64(baseImageLayers))
			require.NoError(t, err)

			baseImage, err := registry.SetStringLabel(baseImage, map[string]string{
				stackMetadataLabel: "io.buildpacks.stack",
			})
			require.NoError(t, err)

			baseImage, err = registry.SetLabels(baseImage, map[string]interface{}{
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

			fakeClient.AddImage(baseBuilder, baseImage)

			runImage, err := random.Image(1, int64(1))
			require.NoError(t, err)

			rawDigest, err := runImage.Digest()
			require.NoError(t, err)
			runImageDigest = rawDigest.Hex

			fakeClient.AddImage(runImageTag, runImage)
		})

		it("creates a custom builder", func() {
			builderRecord, err := subject.CreateBuilder(fakeStoreFactory.keychain, clusterBuilder)
			require.NoError(t, err)

			assert.Len(t, builderRecord.Buildpacks, 3)
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.1", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.2", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.3", Version: "v2"})
			assert.Equal(t, v1alpha1.BuildStack{RunImage: "index.docker.io/kpack/run@sha256:" + runImageDigest, ID: "io.buildpacks.stack"}, builderRecord.Stack)

			assert.Len(t, fakeClient.SavedImages(), 1)
			savedImage := fakeClient.SavedImages()[tag]

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

			buildpackOrder, err := registry.GetStringLabel(savedImage, buildpackOrderLabel)
			assert.NoError(t, err)
			assert.JSONEq(t, //language=json
				`[{"group":[{"id":"io.buildpack.1","version":"v1"},{"id":"io.buildpack.2","version":"v2","optional":true}]}]`, buildpackOrder)

			buildpackMetadata, err := registry.GetStringLabel(savedImage, buildpackMetadataLabel)
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

			buildpackLayers, err := registry.GetStringLabel(savedImage, buildpackLayersLabel)
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
			original, err := subject.CreateBuilder(fakeStoreFactory.keychain, clusterBuilder)
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				other, err := subject.CreateBuilder(fakeStoreFactory.keychain, clusterBuilder)
				require.NoError(t, err)

				require.Equal(t, original.Image, other.Image)
				require.Equal(t, original.Buildpacks, other.Buildpacks)
			}
		})
	})
}

type fakeStoreFactory struct {
	image    string
	keychain authn.Keychain
	store    *fakeStore
}

func (f *fakeStoreFactory) MakeStore(keychain authn.Keychain, image string) (Store, error) {
	if keychain != f.keychain {
		return nil, errors.New("invalid keychain")
	}
	if image != f.image {
		return nil, errors.New("invalid store image")
	}

	return f.store, nil
}

type fakeStore struct {
	buildpacks map[string][]buildpackLayer
}

func (f *fakeStore) FetchBuildpack(id, version string) (RemoteBuildpackInfo, error) {
	layers, ok := f.buildpacks[fmt.Sprintf("%s@%s", id, version)]
	if !ok {
		return RemoteBuildpackInfo{}, errors.New("buildpack not found")
	}

	return RemoteBuildpackInfo{
		BuildpackInfo: BuildpackInfo{
			ID:      id,
			Version: version,
		},
		Layers: layers,
	}, nil
}

func (f *fakeStore) AddBP(id, version string, layers []buildpackLayer) {
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
