package cnb

import (
	"archive/tar"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestCreateBuilder(t *testing.T) {
	spec.Run(t, "Create Builder Linux", testCreateBuilder("linux"))
	spec.Run(t, "Create Builder Windows", testCreateBuilder("windows"))
}

func testCreateBuilder(os string) func(*testing.T, spec.G, spec.S) {
	return func(t *testing.T, when spec.G, it spec.S) {
		testCreateBuilderOs(os, t, when, it)
	}
}

func testCreateBuilderOs(os string, t *testing.T, when spec.G, it spec.S) {
	const (
		stackID              = "io.buildpacks.stacks.some-stack"
		mixin                = "some-mixin"
		tag                  = "custom/example"
		lifecycleImage       = "index.docker.io/kpack/lifecycle@sha256:d19308ce0c1a9ec083432b2c850d615398f0c6a51095d589d58890a721925584"
		buildImage           = "index.docker.io/paketo-buildpacks/build@sha256:d19308ce0c1a9ec083432b2c850d615398f0c6a51095d589d58890a721925584"
		runImage             = "index.docker.io/paketo-buildpacks/run@sha256:469f092c28ab64c6798d6f5e24feb4252ae5b36c2ed79cc667ded85ffb49d996"
		buildImageTag        = "paketo-buildpacks/build:full-cnb"
		runImageTag          = "paketo-buildpacks/run:full-cnb"
		buildImageLayers     = 10
		lifecycleImageLayers = 1

		cnbGroupId = 3000
		cnbUserId  = 4000
	)

	var (
		registryClient = registryfakes.NewFakeClient()

		keychain = authn.NewMultiKeychain(authn.DefaultKeychain)

		buildpackRepository = &fakeBuildpackRepository{buildpacks: map[string][]buildpackLayer{}}
		newBuildpackRepo    = func(store *v1alpha1.ClusterStore) BuildpackRepository {
			return buildpackRepository
		}

		linuxLifecycle = &fakeLayer{
			digest: "sha256:5d43d12dabe6070c4a4036e700a6f88a52278c02097b5f200e0b49b3d874c954",
			diffID: "sha256:5d43d12dabe6070c4a4036e700a6f88a52278c02097b5f200e0b49b3d874c954",
			size:   200,
		}

		windowsLifecycle = &fakeLayer{
			digest: "sha256:e40a7455f5495621a585e68523ab66ad8a0b7c791f40bf3aa97c7858003c1287",
			diffID: "sha256:e40a7455f5495621a585e68523ab66ad8a0b7c791f40bf3aa97c7858003c1287",
			size:   200,
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

		store = &v1alpha1.ClusterStore{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: "sample-store",
			},
			Status: v1alpha1.ClusterStoreStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 10,
				},
			},
		}

		stack = &v1alpha1.ClusterStack{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: "sample-stack",
			},
			Spec: v1alpha1.ClusterStackSpec{
				Id: stackID,
				BuildImage: v1alpha1.ClusterStackSpecImage{
					Image: buildImageTag,
				},
				RunImage: v1alpha1.ClusterStackSpecImage{
					Image: runImageTag,
				},
			},
			Status: v1alpha1.ClusterStackStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 11,
				},
				ResolvedClusterStack: v1alpha1.ResolvedClusterStack{
					Id: stackID,
					BuildImage: v1alpha1.ClusterStackStatusImage{
						LatestImage: buildImage,
						Image:       buildImageTag,
					},
					RunImage: v1alpha1.ClusterStackStatusImage{
						LatestImage: runImage,
						Image:       runImageTag,
					},
					Mixins:  []string{"some-unused-mixin", mixin},
					UserID:  cnbUserId,
					GroupID: cnbGroupId,
				},
			},
		}

		clusterBuilderSpec = v1alpha1.BuilderSpec{
			Tag: "custom/example",
			Stack: corev1.ObjectReference{
				Kind: "Stack",
				Name: "some-stack",
			},
			Store: corev1.ObjectReference{
				Name: "some-buildpackRepository",
				Kind: "ClusterStore",
			},
			Order: []v1alpha1.OrderEntry{
				{
					Group: []v1alpha1.BuildpackRef{
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.1",
								Version: "v1",
							},
						},
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{
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
			RegistryClient:         registryClient,
			LifecycleImage:         lifecycleImage,
			KpackVersion:           "v1.2.3 (git sha: abcdefg123456)",
			NewBuildpackRepository: newBuildpackRepo,
		}
	)

	buildpackRepository.AddBP("io.buildpack.1", "v1", []buildpackLayer{
		{
			v1Layer: buildpack1Layer,
			BuildpackInfo: DescriptiveBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					Id:      "io.buildpack.1",
					Version: "v1",
				},
				Homepage: "buildpack.1.com",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.2",
				LayerDiffID: buildpack1Layer.diffID,
				Stacks: []v1alpha1.BuildpackStack{
					{
						ID:     stackID,
						Mixins: []string{mixin},
					},
				},
			},
		},
	})

	buildpackRepository.AddBP("io.buildpack.2", "v2", []buildpackLayer{
		{
			v1Layer: buildpack3Layer,
			BuildpackInfo: DescriptiveBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					Id:      "io.buildpack.3",
					Version: "v3",
				},
				Homepage: "buildpack.3.com",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.3",
				LayerDiffID: buildpack3Layer.diffID,
				Stacks: []v1alpha1.BuildpackStack{
					{
						ID: stackID,
					},
					{
						ID: "io.some.other.stack",
					},
				},
			},
		},
		{
			v1Layer: buildpack2Layer,
			BuildpackInfo: DescriptiveBuildpackInfo{
				BuildpackInfo: v1alpha1.BuildpackInfo{
					Id:      "io.buildpack.2",
					Version: "v2",
				},
				Homepage: "buildpack.2.com",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.3",
				LayerDiffID: buildpack2Layer.diffID,
				Order: v1alpha1.Order{
					{
						Group: []v1alpha1.BuildpackRef{
							{
								BuildpackInfo: v1alpha1.BuildpackInfo{
									Id:      "io.buildpack.3",
									Version: "v2",
								},
								Optional: false,
							},
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
			buildImg     v1.Image
		)

		it.Before(func() {
			var err error
			lifecycleImg, err = mutate.AppendLayers(empty.Image, linuxLifecycle, windowsLifecycle)
			require.NoError(t, err)

			lifecycleImg, err = imagehelpers.SetStringLabel(lifecycleImg, "linux", linuxLifecycle.diffID)
			require.NoError(t, err)

			lifecycleImg, err = imagehelpers.SetStringLabel(lifecycleImg, "windows", windowsLifecycle.diffID)
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
					APIs: LifecycleAPIs{
						Buildpack: APIVersions{
							Deprecated: []string{"0.2"},
							Supported:  []string{"0.3"},
						},
						Platform: APIVersions{
							Deprecated: []string{"0.1"},
							Supported:  []string{"0.2"},
						},
					},
				},
			})
			require.NoError(t, err)

			registryClient.AddImage(lifecycleImage, lifecycleImg, keychain)

			buildImg, err = random.Image(1, int64(buildImageLayers))
			require.NoError(t, err)

			config, err := buildImg.ConfigFile()
			require.NoError(t, err)

			config.OS = os
			buildImg, err = mutate.ConfigFile(buildImg, config)

			registryClient.AddImage(buildImage, buildImg, keychain)
		})

		it("creates a custom builder", func() {
			builderRecord, err := subject.CreateBuilder(keychain, store, stack, clusterBuilderSpec)
			require.NoError(t, err)

			assert.Len(t, builderRecord.Buildpacks, 3)
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{Id: "io.buildpack.1", Version: "v1", Homepage: "buildpack.1.com"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{Id: "io.buildpack.2", Version: "v2", Homepage: "buildpack.2.com"})
			assert.Contains(t, builderRecord.Buildpacks, v1alpha1.BuildpackMetadata{Id: "io.buildpack.3", Version: "v3", Homepage: "buildpack.3.com"})
			assert.Equal(t, v1alpha1.BuildStack{RunImage: runImage, ID: stackID}, builderRecord.Stack)
			assert.Equal(t, int64(10), builderRecord.ObservedStoreGeneration)
			assert.Equal(t, int64(11), builderRecord.ObservedStackGeneration)

			assert.Equal(t, builderRecord.Order, []v1alpha1.OrderEntry{
				{
					Group: []v1alpha1.BuildpackRef{
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{Id: "io.buildpack.1", Version: "v1"},
							Optional:      false,
						},
						{
							BuildpackInfo: v1alpha1.BuildpackInfo{Id: "io.buildpack.2", Version: "v2"},
							Optional:      true,
						},
					},
				},
			})

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
			orderTomlLayerCount := 1
			assert.Len(t, layers,
				buildImageLayers+
					defaultDirectoryLayerCount+
					lifecycleImageLayers+
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

				assertLayerContents(t, os, defaultDirectoryLayer, map[string]content{
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
				if os == "linux" {
					assert.Equal(t, layers[index], linuxLifecycle)
				} else {
					assert.Equal(t, layers[index], windowsLifecycle)
				}
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
				assertLayerContents(t, os, layers[index], map[string]content{
					"/cnb/stack.toml": //language=toml
					{
						typeflag: tar.TypeReg,
						mode:     0644,
						fileContent: //language=toml
						`[run-image]
  image = "paketo-buildpacks/run:full-cnb"
`,
					},
				})
			})

			layerTester.testNextLayer("order Layer", func(index int) {
				assert.Equal(t, len(layers)-1, index)

				assertLayerContents(t, os, layers[index], map[string]content{
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
      "image": "paketo-buildpacks/run:full-cnb",
      "mirrors": null
    }
  },
  "lifecycle": {
    "version": "0.5.0",
    "api": {
      "buildpack": "0.2",
      "platform": "0.1"
    },
    "apis": {
      "buildpack": {
		"deprecated": ["0.2"],
		"supported": ["0.3"]
      },
      "platform": {
        "deprecated": ["0.1"],
        "supported": ["0.2"]
      }
    }
  },
  "createdBy": {
    "name": "kpack Builder",
    "version": "v1.2.3 (git sha: abcdefg123456)"
  },
  "buildpacks": [
    {
      "id": "io.buildpack.3",
      "version": "v3",
	  "homepage": "buildpack.3.com"
    },
    {
      "id": "io.buildpack.2",
      "version": "v2",
	  "homepage": "buildpack.2.com"
    },
    {
      "id": "io.buildpack.1",
      "version": "v1",
	  "homepage": "buildpack.1.com"
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
          "id": "io.buildpacks.stacks.some-stack",
          "mixins": ["some-mixin"]
        }
      ]
    }
  },
  "io.buildpack.2": {
    "v2": {
      "api": "0.3",
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
    "v3": {
      "api": "0.3",
      "layerDiffID": "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
      "stacks": [
        {
          "id": "io.buildpacks.stacks.some-stack"
        },
        {
          "id": "io.some.other.stack"
        }
      ]
    }
  }
}`, buildpackLayers)

		})

		it("creates images deterministically ", func() {
			original, err := subject.CreateBuilder(keychain, store, stack, clusterBuilderSpec)
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				other, err := subject.CreateBuilder(keychain, store, stack, clusterBuilderSpec)
				require.NoError(t, err)

				require.Equal(t, original.Image, other.Image)
				require.Equal(t, original.Buildpacks, other.Buildpacks)
			}
		})

		when("validating buildpacks", func() {
			it("errors with unsupported stack", func() {
				buildpackRepository.AddBP("io.buildpack.unsupported.stack", "v4", []buildpackLayer{
					{
						v1Layer: buildpack1Layer,
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.unsupported.stack",
								Version: "v4",
							},
							Homepage: "buildpack.4.com",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.2",
							LayerDiffID: buildpack1Layer.diffID,
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID: "io.buildpacks.stacks.unsupported",
								},
							},
						},
					},
				})

				clusterBuilderSpec.Order = []v1alpha1.OrderEntry{
					{
						Group: []v1alpha1.BuildpackRef{
							{
								BuildpackInfo: v1alpha1.BuildpackInfo{
									Id:      "io.buildpack.unsupported.stack",
									Version: "v4",
								},
							},
						},
					},
				}

				_, err := subject.CreateBuilder(keychain, store, stack, clusterBuilderSpec)
				require.EqualError(t, err, "validating buildpack io.buildpack.unsupported.stack@v4: stack io.buildpacks.stacks.some-stack is not supported")
			})

			it("errors with unsupported mixin", func() {
				buildpackRepository.AddBP("io.buildpack.unsupported.mixin", "v4", []buildpackLayer{
					{
						v1Layer: buildpack1Layer,
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.unsupported.mixin",
								Version: "v4",
							},
							Homepage: "buildpack.1.com",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.2",
							LayerDiffID: buildpack1Layer.diffID,
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID:     stackID,
									Mixins: []string{mixin, "something-missing-mixin", "something-missing-mixin2"},
								},
							},
						},
					},
				})

				clusterBuilderSpec.Order = []v1alpha1.OrderEntry{
					{
						Group: []v1alpha1.BuildpackRef{
							{
								BuildpackInfo: v1alpha1.BuildpackInfo{
									Id:      "io.buildpack.unsupported.mixin",
									Version: "v4",
								},
							},
						},
					},
				}

				_, err := subject.CreateBuilder(keychain, store, stack, clusterBuilderSpec)
				require.EqualError(t, err, "validating buildpack io.buildpack.unsupported.mixin@v4: stack missing mixin(s): something-missing-mixin, something-missing-mixin2")
			})

			it("errors with unsupported buildpack version", func() {
				buildpackRepository.AddBP("io.buildpack.unsupported.buildpack.api", "v4", []buildpackLayer{
					{
						v1Layer: buildpack1Layer,
						BuildpackInfo: DescriptiveBuildpackInfo{
							BuildpackInfo: v1alpha1.BuildpackInfo{
								Id:      "io.buildpack.unsupported.buildpack.api",
								Version: "v4",
							},
							Homepage: "buildpack.4.com",
						},
						BuildpackLayerInfo: BuildpackLayerInfo{
							API:         "0.1",
							LayerDiffID: buildpack1Layer.diffID,
							Stacks: []v1alpha1.BuildpackStack{
								{
									ID: stackID,
								},
							},
						},
					},
				})

				clusterBuilderSpec.Order = []v1alpha1.OrderEntry{
					{
						Group: []v1alpha1.BuildpackRef{
							{
								BuildpackInfo: v1alpha1.BuildpackInfo{
									Id:      "io.buildpack.unsupported.buildpack.api",
									Version: "v4",
								},
							},
						},
					},
				}

				_, err := subject.CreateBuilder(keychain, store, stack, clusterBuilderSpec)
				require.EqualError(t, err, "validating buildpack io.buildpack.unsupported.buildpack.api@v4: unsupported buildpack api: 0.1, expecting: 0.2, 0.3")
			})

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
		BuildpackInfo: buildpackInfoInLayers(layers, id, version),
		Layers:        layers,
	}, nil
}

func buildpackInfoInLayers(buildpackLayers []buildpackLayer, id, version string) DescriptiveBuildpackInfo {
	for _, b := range buildpackLayers {
		if b.BuildpackInfo.Id == id && b.BuildpackInfo.Version == version {
			return b.BuildpackInfo
		}
	}
	panic("unexpected missing buildpack info")
}

func (f *fakeBuildpackRepository) AddBP(id, version string, layers []buildpackLayer) {
	f.buildpacks[fmt.Sprintf("%s@%s", id, version)] = layers
}

type content struct {
	typeflag      byte
	fileContent   string
	uid, gid      int
	mode          int64
	linkname      string
	ignoreModTime bool
}

func assertLayerContents(t *testing.T, os string, layer v1.Layer, expectedContents map[string]content) {
	t.Helper()
	expectedContents = expectedPathsIfWindows(os, expectedContents)

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
		if !expectedContent.ignoreModTime {
			require.True(t, header.ModTime.Equal(time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)))
		}
		delete(expectedContents, header.Name)
	}

	for fileName := range expectedContents {
		t.Fatalf("file %s not in layer", fileName)
	}
}

func expectedPathsIfWindows(os string, contents map[string]content) map[string]content {
	if os == "linux" {
		return contents
	}

	newExpectedContents := map[string]content{}
	newExpectedContents["Files"] = content{
		typeflag:      tar.TypeDir,
		ignoreModTime: true,
	}
	newExpectedContents["Hives"] = content{
		typeflag:      tar.TypeDir,
		ignoreModTime: true,
	}
	for headerPath, v := range contents {
		newPath := path.Join("Files", headerPath)

		var parentDir string
		//write windows parent paths
		//extracted from windows writer
		for _, pathPart := range strings.Split(path.Dir(newPath), "/") {
			parentDir = path.Join(parentDir, pathPart)

			if _, present := newExpectedContents[parentDir]; !present {
				newExpectedContents[parentDir] = content{
					typeflag:      tar.TypeDir,
					ignoreModTime: true,
				}
			}
		}
		newExpectedContents[newPath] = v
	}
	return newExpectedContents
}

type layerIteratorTester int

func (i *layerIteratorTester) testNextLayer(name string, test func(index int)) {
	test(int(*i))
	*i++
}
