package cnb

import (
	"archive/tar"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestCreateBuilder(t *testing.T) {
	spec.Run(t, "Create Builder Linux", testCreateBuilder)
}

func testCreateBuilder(t *testing.T, when spec.G, it spec.S) {
	const (
		stackID              = "io.buildpacks.stacks.some-stack"
		mixin                = "some-mixin"
		builderTag           = "custom/example:test-builder"
		buildImage           = "index.docker.io/paketo-buildpacks/build@sha256:d19308ce0c1a9ec083432b2c850d615398f0c6a51095d589d58890a721925584"
		relocatedRunImageTag = "custom/example:test-builder-run-image"
		buildImageTag        = "paketo-buildpacks/build:full-cnb"
		runImageTag          = "paketo-buildpacks/run:full-cnb"
		lifecycleImageTag    = "buildpacksio/lifecycle:latest"
		buildImageLayers     = 10
		lifecycleImageLayers = 1

		cnbGroupId = 3000
		cnbUserId  = 4000
	)

	var (
		registryClient    = registryfakes.NewFakeClient()
		builderKeychain   = authn.NewMultiKeychain(authn.DefaultKeychain)
		stackKeychain     = authn.NewMultiKeychain(authn.DefaultKeychain)
		lifecycleKeychain = authn.NewMultiKeychain(authn.DefaultKeychain)
		runImage          = createRunImage()
		runImageDigest    = digest(runImage)
		runImageRef       = fmt.Sprintf("%s@%s", runImageTag, runImageDigest)
		ctx               = context.Background()

		fetcher = &fakeFetcher{buildpacks: map[string][]buildpackLayer{}, observedGeneration: 10}

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

		stack = &buildapi.ClusterStack{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sample-stack",
			},
			Spec: buildapi.ClusterStackSpec{
				Id: stackID,
				BuildImage: buildapi.ClusterStackSpecImage{
					Image: buildImageTag,
				},
				RunImage: buildapi.ClusterStackSpecImage{
					Image: runImageTag,
				},
			},
			Status: buildapi.ClusterStackStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 11,
				},
				ResolvedClusterStack: buildapi.ResolvedClusterStack{
					Id: stackID,
					BuildImage: buildapi.ClusterStackStatusImage{
						LatestImage: buildImage,
						Image:       buildImageTag,
					},
					RunImage: buildapi.ClusterStackStatusImage{
						LatestImage: runImageRef,
						Image:       runImageTag,
					},
					Mixins:  []string{"some-unused-mixin", mixin, "common-mixin", "build:another-common-mixin", "run:another-common-mixin"},
					UserID:  cnbUserId,
					GroupID: cnbGroupId,
				},
			},
		}

		clusterLifecycle = &buildapi.ClusterLifecycle{
			ObjectMeta: metav1.ObjectMeta{
				Name: "sample-stack",
			},
			Spec: buildapi.ClusterLifecycleSpec{
				ImageSource: corev1alpha1.ImageSource{Image: lifecycleImageTag},
			},
			Status: buildapi.ClusterLifecycleStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 11,
				},
				ResolvedClusterLifecycle: buildapi.ResolvedClusterLifecycle{
					Version: "some-version",
				},
			},
		}

		clusterBuilderSpec = buildapi.BuilderSpec{
			Tag: builderTag,
			Stack: corev1.ObjectReference{
				Kind: "stack",
				Name: "some-stack",
			},
			Store: corev1.ObjectReference{
				Name: "some-buildpackRepository",
				Kind: "ClusterStore",
			},
			Order: []buildapi.BuilderOrderEntry{
				{
					Group: []buildapi.BuilderBuildpackRef{
						{
							BuildpackRef: corev1alpha1.BuildpackRef{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "io.buildpack.1",
									Version: "v1",
								},
							},
						},
						{
							BuildpackRef: corev1alpha1.BuildpackRef{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "io.buildpack.2",
									Version: "v2",
								},
								Optional: true,
							},
						},
						{
							BuildpackRef: corev1alpha1.BuildpackRef{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "io.buildpack.4",
									Version: "v4",
								},
							},
						},
					},
				},
			},
			AdditionalLabels: map[string]string{
				"os":         "special",
				"importance": "high",
			},
		}

		subject = RemoteBuilderCreator{
			RegistryClient: registryClient,
			KpackVersion:   "v1.2.3 (git sha: abcdefg123456)",
			ImageSigner: &fakeBuilderSigner{
				signBuilder: func(ctx context.Context, s string, secrets []*corev1.Secret, keychain authn.Keychain) ([]buildapi.CosignSignature, error) {
					// no-op
					return nil, nil
				},
			},
		}

		addBuildpack = func(t *testing.T, id, version, homepage, api string, stacks []corev1alpha1.BuildpackStack) {
			fetcher.AddBuildpack(t, id, version, []buildpackLayer{{
				v1Layer: buildpack1Layer,
				BuildpackInfo: DescriptiveBuildpackInfo{
					BuildpackInfo: corev1alpha1.BuildpackInfo{
						Id:      id,
						Version: version,
					},
					Homepage: homepage,
				},
				BuildpackLayerInfo: BuildpackLayerInfo{
					API:         api,
					LayerDiffID: buildpack1Layer.diffID,
					Stacks:      stacks,
				},
			}})
		}
	)

	it.Before(func() {
		buildpack1 := buildpackLayer{
			v1Layer: buildpack1Layer,
			BuildpackInfo: DescriptiveBuildpackInfo{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "io.buildpack.1",
					Version: "v1",
				},
				Homepage: "buildpack.1.com",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.2",
				LayerDiffID: buildpack1Layer.diffID,
				Stacks: []corev1alpha1.BuildpackStack{
					{
						ID:     stackID,
						Mixins: []string{mixin},
					},
				},
			},
		}
		buildpack2 := buildpackLayer{
			v1Layer: buildpack2Layer,
			BuildpackInfo: DescriptiveBuildpackInfo{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "io.buildpack.2",
					Version: "v2",
				},
				Homepage: "buildpack.2.com",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.3",
				LayerDiffID: buildpack2Layer.diffID,
				Order: corev1alpha1.Order{
					{
						Group: []corev1alpha1.BuildpackRef{
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "io.buildpack.3",
									Version: "v2",
								},
								Optional: false,
							},
						},
					},
				},
			},
		}
		buildpack3 := buildpackLayer{
			v1Layer: buildpack3Layer,
			BuildpackInfo: DescriptiveBuildpackInfo{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "io.buildpack.3",
					Version: "v3",
				},
				Homepage: "buildpack.3.com",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.3",
				LayerDiffID: buildpack3Layer.diffID,
				Stacks: []corev1alpha1.BuildpackStack{
					{
						ID: stackID,
					},
					{
						ID: "io.some.other.stack",
					},
				},
			},
		}
		buildpackWithDuplicatLayer := buildpackLayer{
			v1Layer: buildpack3Layer,
			BuildpackInfo: DescriptiveBuildpackInfo{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "io.buildpack.4",
					Version: "v4",
				},
				Homepage: "buildpack.4.com",
			},
			BuildpackLayerInfo: BuildpackLayerInfo{
				API:         "0.3",
				LayerDiffID: buildpack3Layer.diffID,
				Stacks: []corev1alpha1.BuildpackStack{
					{
						ID: stackID,
					},
					{
						ID: "io.some.other.stack",
					},
				},
			},
		}

		fetcher.AddBuildpack(t, "io.buildpack.1", "v1", []buildpackLayer{buildpack1})
		fetcher.AddBuildpack(t, "io.buildpack.2", "v2", []buildpackLayer{buildpack3, buildpack2})
		fetcher.AddBuildpack(t, "io.buildpack.4", "v4", []buildpackLayer{buildpackWithDuplicatLayer})
	})

	registryClient.AddSaveKeychain(builderTag, builderKeychain)
	registryClient.AddImage(runImageRef, runImage, stackKeychain)

	when("CreateBuilder", func() {
		var (
			buildImg     v1.Image
			lifecycleImg v1.Image
		)

		it.Before(func() {
			var err error

			// build image

			buildImg, err = random.Image(1, int64(buildImageLayers))
			require.NoError(t, err)

			config, err := buildImg.ConfigFile()
			require.NoError(t, err)

			config.OS = "linux"
			buildImg, err = mutate.ConfigFile(buildImg, config)

			registryClient.AddImage(buildImage, buildImg, stackKeychain)

			// lifecycle image

			lifecycleImg, err = random.Image(1, int64(lifecycleImageLayers))
			require.NoError(t, err)

			lConfig, err := lifecycleImg.ConfigFile()
			require.NoError(t, err)

			lConfig.OS = "linux"
			lifecycleImg, err = mutate.ConfigFile(lifecycleImg, config)

			registryClient.AddImage(lifecycleImageTag, lifecycleImg, lifecycleKeychain)

			// cluster lifecycle

			clusterLifecycle.Status.ResolvedClusterLifecycle = buildapi.ResolvedClusterLifecycle{
				Version: "0.5.0",
				APIs: buildapi.LifecycleAPIs{
					Buildpack: buildapi.APIVersions{
						Deprecated: []string{"0.2"},
						Supported:  []string{"0.3"},
					},
					Platform: buildapi.APIVersions{
						Deprecated: []string{"0.3"},
						Supported:  []string{"0.4"},
					},
				},
			}
		})

		it("creates a custom builder with a relocated run image", func() {
			builderRecord, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
			require.NoError(t, err)

			assert.Len(t, builderRecord.Buildpacks, 4)
			assert.Contains(t, builderRecord.Buildpacks, corev1alpha1.BuildpackMetadata{Id: "io.buildpack.1", Version: "v1", Homepage: "buildpack.1.com"})
			assert.Contains(t, builderRecord.Buildpacks, corev1alpha1.BuildpackMetadata{Id: "io.buildpack.2", Version: "v2", Homepage: "buildpack.2.com"})
			assert.Contains(t, builderRecord.Buildpacks, corev1alpha1.BuildpackMetadata{Id: "io.buildpack.3", Version: "v3", Homepage: "buildpack.3.com"})
			assert.Contains(t, builderRecord.Buildpacks, corev1alpha1.BuildpackMetadata{Id: "io.buildpack.4", Version: "v4", Homepage: "buildpack.4.com"})
			assert.Equal(t, corev1alpha1.BuildStack{RunImage: fmt.Sprintf("%s@%s", relocatedRunImageTag, runImageDigest), ID: stackID}, builderRecord.Stack)
			assert.Equal(t, int64(10), builderRecord.ObservedStoreGeneration)
			assert.Equal(t, int64(11), builderRecord.ObservedStackGeneration)
			assert.Equal(t, "linux", builderRecord.OS)

			assert.Equal(t, builderRecord.Order, []corev1alpha1.OrderEntry{
				{
					Group: []corev1alpha1.BuildpackRef{
						{
							BuildpackInfo: corev1alpha1.BuildpackInfo{Id: "io.buildpack.1", Version: "v1"},
							Optional:      false,
						},
						{
							BuildpackInfo: corev1alpha1.BuildpackInfo{Id: "io.buildpack.2", Version: "v2"},
							Optional:      true,
						},
						{
							BuildpackInfo: corev1alpha1.BuildpackInfo{Id: "io.buildpack.4", Version: "v4"},
							Optional:      false,
						},
					},
				},
			})

			assert.Len(t, registryClient.SavedImages(), 2)
			savedImage := registryClient.SavedImages()[builderTag]
			require.Contains(t, registryClient.SavedImages(), relocatedRunImageTag)
			digest, err := registryClient.SavedImages()[relocatedRunImageTag].Digest()
			require.NoError(t, err)
			require.Equal(t, digest.String(), runImageDigest)

			workingDir, err := imagehelpers.GetWorkingDir(savedImage)
			require.NoError(t, err)
			assert.Equal(t, "/layers", workingDir)

			hash, err := savedImage.Digest()
			require.NoError(t, err)
			assert.Equal(t, fmt.Sprintf("%s@%s", builderTag, hash), builderRecord.Image)

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

			lifecycleImgManifest, err := lifecycleImg.Manifest()
			require.NoError(t, err)
			for i := 0; i < lifecycleImageLayers; i++ {
				layerTester.testNextLayer("Lifecycle Layer", func(index int) {
					lifecycleImgLayer, err := lifecycleImg.LayerByDigest(lifecycleImgManifest.Layers[i].Digest)
					require.NoError(t, err)

					assert.Equal(t, layers[index+i], lifecycleImgLayer)
				})
			}

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
						fmt.Sprintf(`[run-image]
  image = "%s@%s"
`, relocatedRunImageTag, runImageDigest),
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

  [[order.group]]
    id = "io.buildpack.4"
    version = "v4"
`}})

			})

			buildpackOrder, err := imagehelpers.GetStringLabel(savedImage, buildpackOrderLabel)
			assert.NoError(t, err)
			assert.JSONEq(t, //language=json
				`[{"group":[{"id":"io.buildpack.1","version":"v1"},{"id":"io.buildpack.2","version":"v2","optional":true},{"id":"io.buildpack.4","version":"v4"}]}]`, buildpackOrder)

			buildpackMetadata, err := imagehelpers.GetStringLabel(savedImage, buildpackMetadataLabel)
			assert.NoError(t, err)
			assert.JSONEq(t, //language=json
				fmt.Sprintf(`{
  "description": "Custom Builder built with kpack",
  "stack": {
    "runImage": {
      "image": "%s@%s",
      "mirrors": null
    }
  },
  "lifecycle": {
    "version": "0.5.0",
    "apis": {
      "buildpack": {
		"deprecated": ["0.2"],
		"supported": ["0.3"]
      },
      "platform": {
        "deprecated": ["0.3"],
        "supported": ["0.4"]
      }
    }
  },
  "createdBy": {
    "name": "kpack Builder",
    "version": "v1.2.3 (git sha: abcdefg123456)"
  },
  "buildpacks": [
	{
      "id": "io.buildpack.4",
      "version": "v4",
	  "homepage": "buildpack.4.com"
    },
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
}`, relocatedRunImageTag, runImageDigest), buildpackMetadata)

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
  },
  "io.buildpack.4": {
    "v4": {
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

			// Assure the loose coupling of the number of labels that should be there
			assert.Equal(t, len(clusterBuilderSpec.AdditionalLabels), 2)
			for key, value := range clusterBuilderSpec.AdditionalLabels {
				additionalLabel, err := imagehelpers.GetStringLabel(savedImage, key)
				assert.NoError(t, err)
				assert.Equal(t, value, additionalLabel)
			}
		})

		it("creates images deterministically ", func() {
			original, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
			require.NoError(t, err)

			for i := 1; i <= 50; i++ {
				other, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)

				require.NoError(t, err)

				require.Equal(t, original.Image, other.Image)
				require.Equal(t, original.Buildpacks, other.Buildpacks)
			}
		})

		when("validating buildpacks", func() {
			it("errors with unsupported stack", func() {
				addBuildpack(t, "io.buildpack.unsupported.stack", "v4", "buildpack.4.com", "0.2",
					[]corev1alpha1.BuildpackStack{
						{
							ID: "io.buildpacks.stacks.unsupported",
						},
					})

				clusterBuilderSpec.Order = []buildapi.BuilderOrderEntry{
					{
						Group: []buildapi.BuilderBuildpackRef{{
							BuildpackRef: corev1alpha1.BuildpackRef{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "io.buildpack.unsupported.stack",
									Version: "v4",
								},
							},
						}},
					},
				}

				_, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
				require.EqualError(t, err, "validating buildpack io.buildpack.unsupported.stack@v4: stack io.buildpacks.stacks.some-stack is not supported")
			})

			it("works with empty stack", func() {
				addBuildpack(t, "io.buildpack.empty.stack", "v4", "buildpack.4.com", "0.2", []corev1alpha1.BuildpackStack{})

				clusterBuilderSpec.Order = []buildapi.BuilderOrderEntry{
					{
						Group: []buildapi.BuilderBuildpackRef{{
							BuildpackRef: corev1alpha1.BuildpackRef{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "io.buildpack.empty.stack",
									Version: "v4",
								},
							},
						}},
					},
				}

				_, err := subject.CreateBuilder(
					ctx,
					builderKeychain,
					stackKeychain,
					lifecycleKeychain,
					fetcher,
					stack,
					clusterLifecycle,
					clusterBuilderSpec,
					[]*corev1.Secret{},
					builderTag,
				)
				require.NoError(t, err)
			})

			it("errors with unsupported mixin", func() {
				addBuildpack(t, "io.buildpack.unsupported.mixin", "v4", "buildpack.1.com", "0.2",
					[]corev1alpha1.BuildpackStack{
						{
							ID:     stackID,
							Mixins: []string{mixin, "something-missing-mixin", "something-missing-mixin2"},
						},
					})

				clusterBuilderSpec.Order = []buildapi.BuilderOrderEntry{{
					Group: []buildapi.BuilderBuildpackRef{{
						BuildpackRef: corev1alpha1.BuildpackRef{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id:      "io.buildpack.unsupported.mixin",
								Version: "v4",
							},
						},
					}},
				}}

				_, err := subject.CreateBuilder(
					ctx,
					builderKeychain,
					stackKeychain,
					lifecycleKeychain,
					fetcher,
					stack,
					clusterLifecycle,
					clusterBuilderSpec,
					[]*corev1.Secret{},
					builderTag,
				)
				require.EqualError(t, err, "validating buildpack io.buildpack.unsupported.mixin@v4: stack missing mixin(s): something-missing-mixin, something-missing-mixin2")
			})

			it("works with relaxed mixin contract", func() {
				clusterLifecycle.Status.ResolvedClusterLifecycle = buildapi.ResolvedClusterLifecycle{
					Version: "0.5.0",
					APIs: buildapi.LifecycleAPIs{
						Buildpack: buildapi.APIVersions{
							Deprecated: []string{"0.2"},
							Supported:  []string{"0.3"},
						},
						Platform: buildapi.APIVersions{
							Deprecated: []string{},
							Supported:  []string{relaxedMixinMinPlatformAPI},
						},
					},
				}

				addBuildpack(t, "io.buildpack.relaxed.mixin", "v4", "buildpack.1.com", "0.2",
					[]corev1alpha1.BuildpackStack{
						{
							ID:     stackID,
							Mixins: []string{mixin, "build:common-mixin", "run:common-mixin", "another-common-mixin"},
						},
					},
				)

				clusterBuilderSpec.Order = []buildapi.BuilderOrderEntry{{
					Group: []buildapi.BuilderBuildpackRef{{
						BuildpackRef: corev1alpha1.BuildpackRef{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id:      "io.buildpack.relaxed.mixin",
								Version: "v4",
							},
						},
					}},
				}}

				_, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
				require.Nil(t, err)
			})

			it("ignores relaxed mixin contract with an older platform api", func() {
				addBuildpack(t, "io.buildpack.relaxed.old.mixin", "v4", "buildpack.1.com", "0.3",
					[]corev1alpha1.BuildpackStack{
						{
							ID:     stackID,
							Mixins: []string{mixin, "build:common-mixin", "run:common-mixin", "another-common-mixin"},
						},
					},
				)

				clusterBuilderSpec.Order = []buildapi.BuilderOrderEntry{{
					Group: []buildapi.BuilderBuildpackRef{{
						BuildpackRef: corev1alpha1.BuildpackRef{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id:      "io.buildpack.relaxed.old.mixin",
								Version: "v4",
							},
						},
					}},
				}}

				_, err := subject.CreateBuilder(ctx, builderKeychain, nil, nil, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
				require.Error(t, err, "validating buildpack io.buildpack.relaxed.old.mixin@v4: stack missing mixin(s): build:common-mixin, run:common-mixin, another-common-mixin")
			})

			it("errors with unsupported buildpack version", func() {
				addBuildpack(t, "io.buildpack.unsupported.buildpack.api", "v4", "buildpack.4.com", "0.1",
					[]corev1alpha1.BuildpackStack{
						{
							ID: stackID,
						},
					})

				clusterBuilderSpec.Order = []buildapi.BuilderOrderEntry{{
					Group: []buildapi.BuilderBuildpackRef{{
						BuildpackRef: corev1alpha1.BuildpackRef{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id:      "io.buildpack.unsupported.buildpack.api",
								Version: "v4",
							},
						},
					}},
				}}

				_, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
				require.EqualError(t, err, "validating buildpack io.buildpack.unsupported.buildpack.api@v4: unsupported buildpack api: 0.1, expecting: 0.2, 0.3")
			})

			it("supports anystack buildpacks", func() {
				clusterLifecycle.Status.ResolvedClusterLifecycle = buildapi.ResolvedClusterLifecycle{
					Version: "0.5.0",
					APIs: buildapi.LifecycleAPIs{
						Buildpack: buildapi.APIVersions{
							Deprecated: []string{"0.2"},
							Supported:  []string{"0.3", "0.4", "0.5"},
						},
						Platform: buildapi.APIVersions{
							Deprecated: []string{"0.3"},
							Supported:  []string{"0.4"},
						},
					},
				}

				addBuildpack(t, "anystack.buildpack", "v1", "buildpacks.com", "0.5",
					[]corev1alpha1.BuildpackStack{
						{
							ID: "*",
						},
					})

				clusterBuilderSpec.Order = []buildapi.BuilderOrderEntry{{
					Group: []buildapi.BuilderBuildpackRef{{
						BuildpackRef: corev1alpha1.BuildpackRef{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id:      "anystack.buildpack",
								Version: "v1",
							},
						},
					}},
				}}

				_, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
				require.NoError(t, err)
			})
		})

		when("validating platform api", func() {
			it("errors if no lifecycle platform api is supported", func() {
				clusterLifecycle.Status.ResolvedClusterLifecycle = buildapi.ResolvedClusterLifecycle{
					Version: "0.5.0",
					APIs: buildapi.LifecycleAPIs{
						Buildpack: buildapi.APIVersions{
							Deprecated: []string{"0.2"},
							Supported:  []string{"0.3"},
						},
						Platform: buildapi.APIVersions{
							Deprecated: []string{"0.1"},
							Supported:  []string{"0.2", "0.999"},
						},
					},
				}

				_, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
				require.EqualError(t, err, "unsupported platform apis in kpack lifecycle: 0.1, 0.2, 0.999, expecting one of: 0.3, 0.4, 0.5, 0.6, 0.7, 0.8")
			})
		})

		when("signing a builder image", func() {
			it("does not populate the signature paths when no secrets were present", func() {
				builderRecord, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{}, builderTag)
				require.NoError(t, err)
				require.NotNil(t, builderRecord)
				require.Empty(t, builderRecord.SignaturePaths)
			})

			it("returns an error if signing fails", func() {
				subject.ImageSigner = &fakeBuilderSigner{
					signBuilder: func(ctx context.Context, s string, secrets []*corev1.Secret, keychain authn.Keychain) ([]buildapi.CosignSignature, error) {
						return nil, fmt.Errorf("failed to sign builder")
					},
				}

				fakeSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cosign-creds",
						Namespace: "test-namespace",
					},
				}

				_, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{&fakeSecret}, builderTag)
				require.Error(t, err)
			})

			it("populates the signature paths when signing succeeds", func() {
				fakeSecret := corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cosign-creds",
						Namespace: "test-namespace",
					},
				}

				subject.ImageSigner = &fakeBuilderSigner{
					signBuilder: func(ctx context.Context, s string, secrets []*corev1.Secret, keychain authn.Keychain) ([]buildapi.CosignSignature, error) {
						return []buildapi.CosignSignature{
							{
								SigningSecret: fmt.Sprintf("k8s://%s/%s", fakeSecret.Namespace, fakeSecret.Name),
								TargetDigest:  "registry.local/test-image:signature-tag",
							},
						}, nil
					},
				}

				builderRecord, err := subject.CreateBuilder(ctx, builderKeychain, stackKeychain, lifecycleKeychain, fetcher, stack, clusterLifecycle, clusterBuilderSpec, []*corev1.Secret{&fakeSecret}, builderTag)
				require.NoError(t, err)
				require.NotNil(t, builderRecord)
				require.NotEmpty(t, builderRecord.SignaturePaths)
			})
		})
	})
}

type fakeBuilderSigner struct {
	signBuilder func(context.Context, string, []*corev1.Secret, authn.Keychain) ([]buildapi.CosignSignature, error)
}

func (s *fakeBuilderSigner) SignBuilder(ctx context.Context, imageReference string, signingSecrets []*corev1.Secret, builderKeychain authn.Keychain) ([]buildapi.CosignSignature, error) {
	return s.signBuilder(ctx, imageReference, signingSecrets, builderKeychain)
}

func buildpackInfoInLayers(buildpackLayers []buildpackLayer, id, version string) DescriptiveBuildpackInfo {
	for _, b := range buildpackLayers {
		if b.BuildpackInfo.Id == id && b.BuildpackInfo.Version == version {
			return b.BuildpackInfo
		}
	}
	panic("unexpected missing buildpack info")
}

type content struct {
	typeflag      byte
	fileContent   string
	uid, gid      int
	mode          int64
	linkname      string
	ignoreModTime bool
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
		if !expectedContent.ignoreModTime {
			require.True(t, header.ModTime.Equal(time.Date(1980, time.January, 1, 0, 0, 1, 0, time.UTC)))
		}
		delete(expectedContents, header.Name)
	}

	for fileName := range expectedContents {
		t.Fatalf("file %s not in layer", fileName)
	}
}

type layerIteratorTester int

func (i *layerIteratorTester) testNextLayer(_ string, test func(index int)) {
	test(int(*i))
	*i++
}

func createRunImage() v1.Image {
	runImg, _ := random.Image(1, int64(5))

	config, _ := runImg.ConfigFile()

	config.OS = "linux"
	runImg, _ = mutate.ConfigFile(runImg, config)

	return runImg
}

func digest(image v1.Image) string {
	d, _ := image.Digest()
	return d.String()
}
