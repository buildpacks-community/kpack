package v1alpha2

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			Tag:                "some/image",
			ServiceAccountName: "some/service-account",
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
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	builder := &TestBuilderResource{
		Name:         "builder-Name",
		LatestImage:  "some/builder@sha256:builder-digest",
		Kind:         BuilderKind,
		BuilderReady: true,
		BuilderMetadata: []corev1alpha1.BuildpackMetadata{
			{Id: "buildpack.matches", Version: "1"},
		},
		LatestRunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
	}

	latestBuild := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: BuildSpec{
			Tags:               []string{"some/image"},
			Builder:            builder.BuildBuilderSpec(),
			ServiceAccountName: "some/serviceaccount",
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
			BuildMetadata: []corev1alpha1.BuildpackMetadata{
				{Id: "buildpack.matches", Version: "1"},
			},
			Stack: corev1alpha1.BuildStack{
				RunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
				ID:       "io.buildpacks.stack.bionic",
			},
			LatestImage: "some.registry.io/built@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
		},
	}

	when("#build", func() {
		sourceResolver.Status.Source = corev1alpha1.ResolvedSourceConfig{
			Git: &corev1alpha1.ResolvedGitSource{
				URL:      "https://some.git/url",
				Revision: "revision",
				Type:     corev1alpha1.Commit,
			},
		}

		latestBuild.Spec.Source = corev1alpha1.SourceConfig{
			Git: &corev1alpha1.Git{
				URL:      "https://some.git/url",
				Revision: "revision",
			},
		}

		it("generates a build name with build number", func() {
			image.Name = "imageName"
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")
			assert.Contains(t, build.Name, "imageName-build-27")
		})

		it("sets builder to be the Builder's resolved latestImage", func() {
			image.Name = "imageName"
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")
			assert.Equal(t, builder.LatestImage, build.Spec.Builder.Image)
		})

		it("sets build priority class correctly", func() {
			build := image.Build(sourceResolver, builder, latestBuild, "some-reasons", "some-changes", 27, "some-class")
			assert.Equal(t, build.Spec.PriorityClassName, "some-class")
			assert.Equal(t, build.PriorityClassName(), "some-class")
		})

		it("propagates image's annotations onto the build", func() {
			build := image.Build(sourceResolver, builder, latestBuild, "some-reasons", "some-changes", 27, "")
			assert.Equal(t, map[string]string{"annotation-key": "annotation-value", "image.kpack.io/buildChanges": "some-changes", "image.kpack.io/reason": "some-reasons", "image.kpack.io/builderKind": "Builder", "image.kpack.io/builderName": "builder-Name"}, build.Annotations)
		})

		it("sets labels from image metadata and propagates image labels", func() {
			image.Generation = 22
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")
			assert.Equal(t, map[string]string{
				"label-key":                      "label-value",
				"image.kpack.io/buildNumber":     "27",
				"image.kpack.io/imageGeneration": "22",
				"image.kpack.io/image":           "image-name"}, build.Labels)
		})

		it("sets git url and git revision when image source is git", func() {
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")
			assert.Contains(t, build.Spec.Source.Git.URL, "https://some.git/url")
			assert.Contains(t, build.Spec.Source.Git.Revision, "revision")
			assert.Nil(t, build.Spec.Source.Blob)
			assert.Nil(t, build.Spec.Source.Registry)
		})

		it("sets blob url when image source is blob", func() {
			sourceResolver.Status.Source = corev1alpha1.ResolvedSourceConfig{
				Blob: &corev1alpha1.ResolvedBlobSource{
					URL: "https://some.place/blob.jar",
				},
			}
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")
			assert.Nil(t, build.Spec.Source.Git)
			assert.Nil(t, build.Spec.Source.Registry)
			assert.Equal(t, build.Spec.Source.Blob.URL, "https://some.place/blob.jar")
		})

		it("sets registry image when image source is registry", func() {
			sourceResolver.Status.Source = corev1alpha1.ResolvedSourceConfig{
				Registry: &corev1alpha1.ResolvedRegistrySource{
					Image: "some-registry.io/some-image",
				},
			}
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")
			assert.Nil(t, build.Spec.Source.Git)
			assert.Nil(t, build.Spec.Source.Blob)
			assert.Equal(t, build.Spec.Source.Registry.Image, "some-registry.io/some-image")
		})

		it("with excludes additional autogenerated tags names when explicitly disabled", func() {
			image.Spec.Tag = "imagename/foo:test"
			image.Spec.ImageTaggingStrategy = corev1alpha1.None
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
			require.Len(t, build.Spec.Tags, 1)
		})

		it("with adds additional tags names", func() {
			image.Spec.Tag = "imagename/foo:test"
			image.Spec.AdditionalTags = []string{"imagename/foo:test2", "anotherimage/foo:test3"}
			image.Spec.ImageTaggingStrategy = corev1alpha1.None
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
			require.Len(t, build.Spec.Tags, 3)
		})

		it("generates a build with default process when set", func() {
			image.Spec.DefaultProcess = "sys-info"
			image.Name = "imageName"
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")
			assert.Equal(t, "sys-info", build.Spec.DefaultProcess)
		})

		when("generates additional image names for a provided build number", func() {
			it("with tag prefix if image name has a tag", func() {
				image.Spec.Tag = "gcr.io/imagename/foo:test"
				build := image.Build(sourceResolver, builder, latestBuild, "", "", 45, "")
				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/foo:test-b45\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})

			it("without tag prefix if image name has no provided tag", func() {
				image.Spec.Tag = "gcr.io/imagename/notags"
				build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/notags:b1\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})

			it("with additional tags if provided", func() {
				image.Spec.Tag = "gcr.io/imagename/tagged:latest"
				image.Spec.AdditionalTags = []string{"imagename/foo:test2", "anotherimage/foo:test3"}
				build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
				require.Len(t, build.Spec.Tags, 4)
				require.Regexp(t, "gcr.io/imagename/tagged:b1\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})

			it("without tag prefix if image name has the tag 'latest' provided", func() {
				image.Spec.Tag = "gcr.io/imagename/tagged:latest"
				build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
				require.Len(t, build.Spec.Tags, 2)
				require.Regexp(t, "gcr.io/imagename/tagged:b1\\.\\d{8}\\.\\d{6}", build.Spec.Tags[1])
			})
		})

		it("generates a build name less than 64 characters", func() {
			image.Name = "long-image-name-1234567890-1234567890-1234567890-1234567890-1234567890"
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
			assert.True(t, len(build.Name) < 64, "expected %s to be less than 64", build.Name)
		})

		it("adds the env vars to the build spec", func() {
			image.Spec.Build = &ImageBuild{
				Env: []corev1.EnvVar{
					{Name: "keyA", Value: "new"},
				},
			}
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
			assert.Equal(t, image.Spec.Build.Env, build.Spec.Env)
		})

		it("adds build reasons and changes annotation", func() {
			reasons := "some reason"
			changes := "some changes"
			build := image.Build(sourceResolver, builder, latestBuild, reasons, changes, 1, "")
			assert.Equal(t, reasons, build.Annotations[BuildReasonAnnotation])
			assert.Equal(t, changes, build.Annotations[BuildChangesAnnotation])
		})

		it("adds stack information", func() {
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
			assert.Equal(t, "some.registry.io/built@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb", build.Spec.LastBuild.Image)
			assert.Equal(t, "io.buildpacks.stack.bionic", build.Spec.LastBuild.StackId)
		})

		it("adds build resources", func() {
			image.Spec.Build = &ImageBuild{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("256M"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1"),
						corev1.ResourceMemory: resource.MustParse("128M"),
					},
				},
			}

			build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
			assert.Equal(t, image.Spec.Build.Resources, build.Spec.Resources)
		})

		it("adds pod configuration", func() {
			buildTimeout := int64(1800)
			image.Spec.Build = &ImageBuild{
				Tolerations:  []corev1.Toleration{{Key: "some-key"}},
				NodeSelector: map[string]string{"foo": "bar"},
				Affinity: &corev1.Affinity{
					NodeAffinity: &corev1.NodeAffinity{},
				},
				BuildTimeout: &buildTimeout,
			}

			build := image.Build(sourceResolver, builder, latestBuild, "", "", 1, "")
			assert.Equal(t, image.Spec.Build.Tolerations, build.Spec.Tolerations)
			assert.Equal(t, image.Spec.Build.NodeSelector, build.Spec.NodeSelector)
			assert.Equal(t, image.Spec.Build.Affinity, build.Spec.Affinity)
			assert.Equal(t, image.Spec.Build.BuildTimeout, build.Spec.ActiveDeadlineSeconds)
		})

		it("sets the notary config when present", func() {
			image.Spec.Notary = &corev1alpha1.NotaryConfig{
				V1: &corev1alpha1.NotaryV1Config{
					URL: "some-notary-server",
					SecretRef: corev1alpha1.NotarySecretRef{
						Name: "some-secret-name",
					},
				},
			}
			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")

			assert.Equal(t, image.Spec.Notary, build.Spec.Notary)
		})

		it("sets the cosign config when present", func() {
			image.Spec.Cosign = &CosignConfig{
				Annotations: []CosignAnnotation{
					{
						Name:  "1",
						Value: "1",
					},
				},
			}

			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")

			assert.Equal(t, image.Spec.Cosign, build.Spec.Cosign)
		})

		it("sets the cosign and notary config when present", func() {
			image.Spec.Notary = &corev1alpha1.NotaryConfig{
				V1: &corev1alpha1.NotaryV1Config{
					URL: "some-notary-server",
					SecretRef: corev1alpha1.NotarySecretRef{
						Name: "some-secret-name",
					},
				},
			}

			image.Spec.Cosign = &CosignConfig{
				Annotations: []CosignAnnotation{
					{
						Name:  "1",
						Value: "1",
					},
				},
			}

			build := image.Build(sourceResolver, builder, latestBuild, "", "", 27, "")

			assert.Equal(t, image.Spec.Notary, build.Spec.Notary)
			assert.Equal(t, image.Spec.Cosign, build.Spec.Cosign)
		})
	})
}

type TestBuilderResource struct {
	BuilderReady     bool
	BuilderMetadata  []corev1alpha1.BuildpackMetadata
	ImagePullSecrets []corev1.LocalObjectReference
	Kind             string
	LatestImage      string
	LatestRunImage   string
	Name             string
}

func (t TestBuilderResource) BuildBuilderSpec() corev1alpha1.BuildBuilderSpec {
	return corev1alpha1.BuildBuilderSpec{
		Image:            t.LatestImage,
		ImagePullSecrets: t.ImagePullSecrets,
	}
}

func (t TestBuilderResource) Ready() bool {
	return t.BuilderReady
}

func (t TestBuilderResource) BuildpackMetadata() corev1alpha1.BuildpackMetadataList {
	return t.BuilderMetadata
}

func (t TestBuilderResource) RunImage() string {
	return t.LatestRunImage
}

func (t TestBuilderResource) GetName() string {
	return t.Name
}

func (t TestBuilderResource) GetKind() string {
	return t.Kind
}
