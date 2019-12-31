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
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		tag                  = "custom/example"
		lifecycleImageRef    = "index.docker.io/cloudfoundry/lifecycle@sha256:d19308ce0c1a9ec083432b2c850d615398f0c6a51095d589d58890a721925584"
		buildImageRef        = "index.docker.io/cloudfoundry/build@sha256:d19308ce0c1a9ec083432b2c850d615398f0c6a51095d589d58890a721925584"
		runImageRef          = "index.docker.io/cloudfoundry/run@sha256:469f092c28ab64c6798d6f5e24feb4252ae5b36c2ed79cc667ded85ffb49d996"
		buildImageLayers     = 10
		lifecycleImageLayers = 1
		stackTomlLayers      = 1
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

		stack = &expv1alpha1.Stack{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: "sample-stack",
			},
			Spec: expv1alpha1.StackSpec{
				Id: "io.buildpacks.stacks.cflinuxfs3",
				BuildImage: expv1alpha1.StackImage{
					Image: "cloudfoundry/build:full-cnb",
				},
				RunImage: expv1alpha1.StackImage{
					Image: "cloudfoundry/run:full-cnb",
				},
			},
			Status: expv1alpha1.StackStatus{
				BuildImageRef: buildImageRef,
				RunImageRef:   runImageRef,
			},
		}

		clusterBuilderSpec = expv1alpha1.CustomBuilderSpec{
			Tag:   "custom/example",
			Stack: "some-stack",
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
			LifecycleImage: lifecycleImageRef,
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
			lifecycleImg v1.Image
		)

		it.Before(func() {
			var err error
			lifecycleImg, err = random.Image(10, int64(lifecycleImageLayers))
			require.NoError(t, err)

			lifecycleImg, err = imagehelpers.SetLabels(lifecycleImg, map[string]interface{}{
				lifecycleMetadataLabel: LifecycleMetadata{
					LifecycleInfo: LifecycleInfo{
						Version: "0.5.0",
					},
					API: LifecycleAPI{
						BuildpackVersion: "0.2",
						PlatformVersion:  "0.1",
					},
				},
			})
			require.NoError(t, err)

			registryClient.AddImage(lifecycleImageRef, lifecycleImg, keychain)

			buildImage, err := random.Image(1, int64(buildImageLayers))
			require.NoError(t, err)

			registryClient.AddImage(buildImageRef, buildImage, keychain)
		})

		it("creates a custom builder", func() {
			builderRecord, err := subject.CreateBuilder(keychain, buildpackRepository, stack, clusterBuilderSpec)
			require.NoError(t, err)

			assert.Len(t, builderRecord.Buildpacks, 3)
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.1", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.2", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{ID: "io.buildpack.3", Version: "v2"})
			assert.Equal(t, v1alpha1.BuildStack{RunImage: runImageRef, ID: "io.buildpacks.stacks.cflinuxfs3"}, builderRecord.Stack)

			assert.Len(t, registryClient.SavedImages(), 1)
			savedImage := registryClient.SavedImages()[tag]

			hash, err := savedImage.Digest()
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s@%s", tag, hash), builderRecord.Image)

			layers, err := savedImage.Layers()
			require.NoError(t, err)

			numberOfBuildpackLayers := 3
			numberOrderLayers := 1
			assert.Len(t, layers, buildImageLayers+lifecycleImageLayers+stackTomlLayers+numberOfBuildpackLayers+numberOrderLayers)
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

			stackLayer := layers[buildImageLayers+lifecycleImageLayers]
			assertLayerContents(t, stackLayer, 0644, map[string]string{
				"/cnb/stack.toml": //language=toml
				`[run-image]
  image = "cloudfoundry/run:full-cnb"
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
      "image": "cloudfoundry/run:full-cnb",
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
			original, err := subject.CreateBuilder(keychain, buildpackRepository, stack, clusterBuilderSpec)
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				other, err := subject.CreateBuilder(keychain, buildpackRepository, stack, clusterBuilderSpec)
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
