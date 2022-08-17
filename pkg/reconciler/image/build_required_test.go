package image

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestImageBuilds(t *testing.T) {
	spec.Run(t, "Image build Needed", testImageBuilds)
}

func testImageBuilds(t *testing.T, when spec.G, it spec.S) {
	image := &buildapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
			Annotations: map[string]string{
				"annotation-key": "annotation-value",
			},
			Labels: map[string]string{
				"label-key": "label-value",
			},
		},
		Spec: buildapi.ImageSpec{
			Tag:                "some/image",
			ServiceAccountName: "some/service-account",
			Builder: corev1.ObjectReference{
				Kind: "Builder",
				Name: "builder-name",
			},
		},
	}

	sourceResolver := &buildapi.SourceResolver{
		Status: buildapi.SourceResolverStatus{
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
		BuilderMetadata: []corev1alpha1.BuildpackMetadata{
			{Id: "buildpack.matches", Version: "1"},
		},
		LatestRunImage: "some.registry.io/run-image@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
		Kind:           buildapi.BuilderKind,
	}

	latestBuild := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-name",
		},
		Spec: buildapi.BuildSpec{
			Tags:               []string{"some/image"},
			Builder:            builder.BuildBuilderSpec(),
			ServiceAccountName: "some/serviceaccount",
		},
		Status: buildapi.BuildStatus{
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

	when("#buildNeeded", func() {
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

		it("false for no changes", func() {
			result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
			assert.NoError(t, err)
			assert.Equal(t, corev1.ConditionFalse, result.ConditionStatus)
			assert.Equal(t, "", result.ReasonsStr)
			assert.Equal(t, "", result.ChangesStr)
			assert.Equal(t, "", result.PriorityClass)
		})

		it("false for different ServiceAccount", func() {
			image.Spec.ServiceAccountName = "different"

			result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
			assert.NoError(t, err)
			assert.Equal(t, corev1.ConditionFalse, result.ConditionStatus)
			assert.Equal(t, "", result.ReasonsStr)
			assert.Equal(t, "", result.ChangesStr)
			assert.Equal(t, "", result.PriorityClass)
		})

		it("true if build env changes", func() {
			latestBuild.Spec.Env = []corev1.EnvVar{
				{Name: "keyA", Value: "previous-value"},
			}

			expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "env": [
        {
          "name": "keyA",
          "value": "previous-value"
        }
      ],
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    }
  }
]`)

			result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
			assert.NoError(t, err)
			assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
			assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
			assert.Equal(t, expectedChanges, result.ChangesStr)
			assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
		})

		it("true if build service bindings changes", func() {
			latestBuild.Spec.Services = buildapi.Services{
				{
					Name: "some-value",
				},
			}

			expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "services": [
        {
          "name": "some-value"
        }
      ],
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    }
  }
]`)

			result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
			assert.NoError(t, err)
			assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
			assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
			assert.Equal(t, expectedChanges, result.ChangesStr)
			assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
		})

		it("true if build cnb bindings changes", func() {
			latestBuild.Spec.CNBBindings = corev1alpha1.CNBBindings{
				{
					Name: "some-value",
				},
			}

			expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "cnbBindings": [
        {
          "name": "some-value"
        }
      ],
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    }
  }
]`)

			result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
			assert.NoError(t, err)
			assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
			assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
			assert.Equal(t, expectedChanges, result.ChangesStr)
			assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
		})

		it("false if last build failed but no spec changes", func() {
			latestBuild.Status = buildapi.BuildStatus{
				Status: corev1alpha1.Status{
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionSucceeded,
							Status: corev1.ConditionFalse,
						},
					},
				},
				Stack: corev1alpha1.BuildStack{},
			}

			result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
			assert.NoError(t, err)
			assert.Equal(t, corev1.ConditionFalse, result.ConditionStatus)
			assert.Equal(t, "", result.ReasonsStr)
			assert.Equal(t, "", result.ChangesStr)
			assert.Equal(t, "", result.PriorityClass)
		})

		it("true if build is annotated additional build needed", func() {
			latestBuild.Annotations = map[string]string{
				buildapi.BuildNeededAnnotation: "true",
			}

			result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
			assert.NoError(t, err)
			assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
			assert.Equal(t, buildapi.BuildReasonTrigger, result.ReasonsStr)
			assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)

			var changes []buildchange.GenericChange
			err = json.Unmarshal([]byte(result.ChangesStr), &changes)
			assert.NoError(t, err)
			assert.Len(t, changes, 1)
			assert.Equal(t, buildapi.BuildReasonTrigger, changes[0].Reason)
			assert.Equal(t, "", changes[0].Old)
			assert.True(t, strings.HasPrefix((changes[0].New).(string), "A new build was manually triggered on "))
		})

		when("Builder Metadata changes", func() {
			it("false if builder has additional unused buildpacks", func() {
				builder.BuilderMetadata = []corev1alpha1.BuildpackMetadata{
					{Id: "buildpack.matches", Version: "1"},
					{Id: "buildpack.unused", Version: "unused"},
				}

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionFalse, result.ConditionStatus)
				assert.Equal(t, "", result.PriorityClass)
				assert.Equal(t, "", result.ReasonsStr)
				assert.Equal(t, "", result.ChangesStr)
			})

			it("true if builder metadata has different buildpack version from used buildpack version", func() {
				builder.BuilderMetadata = []corev1alpha1.BuildpackMetadata{
					{Id: "buildpack.matches", Version: "NEW_VERSION"},
					{Id: "buildpack.different", Version: "different"},
				}

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "BUILDPACK",
    "old": [
      {
        "id": "buildpack.matches",
        "version": "1"
      }
    ],
    "new": null
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonBuildpack, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassLow, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("true if builder does not have all most recent used buildpacks", func() {
				builder.BuilderMetadata = []corev1alpha1.BuildpackMetadata{
					{Id: "buildpack.only.new.buildpacks", Version: "1"},
					{Id: "buildpack.only.new.or.unused.buildpacks", Version: "1"},
				}

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "BUILDPACK",
    "old": [
      {
        "id": "buildpack.matches",
        "version": "1"
      }
    ],
    "new": null
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonBuildpack, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassLow, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("true if builder has a different run image", func() {
				builder.LatestRunImage = "some.registry.io/run-image@sha256:a1aa3da2a80a775df55e880b094a1a8de19b919435ad0c71c29a0983d64e65db"

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "STACK",
    "old": "sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
    "new": "sha256:a1aa3da2a80a775df55e880b094a1a8de19b919435ad0c71c29a0983d64e65db"
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonStack, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassLow, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})
		})

		when("Git", func() {
			it("true for different GitURL", func() {
				sourceResolver.Status.Source.Git.URL = "different"

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "different",
          "revision": "revision"
        }
      }
    }
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("true for different Git SubPath", func() {
				sourceResolver.Status.Source.Git.SubPath = "different"

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        },
        "subPath": "different"
      }
    }
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("true for different GitRevision", func() {
				sourceResolver.Status.Source.Git.Revision = "different"

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "COMMIT",
    "old": "revision",
    "new": "different"
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonCommit, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("false if source resolver is not ready", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: corev1.ConditionFalse,
					}}

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionUnknown, result.ConditionStatus)
				assert.Equal(t, "", result.PriorityClass)
				assert.Equal(t, "", result.ReasonsStr)
				assert.Equal(t, "", result.ChangesStr)
			})

			it("false if builder is not ready", func() {
				sourceResolver.Status.Source.Git.URL = "some-change"
				builder.BuilderReady = false

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionUnknown, result.ConditionStatus)
				assert.Equal(t, "", result.PriorityClass)
				assert.Equal(t, "", result.ReasonsStr)
				assert.Equal(t, "", result.ChangesStr)
			})

			it("false if source resolver has not resolved", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{}

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionUnknown, result.ConditionStatus)
				assert.Equal(t, "", result.PriorityClass)
				assert.Equal(t, "", result.ReasonsStr)
				assert.Equal(t, "", result.ChangesStr)
			})

			it("false if source resolver has not resolved and there is no previous build", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.Status.Conditions = []corev1alpha1.Condition{}

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionUnknown, result.ConditionStatus)
				assert.Equal(t, "", result.PriorityClass)
				assert.Equal(t, "", result.ReasonsStr)
				assert.Equal(t, "", result.ChangesStr)
			})

			it("false if source resolver has not processed current generation", func() {
				sourceResolver.Status.Source.Git.Revision = "different"
				sourceResolver.ObjectMeta.Generation = 2
				sourceResolver.Status.ObservedGeneration = 1

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionUnknown, result.ConditionStatus)
				assert.Equal(t, "", result.PriorityClass)
				assert.Equal(t, "", result.ReasonsStr)
				assert.Equal(t, "", result.ChangesStr)
			})

			it("true if build env order changes and git url changes", func() {
				latestBuild.Spec.Source.Git.URL = "old-git.com/url"
				latestBuild.Spec.Env = []corev1.EnvVar{
					{Name: "keyA", Value: "old"},
				}
				image.Spec.Build = &buildapi.ImageBuild{
					Env: []corev1.EnvVar{
						{Name: "keyA", Value: "new"},
					},
				}

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "env": [
        {
          "name": "keyA",
          "value": "old"
        }
      ],
      "resources": {},
      "source": {
        "git": {
          "url": "old-git.com/url",
          "revision": "revision"
        }
      }
    },
    "new": {
      "env": [
        {
          "name": "keyA",
          "value": "new"
        }
      ],
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url",
          "revision": "revision"
        }
      }
    }
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("true if both config and commit have changed", func() {
				sourceResolver.Status.Source.Git.Revision = "different"

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "COMMIT",
    "old": "revision",
    "new": "different"
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonCommit, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})
		})

		when("Blob", func() {
			sourceResolver.Status.Source = corev1alpha1.ResolvedSourceConfig{
				Blob: &corev1alpha1.ResolvedBlobSource{
					URL: "different",
				},
			}

			latestBuild.Spec.Source = corev1alpha1.SourceConfig{
				Blob: &corev1alpha1.Blob{
					URL: "some-url",
				},
			}

			it("true for different BlobURL", func() {
				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "blob": {
          "url": "some-url"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "blob": {
          "url": "different"
        }
      }
    }
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("true for different Blob SubPath", func() {
				sourceResolver.Status.Source.Blob.SubPath = "different"

				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "blob": {
          "url": "some-url"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "blob": {
          "url": "different"
        },
        "subPath": "different"
      }
    }
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})
		})

		when("Registry", func() {
			sourceResolver.Status.Source = corev1alpha1.ResolvedSourceConfig{
				Registry: &corev1alpha1.ResolvedRegistrySource{
					Image: "different",
				},
			}

			latestBuild.Spec.Source = corev1alpha1.SourceConfig{
				Registry: &corev1alpha1.Registry{
					Image: "some-image",
				},
			}

			it("true for different RegistryImage", func() {
				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "registry": {
          "image": "some-image"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "registry": {
          "image": "different"
        }
      }
    }
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})

			it("true for different Registry SubPath", func() {
				sourceResolver.Status.Source.Registry.SubPath = "different"
				expectedChanges := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "registry": {
          "image": "some-image"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "registry": {
          "image": "different"
        },
        "subPath": "different"
      }
    }
  }
]`)

				result, err := isBuildRequired(image, latestBuild, sourceResolver, builder)
				assert.NoError(t, err)
				assert.Equal(t, corev1.ConditionTrue, result.ConditionStatus)
				assert.Equal(t, buildapi.BuildReasonConfig, result.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityClassHigh, result.PriorityClass)
				assert.Equal(t, expectedChanges, result.ChangesStr)
			})
		})
	})
}

type TestBuilderResource struct {
	BuilderReady     bool
	BuilderMetadata  []corev1alpha1.BuildpackMetadata
	ImagePullSecrets []corev1.LocalObjectReference
	LatestImage      string
	LatestRunImage   string
	Name             string
	Kind             string
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
