package image

import (
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestImageBuilds(t *testing.T) {
	spec.Run(t, "Image build Needed", testImageBuilds)
}

func testImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &v1alpha1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
			Labels: map[string]string{
				"label-key": "label-value",
			},
		},
		Spec: v1alpha1.ImageSpec{
			Tag:            "some/image",
			ServiceAccount: "some/service-account",
			Builder: corev1.ObjectReference{
				Kind: "Builder",
				Name: "builder-name",
			},
		},
	}

	sourceResolver := &v1alpha1.SourceResolver{
		Status: v1alpha1.SourceResolverStatus{
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
		BuilderReady: true,
		BuilderMetadata: []v1alpha1.BuildpackMetadata{
			{Id: "buildpack.matches", Version: "1"},
		},
		LatestRunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
	}

	latestBuild := &v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: v1alpha1.BuildSpec{
			Tags:           []string{"some/image"},
			Builder:        builder.BuildBuilderSpec(),
			ServiceAccount: "some/serviceaccount",
		},
		Status: v1alpha1.BuildStatus{
			Status: corev1alpha1.Status{
				Conditions: corev1alpha1.Conditions{
					{
						Type:   corev1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					},
				},
			},
			BuildMetadata: []v1alpha1.BuildpackMetadata{
				{Id: "buildpack.matches", Version: "1"},
			},
			Stack: v1alpha1.BuildStack{
				RunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
				ID:       "io.buildpacks.stack.bionic",
			},
			LatestImage: "some.registry.io/built@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
		},
	}

	when("#buildNeeded", func() {
		sourceResolver.Status.Source = v1alpha1.ResolvedSourceConfig{
			Git: &v1alpha1.ResolvedGitSource{
				URL:      "https://some.git/url",
				Revision: "revision",
				Type:     v1alpha1.Commit,
			},
		}

		latestBuild.Spec.Source = v1alpha1.SourceConfig{
			Git: &v1alpha1.Git{
				URL:      "https://some.git/url",
				Revision: "revision",
			},
		}

		it("false for no changes", func() {
			reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
			assert.Equal(t, corev1.ConditionFalse, needed)
			require.Len(t, reasons, 0)
		})

		it("true for different image", func() {
			image.Spec.Tag = "different"

			reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
			assert.Equal(t, corev1.ConditionTrue, needed)
			require.Len(t, reasons, 1)
			assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
		})

		it("false for different ServiceAccount", func() {
			image.Spec.ServiceAccount = "different"

			reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
			assert.Equal(t, corev1.ConditionFalse, needed)
			require.Len(t, reasons, 0)
		})

		it("true if build env changes", func() {
			latestBuild.Spec.Env = []corev1.EnvVar{
				{Name: "keyA", Value: "previous-value"},
			}

			reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
			assert.Equal(t, corev1.ConditionTrue, needed)
			require.Len(t, reasons, 1)
			assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
		})

		it("true if build bindings changes", func() {
			latestBuild.Spec.Bindings = v1alpha1.Bindings{
				{
					Name: "some-old-value",
					MetadataRef: &corev1.LocalObjectReference{
						Name: "some-old-config-map",
					},
				},
			}

			reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
			assert.Equal(t, corev1.ConditionTrue, needed)
			require.Len(t, reasons, 1)
			assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
		})

		it("false if last build failed but no spec changes", func() {
			latestBuild.Status = v1alpha1.BuildStatus{
				Status: corev1alpha1.Status{
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionSucceeded,
							Status: corev1.ConditionFalse,
						},
					},
				},
				Stack: v1alpha1.BuildStack{},
			}

			reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
			assert.Equal(t, corev1.ConditionFalse, needed)
			require.Len(t, reasons, 0)
		})

		it("true if build is annotated additional build needed", func() {
			latestBuild.Annotations = map[string]string{
				v1alpha1.BuildNeededAnnotation: time.Now().String(),
			}

			reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
			assert.Equal(t, corev1.ConditionTrue, needed)
			require.Len(t, reasons, 1)
			assert.Contains(t, reasons, v1alpha1.BuildReasonTrigger)
		})

		when("Builder Metadata changes", func() {
			it("false if builder has additional unused buildpacks", func() {
				builder.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{Id: "buildpack.matches", Version: "1"},
					{Id: "buildpack.unused", Version: "unused"},
				}

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionFalse, needed)
				require.Len(t, reasons, 0)
			})

			it("true if builder metadata has different buildpack version from used buildpack version", func() {
				builder.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{Id: "buildpack.matches", Version: "NEW_VERSION"},
					{Id: "buildpack.different", Version: "different"},
				}

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonBuildpack)
			})

			it("true if builder does not have all most recent used buildpacks", func() {
				builder.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{Id: "buildpack.only.new.buildpacks", Version: "1"},
					{Id: "buildpack.only.new.or.unused.buildpacks", Version: "1"},
				}

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonBuildpack)
			})

			it("true if builder has a different run image", func() {
				builder.LatestRunImage = "some.registry.io/run-image@sha256:a1aa3da2a80a775df55e880b094a1a8de19b919435ad0c71c29a0983d64e65db"

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonStack)
			})
		})

		when("Git", func() {
			it("true for different GitURL", func() {
				sourceResolver.Status.Source.Git.URL = "different"

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
			})

			it("true for different Git SubPath", func() {
				sourceResolver.Status.Source.Git.SubPath = "different"

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
			})

			it("true for different GitRevision", func() {
				sourceResolver.Status.Source.Git.Revision = "different"

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonCommit)
			})

			it("false if source resolver is not ready", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: corev1.ConditionFalse,
					}}

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionUnknown, needed)
				require.Len(t, reasons, 0)
			})

			it("false if builder is not ready", func() {
				sourceResolver.Status.Source.Git.URL = "some-change"
				builder.BuilderReady = false

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionUnknown, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not resolved", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{}

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionUnknown, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not resolved and there is no previous build", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{}

				reasons, needed := buildNeeded(image, nil, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionUnknown, needed)
				require.Len(t, reasons, 0)
			})

			it("false if source resolver has not processed current generation", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.ObjectMeta.Generation = 2
				sourceResolver.Status.ObservedGeneration = 1

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionUnknown, needed)
				require.Len(t, reasons, 0)
			})

			it("true if build env order changes and git url changes", func() {
				latestBuild.Spec.Source.Git.URL = "old-git.com/url"
				latestBuild.Spec.Env = []corev1.EnvVar{
					{Name: "keyA", Value: "old"},
				}
				image.Spec.Build = &v1alpha1.ImageBuild{
					Env: []corev1.EnvVar{
						{Name: "keyA", Value: "new"},
					},
				}

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
			})

			it("true if both config and commit have changed", func() {
				sourceResolver.Status.Source.Git.URL = "different"
				sourceResolver.Status.Source.Git.Revision = "different"

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 2)
				assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
				assert.Contains(t, reasons, v1alpha1.BuildReasonCommit)
			})
		})

		when("Blob", func() {
			sourceResolver.Status.Source = v1alpha1.ResolvedSourceConfig{
				Blob: &v1alpha1.ResolvedBlobSource{
					URL: "different",
				},
			}

			latestBuild.Spec.Source = v1alpha1.SourceConfig{
				Blob: &v1alpha1.Blob{
					URL: "some-url",
				},
			}

			it("true for different BlobURL", func() {
				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
			})

			it("true for different Blob SubPath", func() {
				sourceResolver.Status.Source.Blob.SubPath = "different"

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
			})
		})

		when("Registry", func() {
			sourceResolver.Status.Source = v1alpha1.ResolvedSourceConfig{
				Registry: &v1alpha1.ResolvedRegistrySource{
					Image: "different",
				},
			}

			latestBuild.Spec.Source = v1alpha1.SourceConfig{
				Registry: &v1alpha1.Registry{
					Image: "some-image",
				},
			}

			it("true for different RegistryImage", func() {
				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
			})

			it("true for different Registry SubPath", func() {
				sourceResolver.Status.Source.Registry.SubPath = "different"

				reasons, needed := buildNeeded(image, latestBuild, sourceResolver, builder)
				assert.Equal(t, corev1.ConditionTrue, needed)
				require.Len(t, reasons, 1)
				assert.Contains(t, reasons, v1alpha1.BuildReasonConfig)
			})
		})
	})
}

type TestBuilderResource struct {
	BuilderReady     bool
	BuilderMetadata  []v1alpha1.BuildpackMetadata
	ImagePullSecrets []corev1.LocalObjectReference
	LatestImage      string
	LatestRunImage   string
	Name             string
}

func (t TestBuilderResource) BuildBuilderSpec() v1alpha1.BuildBuilderSpec {
	return v1alpha1.BuildBuilderSpec{
		Image:            t.LatestImage,
		ImagePullSecrets: t.ImagePullSecrets,
	}
}

func (t TestBuilderResource) Ready() bool {
	return t.BuilderReady
}

func (t TestBuilderResource) BuildpackMetadata() v1alpha1.BuildpackMetadataList {
	return t.BuilderMetadata
}

func (t TestBuilderResource) RunImage() string {
	return t.LatestRunImage
}

func (t TestBuilderResource) GetName() string {
	return t.Name
}
