package cnb

import (
	"archive/tar"
	"fmt"
	"strconv"
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
		lifecycleImage       = "index.docker.io/cloudfoundry/lifecycle@sha256:d19308ce0c1a9ec083432b2c850d615398f0c6a51095d589d58890a721925584"
		buildImage           = "index.docker.io/cloudfoundry/build@sha256:d19308ce0c1a9ec083432b2c850d615398f0c6a51095d589d58890a721925584"
		runImage             = "index.docker.io/cloudfoundry/run@sha256:469f092c28ab64c6798d6f5e24feb4252ae5b36c2ed79cc667ded85ffb49d996"
		buildImageLayers     = 10
		lifecycleImageLayers = 1

		cnbGroupId = 3000
		cnbUserId  = 4000
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
				BuildImage: expv1alpha1.StackSpecImage{
					Image: "cloudfoundry/build:full-cnb",
				},
				RunImage: expv1alpha1.StackSpecImage{
					Image: "cloudfoundry/run:full-cnb",
				},
			},
			Status: expv1alpha1.StackStatus{
				BuildImage: expv1alpha1.StackStatusImage{LatestImage: buildImage},
				RunImage:   expv1alpha1.StackStatusImage{LatestImage: runImage},
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
								Id:      "io.buildpack.1",
								Version: "v1",
							},
						},
						{
							BuildpackInfo: expv1alpha1.BuildpackInfo{
								Id:      "io.buildpack.2",
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
			LifecycleImage: lifecycleImage,
			KpackVersion:   "v1.2.3 (git sha: abcdefg123456)",
		}
	)

	buildpackRepository.AddBP("io.buildpack.1", "v1", []buildpackLayer{
		{
			v1Layer: buildpack1Layer,
			BuildpackInfo: expv1alpha1.BuildpackInfo{
				Id:      "io.buildpack.1",
				Version: "v1",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.2",
				LayerDiffID: buildpack1Layer.diffID,
				Stacks: []expv1alpha1.BuildpackStack{
					{
						ID: "io.buildpacks.stacks.cflinuxfs3",
					},
				},
			},
		},
	})

	buildpackRepository.AddBP("io.buildpack.2", "v2", []buildpackLayer{
		{
			v1Layer: buildpack3Layer,
			BuildpackInfo: expv1alpha1.BuildpackInfo{
				Id:      "io.buildpack.3",
				Version: "v2",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.2",
				LayerDiffID: buildpack3Layer.diffID,
				Stacks: []expv1alpha1.BuildpackStack{
					{
						ID: "io.buildpacks.stacks.cflinuxfs3",
					},
				},
			},
		},
		{
			v1Layer: buildpack2Layer,
			BuildpackInfo: expv1alpha1.BuildpackInfo{
				Id:      "io.buildpack.2",
				Version: "v1",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.2",
				LayerDiffID: buildpack2Layer.diffID,
				Order: expv1alpha1.Order{
					{
						Group: []expv1alpha1.BuildpackRef{
							{
								BuildpackInfo: expv1alpha1.BuildpackInfo{
									Id:      "io.buildpack.3",
									Version: "v2",
								},
								Optional: false,
							},
						},
					},
				},
				Stacks: []expv1alpha1.BuildpackStack{
					{
						ID: "io.buildpacks.stacks.cflinuxfs3",
					},
					{
						ID: "io.some.other.stack",
					},
				},
			},
		},
	})

	registryClient.AddSaveKeychain("custom/example", keychain)

	when("CreateBuilder", func() {
		var (
			lifecycleImg v1.Image
			buildImg     v1.Image
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

			registryClient.AddImage(lifecycleImage, lifecycleImg, keychain)

			buildImg, err = random.Image(1, int64(buildImageLayers))
			require.NoError(t, err)

			buildImg, err := imagehelpers.SetEnv(buildImg, "CNB_USER_ID", strconv.Itoa(cnbUserId))
			require.NoError(t, err)
			buildImg, err = imagehelpers.SetEnv(buildImg, "CNB_GROUP_ID", strconv.Itoa(cnbGroupId))
			require.NoError(t, err)

			registryClient.AddImage(buildImage, buildImg, keychain)
		})

		it("creates a custom builder", func() {
			builderRecord, err := subject.CreateBuilder(keychain, buildpackRepository, stack, clusterBuilderSpec)
			require.NoError(t, err)

			assert.Len(t, builderRecord.Buildpacks, 3)
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{Id: "io.buildpack.1", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{Id: "io.buildpack.2", Version: "v1"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{Id: "io.buildpack.3", Version: "v2"})
			assert.Equal(t, v1alpha1.BuildStack{RunImage: runImage, ID: "io.buildpacks.stacks.cflinuxfs3"}, builderRecord.Stack)

			assert.Len(t, registryClient.SavedImages(), 1)
			savedImage := registryClient.SavedImages()[tag]

			workingDir, err := imagehelpers.GetWorkingDir(savedImage)
			require.NoError(t, err)
			assert.Equal(t, "/layers", workingDir)

			hash, err := savedImage.Digest()
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s@%s", tag, hash), builderRecord.Image)

			layers, err := savedImage.Layers()
			require.NoError(t, err)

			buildpackLayerCount := 3
			defaultDirectoryLayerCount := 1
			stackTomlLayerCount := 1
			lifecycleSymlinkLayerCount := 1
			orderTomlLayerCount := 1
			assert.Len(t, layers,
				buildImageLayers+
					defaultDirectoryLayerCount+
					lifecycleImageLayers+
					lifecycleSymlinkLayerCount+
					stackTomlLayerCount+
					buildpackLayerCount+
					orderTomlLayerCount)

			var layerTester = layerIteratorTester(0)

			for i := 0; i < buildImageLayers; i++ {
				layerTester.testNextLayer("Build Image Layer", func(index int) {
					buildImgLayers, err := buildImg.Layers()
					require.NoError(t, err)

					assert.Equal(t, layers[i], buildImgLayers[i])
				})
			}

			layerTester.testNextLayer("Default Directory Layer", func(index int) {
				defaultDirectoryLayer := layers[index]

				assertLayerContents(t, defaultDirectoryLayer, map[string]content{
					"/workspace": {
						typeflag: tar.TypeDir,
						mode:     0755,
						uid:      cnbUserId,
						gid:      cnbGroupId,
					},
					"/layers": {
						typeflag: tar.TypeDir,
						mode:     0755,
						uid:      cnbUserId,
						gid:      cnbGroupId,
					},
					"/cnb": {
						typeflag: tar.TypeDir,
						mode:     0755,
					},
					"/cnb/buildpacks": {
						typeflag: tar.TypeDir,
						mode:     0755,
					},
					"/platform": {
						typeflag: tar.TypeDir,
						mode:     0755,
					},
					"/platform/env": {
						typeflag: tar.TypeDir,
						mode:     0755,
					},
				})
			})

			layerTester.testNextLayer("Lifecycle Layer", func(index int) {
				lifecycleLayers, err := lifecycleImg.Layers()
				require.NoError(t, err)

				assert.Equal(t, layers[index], lifecycleLayers[0])
			})

			layerTester.testNextLayer("Lifecycle Symlink", func(index int) {
				assertLayerContents(t, layers[index], map[string]content{
					"/lifecycle": {
						linkname: "/cnb/lifecycle",
						typeflag: tar.TypeSymlink,
						mode:     0644,
					},
				})

			})

			layerTester.testNextLayer("Largest Buildpack Layer", func(index int) {
				assert.Equal(t, layers[index], buildpack3Layer)
			})

			layerTester.testNextLayer("Middle Buildpack Layer", func(index int) {
				assert.Equal(t, layers[index], buildpack2Layer)
			})

			layerTester.testNextLayer("Smallest Buildpack Layer", func(index int) {
				assert.Equal(t, layers[index], buildpack1Layer)
			})

			layerTester.testNextLayer("stack Layer", func(index int) {
				assertLayerContents(t, layers[index], map[string]content{
					"/cnb/stack.toml": //language=toml
					{
						typeflag: tar.TypeReg,
						mode:     0644,
						fileContent: //language=toml
						`[run-image]
  image = "cloudfoundry/run:full-cnb"
`,
					},
				})
			})

			layerTester.testNextLayer("order Layer", func(index int) {
				assert.Equal(t, len(layers)-1, index)

				assertLayerContents(t, layers[index], map[string]content{
					"/cnb/order.toml": {
						typeflag: tar.TypeReg,
						mode:     0644,
						fileContent: //language=toml
						`[[order]]

  [[order.group]]
    id = "io.buildpack.1"
    version = "v1"

  [[order.group]]
    id = "io.buildpack.2"
    version = "v2"
    optional = true
`}})

			})

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
    "version": "v1.2.3 (git sha: abcdefg123456)"
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
      "api": "0.2",
      "layerDiffID": "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
      "stacks": [
        {
          "id": "io.buildpacks.stacks.cflinuxfs3"
        }
      ]
    }
  },
  "io.buildpack.2": {
    "v1": {
      "api": "0.2",
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
      ],
      "stacks": [
        {
          "id": "io.buildpacks.stacks.cflinuxfs3"
        },
        {
          "id": "io.some.other.stack"
        }
      ]
    }
  },
  "io.buildpack.3": {
    "v2": {
      "api": "0.2",
      "layerDiffID": "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
      "stacks": [
        {
          "id": "io.buildpacks.stacks.cflinuxfs3"
        }
      ]
    }
  }
}`, buildpackLayers)

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
			Id:      id,
			Version: version,
		},
		Layers: layers,
	}, nil
}

func (f *fakeBuildpackRepository) AddBP(id, version string, layers []buildpackLayer) {
	f.buildpacks[fmt.Sprintf("%s@%s", id, version)] = layers
}

type content struct {
	typeflag    byte
	fileContent string
	uid, gid    int
	mode        int64
	linkname    string
}

func assertLayerContents(t *testing.T, layer v1.Layer, expectedContents map[string]content) {
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

		require.Equal(t, expectedContent.typeflag, header.Typeflag)

		if header.Typeflag == tar.TypeReg {
			fileContents := make([]byte, header.Size)
			_, _ = reader.Read(fileContents) //todo check error

			require.Equal(t, expectedContent.fileContent, string(fileContents))
		} else if header.Typeflag == tar.TypeSymlink {
			require.Equal(t, expectedContent.linkname, header.Linkname)
		}

		require.Equal(t, header.Uid, expectedContent.uid)
		require.Equal(t, header.Gid, expectedContent.gid)
		require.Equal(t, header.Mode, expectedContent.mode)
		require.True(t, header.ModTime.Equal(time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)))
		delete(expectedContents, header.Name)
	}

	for fileName := range expectedContents {
		t.Fatalf("file %s not in layer", fileName)
	}
}

type layerIteratorTester int

func (i *layerIteratorTester) testNextLayer(name string, test func(index int)) {
	test(int(*i))
	*i++
}
