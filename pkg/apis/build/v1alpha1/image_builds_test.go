package v1alpha1

import (
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestImageBuilds(t *testing.T) {
	spec.Run(t, "Image build Needed", testImageBuilds)
}

func testImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
			Labels: map[string]string{
				"label-key": "label-value",
			},
		},
		Spec: ImageSpec{
			Tag:            "some/image",
			ServiceAccount: "some/service-account",
			Builder: corev1.ObjectReference{
				Kind: "Builder",
				Name: "builder-name",
			},
		},
	}

	sourceResolver := &SourceResolver{
		Status: SourceResolverStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: 0,
				Conditions: []corev1alpha1.Condition{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: v1.ConditionTrue,
					},
				},
			},
		},
	}

	builder := &TestBuilderResource{
		Name:         "builder-Name",
		LatestImage:  "some/builder@sha256:builder-digest",
		BuilderReady: true,
		BuilderMetadata: []BuildpackMetadata{
			{Id: "buildpack.matches", Version: "1"},
		},
		LatestRunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
	}

	latestBuild := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: BuildSpec{
			Tags:           []string{"some/image"},
			Builder:        builder.BuildBuilderSpec(),
			ServiceAccount: "some/serviceaccount",
		},
		Status: BuildStatus{
			Status: corev1alpha1.Status{
				Conditions: corev1alpha1.Conditions{
					{
						Type:   corev1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			BuildMetadata: []BuildpackMetadata{
				{Id: "buildpack.matches", Version: "1"},
			},
			Stack: BuildStack{
				RunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
				ID:       "io.buildpacks.stack.bionic",
			},
			LatestImage: "some.registry.io/built@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
		},
	}

	when("#buildNeeded", func() {
		sourceResolver.Status.Source = ResolvedSourceConfig{
			Git: &ResolvedGitSource{
				URL:      "https://some.git/url",
				Revision: "revision",
				Type:     Commit,
			},
		}

		latestBuild.Spec.Source = SourceConfig{
			Git: &Git{
				URL:      "https://some.git/url",
				Revision: "revision",
			},
		}

		it("false for no changes", func() {
			reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
			assert.False(t, needed)
			require.Len(t, reasons, 0)
		})

		it("true for different image", func() {
			image.Spec.Tag = "different"

			reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
			assert.True(t, needed)
			require.Len(t, reasons, 1)
			assert.Contains(t, reasons, BuildReasonConfig)
		})

		it("false for different ServiceAccount", func() {
			image.Spec.ServiceAccount = "different"

			reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
			assert.False(t, needed)
			require.Len(t, reasons, 0)
		})

		it("true if build env changes", func() {
			latestBuild.Spec.Env = []v1.EnvVar{
				{Name: "keyA", Value: "previous-value"},
			}

			reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
			assert.True(t, needed)
			require.Len(t, reasons, 1)
			assert.Contains(t, reasons, BuildReasonConfig)
		})

		it("false if last build failed but no spec changes", func() {
			latestBuild.Status = BuildStatus{
				Status: corev1alpha1.Status{
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionSucceeded,
							Status: corev1.ConditionFalse,
						},
					},
				},
				Stack: BuildStack{},
			}

			reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
			assert.False(t, needed)
			require.Len(t, reasons, 0)
		})

		it("true if build is annotated additional build needed", func() {
			latestBuild.Annotations = map[string]string{
				BuildNeededAnnotation: time.Now().String(),
			}

			reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
			assert.True(t, needed)
			require.Len(t, reasons, 1)
			assert.Contains(t, reasons, BuildReasonTrigger)
		})

		when("Builder Metadata changes", func() {
			it("false if builder has additional unused buildpacks", func() {
				builder.BuilderMetadata = []BuildpackMetadata{
					{Id: "buildpack.matches", Version: "1"},
					{Id: "buildpack.unused", Version: "unused"},
				}

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("true if builder metadata has different buildpack version from used buildpack version", func() {
				builder.BuilderMetadata = []BuildpackMetadata{
					{Id: "buildpack.matches", Version: "NEW_VERSION"},
					{Id: "buildpack.different", Version: "different"},
				}

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonBuildpack)
			})

			it("true if builder does not have all most recent used buildpacks", func() {
				builder.BuilderMetadata = []BuildpackMetadata{
					{Id: "buildpack.only.new.buildpacks", Version: "1"},
					{Id: "buildpack.only.new.or.unused.buildpacks", Version: "1"},
				}

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonBuildpack)
			})

			it("true if builder has a different run image", func() {
				builder.LatestRunImage = "some.registry.io/run-image@sha256:a1aa3da2a80a775df55e880b094a1a8de19b919435ad0c71c29a0983d64e65db"

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonStack)
			})
		})

		when("Git", func() {
			it("true for different GitURL", func() {
				sourceResolver.Status.Source.Git.URL = "different"

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different Git SubPath", func() {
				sourceResolver.Status.Source.Git.SubPath = "different"

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different GitRevision", func() {
				sourceResolver.Status.Source.Git.Revision = "different"

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonCommit)
			})

			it("false if source resolver is not ready", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: v1.ConditionFalse,
					}}

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if builder is not ready", func() {
				sourceResolver.Status.Source.Git.URL = "some-change"
				builder.BuilderReady = false

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not resolved", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{}

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not resolved and there is no previous build", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{}

				reasons, needed := image.buildNeeded(nil, sourceResolver, builder)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not processed current generation", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.ObjectMeta.Generation = 2
				sourceResolver.Status.ObservedGeneration = 1

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.False(t, needed)
				require.Len(t, reasons, 0)
			})

			it("true if build env order changes and git url changes", func() {
				latestBuild.Spec.Source.Git.URL = "old-git.com/url"
				latestBuild.Spec.Env = []v1.EnvVar{
					{Name: "keyA", Value: "old"},
				}
				image.Spec.Build = &ImageBuild{
					Env: []v1.EnvVar{
						{Name: "keyA", Value: "new"},
					},
				}

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
			})

			it("true if both config and commit have changed", func() {
				sourceResolver.Status.Source.Git.URL = "different"
				sourceResolver.Status.Source.Git.Revision = "different"

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 2)
				assert.Contains(t, reasons, BuildReasonConfig)
				assert.Contains(t, reasons, BuildReasonCommit)
			})
		})

		when("Blob", func() {
			sourceResolver.Status.Source = ResolvedSourceConfig{
				Blob: &ResolvedBlobSource{
					URL: "different",
				},
			}

			latestBuild.Spec.Source = SourceConfig{
				Blob: &Blob{
					URL: "some-url",
				},
			}

			it("true for different BlobURL", func() {
				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different Blob SubPath", func() {
				sourceResolver.Status.Source.Blob.SubPath = "different"

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})
		})

		when("Registry", func() {
			sourceResolver.Status.Source = ResolvedSourceConfig{
				Registry: &ResolvedRegistrySource{
					Image: "different",
				},
			}

			latestBuild.Spec.Source = SourceConfig{
				Registry: &Registry{
					Image: "some-image",
				},
			}

			it("true for different RegistryImage", func() {
				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})

			it("true for different Registry SubPath", func() {
				sourceResolver.Status.Source.Registry.SubPath = "different"

				reasons, needed := image.buildNeeded(latestBuild, sourceResolver, builder)
				assert.True(t, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, BuildReasonConfig)
			})
		})
	})

	when("#build", func() {
		sourceResolver.Status.Source = ResolvedSourceConfig{
			Git: &ResolvedGitSource{
				URL:      "https://some.git/url",
				Revision: "revision",
				Type:     Commit,
			},
		}

		latestBuild.Spec.Source = SourceConfig{
			Git: &Git{
				URL:      "https://some.git/url",
				Revision: "revision",
			},
		}

		it("generates a build name with build number", func() {
			image.Name = "imageName"

			build := image.build(sourceResolver, builder, latestBuild, []string{}, 27)

			assert.Contains(t, build.GenerateName, "imageName-build-27-")
		})

		it("sets builder to be the Builder's resolved latestImage", func() {
			image.Name = "imageName"

			build := image.build(sourceResolver, builder, latestBuild, []string{}, 27)

			assert.Equal(t, builder.LatestImage, build.Spec.Builder.Image)
		})

		it("propagates image's annotations onto the build", func() {
			build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 27)
			assert.Equal(t, map[string]string{"annotation-key": "annotation-value", "image.build.pivotal.io/reason": "CONFIG"}, build.Annotations)
		})

		it("sets builder to be the Builder's resolved latestImage", func() {
			build := image.build(sourceResolver, builder, latestBuild, []string{}, 27)
			assert.Equal(t, map[string]string{"label-key": "label-value", "image.build.pivotal.io/buildNumber": "27", "image.build.pivotal.io/image": "image-name"}, build.Labels)
		})

		it("sets git url and git revision when image source is git", func() {
			build := image.build(sourceResolver, builder, latestBuild, []string{}, 27)

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
			build := image.build(sourceResolver, builder, latestBuild, []string{}, 27)

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
			build := image.build(sourceResolver, builder, latestBuild, []string{}, 27)

			assert.Nil(t, build.Spec.Source.Git)
			assert.Nil(t, build.Spec.Source.Blob)
			assert.Equal(t, build.Spec.Source.Registry.Image, "some-registry.io/some-image")
		})

		it("with excludes additional tags names when explicitly disabled", func() {
			image.Spec.Tag = "imagename/foo:test"
			image.Spec.ImageTaggingStrategy = None
			build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 1)
			require.Len(t, build.Spec.Tags, 1)
		})

		when("generates additional image names for a provided build number", func() {
			it("with tag prefix if image name has a tag", func() {
				image.Spec.Tag = "gcr.io/imagename/foo:test"
				build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 45)
				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/foo:test-b45\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})

			it("without tag prefix if image name has no provided tag", func() {
				image.Spec.Tag = "gcr.io/imagename/notags"
				build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 1)

				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/notags:b1\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})

			it("without tag prefix if image name has the tag 'latest' provided", func() {
				image.Spec.Tag = "gcr.io/imagename/tagged:latest"
				build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 1)

				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/tagged:b1\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})
		})

		it("generates a build name less than 64 characters", func() {
			image.Name = "long-image-name-1234567890-1234567890-1234567890-1234567890-1234567890"

			build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 1)

			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
		})

		it("adds the env vars to the build spec", func() {
			image.Spec.Build = &ImageBuild{
				Env: []v1.EnvVar{
					{Name: "keyA", Value: "new"},
				},
			}

			build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 1)

			assert.Equal(t, image.Spec.Build.Env, build.Spec.Env)
		})

		it("adds build reasons annotation", func() {
			build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig, BuildReasonCommit}, 1)

			assert.Equal(t, "CONFIG,COMMIT", build.Annotations[BuildReasonAnnotation])
		})

		it("adds stack information", func() {
			build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig, BuildReasonCommit}, 1)

			assert.Equal(t, "some.registry.io/built@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb", build.Spec.LastBuild.Image)
			assert.Equal(t, "io.buildpacks.stack.bionic", build.Spec.LastBuild.StackId)
		})

		it("adds build resources", func() {
			image.Spec.Build = &ImageBuild{
				Resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("2"),
						v1.ResourceMemory: resource.MustParse("256M"),
					},
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse("1"),
						v1.ResourceMemory: resource.MustParse("128M"),
					},
				},
			}

			build := image.build(sourceResolver, builder, latestBuild, []string{BuildReasonConfig}, 1)

			assert.Equal(t, image.Spec.Build.Resources, build.Spec.Resources)
		})
	})
}

type TestBuilderResource struct {
	BuilderReady     bool
	BuilderMetadata  []BuildpackMetadata
	ImagePullSecrets []corev1.LocalObjectReference
	LatestImage      string
	LatestRunImage   string
	Name             string
}

func (t TestBuilderResource) BuildBuilderSpec() BuildBuilderSpec {
	return BuildBuilderSpec{
		Image:            t.LatestImage,
		ImagePullSecrets: t.ImagePullSecrets,
	}
}

func (t TestBuilderResource) Ready() bool {
	return t.BuilderReady
}

func (t TestBuilderResource) BuildpackMetadata() BuildpackMetadataList {
	return t.BuilderMetadata
}

func (t TestBuilderResource) RunImage() string {
	return t.LatestRunImage
}

func (t TestBuilderResource) GetName() string {
	return t.Name
}
