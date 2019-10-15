package v1alpha1

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

func TestImageBuilds(t *testing.T) {
	spec.Run(t, "Image build Needed", testImageBuilds)
}

func testImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: ImageSpec{
			Tag:            "some/image",
			ServiceAccount: "some/service-account",
			Builder: ImageBuilder{
				Name: "builder-name",
			},
			Build: ImageBuild{
				Env: []v1.EnvVar{
					{
						Name:  "keyA",
						Value: "ValueA",
					},
					{
						Name:  "keyB",
						Value: "ValueB",
					},
				},
			},
		},
	}

	sourceResolver := &SourceResolver{
		Status: SourceResolverStatus{
			Status: duckv1alpha1.Status{
				ObservedGeneration: 0,
				Conditions: []duckv1alpha1.Condition{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: v1.ConditionTrue,
					},
				},
			},
		},
	}

	builder := &Builder{
		ObjectMeta: metav1.ObjectMeta{
			Name: "builder-name",
		},
		Spec: BuilderWithSecretsSpec{
			BuilderSpec:      BuilderSpec{Image: "some/builder"},
			ImagePullSecrets: nil,
		},
		Status: BuilderStatus{
			Status: duckv1alpha1.Status{
				Conditions: duckv1alpha1.Conditions{
					{
						Type:               duckv1alpha1.ConditionReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: apis.VolatileTime{Inner: metav1.Now()},
					},
				},
			},
			BuilderMetadata: []BuildpackMetadata{
				{ID: "buildpack.matches", Version: "1"},
			},
			LatestImage: "some/builder@sha256:builder-digest",
			Stack: BuildStack{
				RunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
				ID:       "io.buildpacks.stack.bionic",
			},
		},
	}

	build := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: BuildSpec{
			Tags:           []string{"some/image"},
			Builder:        builder.BuildBuilderSpec(),
			ServiceAccount: "some/serviceaccount",
			Env: []v1.EnvVar{
				{
					Name:  "keyA",
					Value: "ValueA",
				},
				{
					Name:  "keyB",
					Value: "ValueB",
				},
			},
		},
		Status: BuildStatus{
			Stack: BuildStack{
				RunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
				ID:       "io.buildpacks.stack.bionic",
			},
			BuildMetadata: []BuildpackMetadata{
				{ID: "buildpack.matches", Version: "1"},
			},
		},
	}

	when("#buildNeeded", func() {
		when("Git", func() {
			it.Before(func() {
				sourceResolver.Status.Source = ResolvedSourceConfig{
					Git: &ResolvedGitSource{
						URL:      "https://some.git/url",
						Revision: "revision",
						Type:     Commit,
					},
				}

				build.Spec.Source = SourceConfig{
					Git: &Git{
						URL:      "https://some.git/url",
						Revision: "revision",
					},
				}
			})

			it("false for no changes", func() {
				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("true for different image", func() {
				image.Spec.Tag = "different"

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different GitURL", func() {
				sourceResolver.Status.Source.Git.URL = "different"

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different Git SubPath", func() {
				sourceResolver.Status.Source.Git.SubPath = "different"

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different GitRevision", func() {
				sourceResolver.Status.Source.Git.Revision = "different"

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonCommit)
			})

			it("false if source resolver is not ready", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []duckv1alpha1.Condition{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: v1.ConditionFalse,
					}}

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if builder has not processed", func() {
				sourceResolver.Status.Source.Git.URL = "some-change"
				builder.Status.Conditions = nil

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if builder is not ready", func() {
				sourceResolver.Status.Source.Git.URL = "some-change"
				builder.Status.Conditions = []duckv1alpha1.Condition{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: v1.ConditionFalse,
					},
				}

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if builder has not processed current generation", func() {
				sourceResolver.Status.Source.Git.URL = "some-change"
				builder.ObjectMeta.Generation = 2
				builder.Status.ObservedGeneration = 1
				builder.Status.Conditions = []duckv1alpha1.Condition{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: v1.ConditionTrue,
					},
				}

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not resolved", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []duckv1alpha1.Condition{}

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not resolved and there is no previous build", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []duckv1alpha1.Condition{}

				reasons, needed, err := image.buildNeeded(nil, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not processed current generation", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.ObjectMeta.Generation = 2
				sourceResolver.Status.ObservedGeneration = 1

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false for different ServiceAccount", func() {
				image.Spec.ServiceAccount = "different"

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("true if build env changes", func() {
				build.Spec.Env = []v1.EnvVar{
					{Name: "keyA", Value: "previous-value"},
				}

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true if build env order changes and git url changes", func() {
				build.Spec.Source.Git.URL = "old-git.com/url"
				build.Spec.Env = []v1.EnvVar{
					{Name: "keyA", Value: "old"},
				}
				image.Spec.Build.Env = []v1.EnvVar{
					{Name: "keyA", Value: "new"},
				}

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
			})

			it("true if build resources change", func() {
				build.Spec.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("256M"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("1"),
						v1.ResourceMemory: resource.MustParse("128M"),
					},
				}

				image.Spec.Build.Resources = v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("3"),
						v1.ResourceMemory: resource.MustParse("512M"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("256M"),
					},
				}

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			when("Builder Metadata changes", func() {
				it("false if builder has additional unused buildpack metadata", func() {
					builder.Status.BuilderMetadata = []BuildpackMetadata{
						{ID: "buildpack.matches", Version: "1"},
						{ID: "buildpack.unused", Version: "unused"},
					}

					reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
					require.NoError(t, err)
					assert.False(t, needed)
					require.Len(t, reasons, 0)
				})

				it("true if builder metadata has different buildpack from used buildpack", func() {
					builder.Status.BuilderMetadata = []BuildpackMetadata{
						{ID: "buildpack.matches", Version: "NEW_VERSION"},
						{ID: "buildpack.different", Version: "different"},
					}

					reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
					require.NoError(t, err)
					assert.True(t, needed)
					require.Len(t, reasons, 1)
					assert.Contains(t, reasons, BuildReasonBuildpack)
				})

				it("true if builder has a different stack", func() {
					builder.Status.Stack.ID = "io.buildpacks.cflinuxfs3"

					reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
					require.NoError(t, err)
					assert.True(t, needed)
					require.Len(t, reasons, 1)
					assert.Contains(t, reasons, BuildReasonStackUpdate)
				})

				it("true if builder has a different run image", func() {
					builder.Status.Stack.RunImage = "some.registry.io/run-image@sha256:a1aa3da2a80a775df55e880b094a1a8de19b919435ad0c71c29a0983d64e65db"

					reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
					require.NoError(t, err)
					assert.True(t, needed)
					require.Len(t, reasons, 1)
					assert.Contains(t, reasons, BuildReasonStack)
				})

				it("true if builder does not have all most recent used buildpacks and is not currently building", func() {
					builder.Status.BuilderMetadata = []BuildpackMetadata{
						{ID: "buildpack.only.new.buildpacks", Version: "1"},
						{ID: "buildpack.only.new.or.unused.buildpacks", Version: "1"},
					}

					reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
					require.NoError(t, err)
					assert.True(t, needed)
					require.Len(t, reasons, 1)
					assert.Contains(t, reasons, BuildReasonBuildpack)
				})

				it("true if both config and commit have changed", func() {
					sourceResolver.Status.Source.Git.URL = "different"
					sourceResolver.Status.Source.Git.Revision = "different"

					reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
					require.NoError(t, err)
					assert.True(t, needed)
					require.Len(t, reasons, 2)
					assert.Contains(t, reasons, BuildReasonConfig)
					assert.Contains(t, reasons, BuildReasonCommit)
				})
			})
		})

		when("Blob", func() {
			it.Before(func() {
				sourceResolver.Status.Source = ResolvedSourceConfig{
					Blob: &ResolvedBlobSource{
						URL: "different",
					},
				}

				build.Spec.Source = SourceConfig{
					Blob: &Blob{
						URL: "some-url",
					},
				}
			})

			it("true for different BlobURL", func() {
				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different Blob SubPath", func() {
				sourceResolver.Status.Source.Blob.SubPath = "different"

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})
		})

		when("Registry", func() {
			it.Before(func() {
				sourceResolver.Status.Source = ResolvedSourceConfig{
					Registry: &ResolvedRegistrySource{
						Image: "different",
					},
				}

				build.Spec.Source = SourceConfig{
					Registry: &Registry{
						Image: "some-image",
					},
				}
			})

			it("true for different RegistryImage", func() {
				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different Registry SubPath", func() {
				sourceResolver.Status.Source.Registry.SubPath = "different"

				reasons, needed, err := image.buildNeeded(build, sourceResolver, builder)
				require.NoError(t, err)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})
		})
	})

	when("#build", func() {
		it.Before(func() {
			sourceResolver.Status.Source = ResolvedSourceConfig{
				Git: &ResolvedGitSource{
					URL:      "https://some.git/url",
					Revision: "revision",
					Type:     Commit,
				},
			}

			build.Spec.Source = SourceConfig{
				Git: &Git{
					URL:      "https://some.git/url",
					Revision: "revision",
				},
			}
		})

		it("generates a build name with build number", func() {
			image.Name = "imageName"

			build := image.build(sourceResolver, builder, []string{}, 27)

			assert.Contains(t, build.GenerateName, "imageName-build-27-")
		})

		it("sets builder to be the Builder's resolved latestImage", func() {
			image.Name = "imageName"

			build := image.build(sourceResolver, builder, []string{}, 27)

			assert.Equal(t, builder.Status.LatestImage, build.Spec.Builder.Image)
		})

		it("sets git url and git revision when image source is git", func() {
			build := image.build(sourceResolver, builder, []string{}, 27)

			assert.Contains(t, build.Spec.Source.Git.URL, "https://some.git/url")
			assert.Contains(t, build.Spec.Source.Git.Revision, "revision")
			assert.Nil(t, build.Spec.Source.Blob)
			assert.Nil(t, build.Spec.Source.Registry)
		})

		it("sets blob url when image source is blob", func() {
			sourceResolver.Status.Source = ResolvedSourceConfig{
				Blob: &ResolvedBlobSource{
					URL: "https://some.place/blob.jar",
				},
			}
			build := image.build(sourceResolver, builder, []string{}, 27)

			assert.Nil(t, build.Spec.Source.Git)
			assert.Nil(t, build.Spec.Source.Registry)
			assert.Equal(t, build.Spec.Source.Blob.URL, "https://some.place/blob.jar")
		})

		it("sets registry image when image source is registry", func() {
			sourceResolver.Status.Source = ResolvedSourceConfig{
				Registry: &ResolvedRegistrySource{
					Image: "some-registry.io/some-image",
				},
			}
			build := image.build(sourceResolver, builder, []string{}, 27)

			assert.Nil(t, build.Spec.Source.Git)
			assert.Nil(t, build.Spec.Source.Blob)
			assert.Equal(t, build.Spec.Source.Registry.Image, "some-registry.io/some-image")
		})

		it("with excludes additional tags names when explicitly disabled", func() {
			image.Spec.Tag = "imagename/foo:test"
			image.Spec.ImageTaggingStrategy = None
			build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)
			require.Len(t, build.Spec.Tags, 1)
		})

		when("generates additional image names for a provided build number", func() {
			it("with tag prefix if image name has a tag", func() {
				image.Spec.Tag = "gcr.io/imagename/foo:test"
				build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 45)
				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/foo:test-b45\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})

			it("without tag prefix if image name has no provided tag", func() {
				image.Spec.Tag = "gcr.io/imagename/notags"
				build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/notags:b1\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})

			it("without tag prefix if image name has the tag 'latest' provided", func() {
				image.Spec.Tag = "gcr.io/imagename/tagged:latest"
				build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/tagged:b1\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})
		})

		it("generates a build name less than 64 characters", func() {
			image.Name = "long-image-name-1234567890-1234567890-1234567890-1234567890-1234567890"

			build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
		})

		it("adds the env vars to the build spec", func() {
			build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

			assert.Equal(t, image.Spec.Build.Env, build.Spec.Env)
		})

		it("adds build reasons annotation", func() {
			build := image.build(sourceResolver, builder, []string{BuildReasonConfig, BuildReasonCommit}, 1)

			assert.Equal(t, "CONFIG,COMMIT", build.Annotations[BuildReasonAnnotation])
		})

		it("adds build resources", func() {
			image.Spec.Build.Resources = v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("2"),
					v1.ResourceMemory: resource.MustParse("256M"),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("1"),
					v1.ResourceMemory: resource.MustParse("128M"),
				},
			}

			build := image.build(sourceResolver, builder, []string{BuildReasonConfig}, 1)

			assert.Equal(t, image.Spec.Build.Resources, build.Spec.Resources)
		})
	})
}
