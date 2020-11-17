package buildchange_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestChangeProcessor(t *testing.T) {
	spec.Run(t, "ChangeProcessor", testChangeProcessor)
}

func testChangeProcessor(t *testing.T, when spec.G, it spec.S) {
	cp := buildchange.NewChangeProcessor()

	when("no changes processed", func() {
		it("returns HasChanges as false", func() {
			assert.Equal(t, false, cp.HasChanges())
		})

		it("returns ReasonsStr as empty string", func() {
			assert.Equal(t, "", cp.ReasonsStr())
		})

		it("returns ChangesStr as empty string and does not error", func() {
			changesStr, err := cp.ChangesStr()
			assert.NoError(t, err)
			assert.Equal(t, "", changesStr)
		})
	})

	when("single change processed", func() {
		when("TriggerChange", func() {
			when("invalid", func() {
				change := buildchange.TriggerChange{New: "invalid-time-string"}

				it("returns HasChanges as false", func() {
					assert.False(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as empty string", func() {
					assert.Equal(t, "", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as empty string", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					assert.NoError(t, err)
					assert.Equal(t, "", changesStr)
				})
			})

			when("valid", func() {
				change := buildchange.TriggerChange{New: "2020-11-20 15:38:15.794105 -0500 EST m=+0.022963826"}

				it("returns HasChanges as true", func() {
					assert.True(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as TRIGGER", func() {
					assert.Equal(t, "TRIGGER", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as a JSON string mapping TRIGGER to TriggerChange", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					expectedChangesStr := fmt.Sprintf(`{"TRIGGER":{"new":"%s"}}`, change.New)
					assert.NoError(t, err)
					assert.Equal(t, expectedChangesStr, changesStr)

					changesMap := map[string]buildchange.TriggerChange{}
					err = json.Unmarshal([]byte(changesStr), &changesMap)
					assert.NoError(t, err)

					unmarshalledChange, ok := changesMap["TRIGGER"]
					assert.True(t, ok)
					assert.True(t, cmp.Equal(change, unmarshalledChange))
				})
			})
		})

		when("CommitChange", func() {
			when("invalid", func() {
				change := buildchange.CommitChange{Old: "same-revision", New: "same-revision"}

				it("returns HasChanges as false", func() {
					assert.False(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as empty string", func() {
					assert.Equal(t, "", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as empty string", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					assert.NoError(t, err)
					assert.Equal(t, "", changesStr)
				})
			})

			when("valid", func() {
				change := buildchange.CommitChange{Old: "old-revision", New: "new-revision"}

				it("returns reasons as COMMIT", func() {
					assert.Equal(t, "COMMIT", cp.Process(change).ReasonsStr())
				})

				it("returns changes as a JSON string mapping COMMIT to CommitChange", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					expectedChangesStr := `{"COMMIT":{"old":"old-revision","new":"new-revision"}}`
					assert.NoError(t, err)
					assert.Equal(t, expectedChangesStr, changesStr)

					changesMap := map[string]buildchange.CommitChange{}
					err = json.Unmarshal([]byte(changesStr), &changesMap)
					assert.NoError(t, err)

					unmarshalledChange, ok := changesMap["COMMIT"]
					assert.True(t, ok)
					assert.True(t, cmp.Equal(change, unmarshalledChange))
				})

				it("returns true for HasChanges", func() {
					cp.Process(change)
					assert.True(t, cp.HasChanges())
				})
			})
		})

		when("ConfigChange", func() {
			when("invalid", func() {
				change := buildchange.ConfigChange{
					Old: buildchange.Config{
						Source: v1alpha1.SourceConfig{
							Git: &v1alpha1.Git{
								URL:      "some-git-url",
								Revision: "some-git-revision",
							},
						},
					},
					New: buildchange.Config{
						Source: v1alpha1.SourceConfig{
							Git: &v1alpha1.Git{
								URL:      "some-git-url",
								Revision: "some-git-revision",
							},
						},
					},
				}

				it("returns HasChanges as false", func() {
					assert.False(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as empty string", func() {
					assert.Equal(t, "", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as empty string", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					assert.NoError(t, err)
					assert.Equal(t, "", changesStr)
				})
			})

			when("valid", func() {
				change := buildchange.ConfigChange{
					Old: buildchange.Config{
						Env: []corev1.EnvVar{
							{Name: "env-var-name", Value: "env-var-value"},
							{Name: "another-env-var-name", Value: "another-env-var-value"},
						},
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resourceQuantity(t, "500m"),
								corev1.ResourceMemory: resourceQuantity(t, "2G"),
							},
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resourceQuantity(t, "100m"),
								corev1.ResourceMemory: resourceQuantity(t, "512M"),
							},
						},
						Source: v1alpha1.SourceConfig{
							Git: &v1alpha1.Git{
								URL:      "some-git-url",
								Revision: "some-git-revision",
							},
							SubPath: "some-sub-path",
						},
					},
					New: buildchange.Config{
						Env: []corev1.EnvVar{
							{Name: "new-env-var-name", Value: "new-env-var-value"},
							{Name: "another-env-var-name", Value: "another-env-var-value"},
						},
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resourceQuantity(t, "300m"),
								corev1.ResourceMemory: resourceQuantity(t, "1G"),
							},
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    resourceQuantity(t, "200m"),
								corev1.ResourceMemory: resourceQuantity(t, "512M"),
							},
						},
						Source: v1alpha1.SourceConfig{
							Blob: &v1alpha1.Blob{URL: "some-blob-url"},
						},
						Bindings: []v1alpha1.Binding{
							{
								Name: "binding-name",
								MetadataRef: &corev1.LocalObjectReference{
									Name: "some-metadata-ref",
								},
								SecretRef: &corev1.LocalObjectReference{
									Name: "some-secret-ref",
								},
							},
						},
					},
				}

				expectedChangesStr := testhelpers.CompactJSON(`
{
  "CONFIG": {
    "old": {
      "env": [
        {
          "name": "env-var-name",
          "value": "env-var-value"
        },
        {
          "name": "another-env-var-name",
          "value": "another-env-var-value"
        }
      ],
      "resources": {
        "limits": {
          "cpu": "500m",
          "memory": "2G"
        },
        "requests": {
          "cpu": "100m",
          "memory": "512M"
        }
      },
      "source": {
        "git": {
          "url": "some-git-url",
          "revision": "some-git-revision"
        },
        "subPath": "some-sub-path"
      }
    },
    "new": {
      "env": [
        {
          "name": "new-env-var-name",
          "value": "new-env-var-value"
        },
        {
          "name": "another-env-var-name",
          "value": "another-env-var-value"
        }
      ],
      "resources": {
        "limits": {
          "cpu": "300m",
          "memory": "1G"
        },
        "requests": {
          "cpu": "200m",
          "memory": "512M"
        }
      },
      "bindings": [
        {
          "name": "binding-name",
          "metadataRef": {
            "name": "some-metadata-ref"
          },
          "secretRef": {
            "name": "some-secret-ref"
          }
        }
      ],
      "source": {
        "blob": {
          "url": "some-blob-url"
        }
      }
    }
  }
}`)

				it("returns HasChanges as true", func() {
					assert.True(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as CONFIG", func() {
					assert.Equal(t, "CONFIG", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as a JSON string mapping CONFIG to ConfigChange", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					assert.NoError(t, err)
					assert.Equal(t, expectedChangesStr, changesStr)

					changesMap := map[string]buildchange.ConfigChange{}
					err = json.Unmarshal([]byte(changesStr), &changesMap)
					assert.NoError(t, err)

					unmarshalledChange, ok := changesMap["CONFIG"]
					assert.True(t, ok)
					assert.True(t, cmp.Equal(change, unmarshalledChange))
				})
			})
		})

		when("BuildpackChange", func() {
			when("invalid", func() {
				change := buildchange.BuildpackChange{
					Old: []buildchange.BuildpackInfo{
						{Id: "some-buildpack-id", Version: "some-buildpack-version"},
					},
					New: []buildchange.BuildpackInfo{
						{Id: "some-buildpack-id", Version: "some-buildpack-version"},
					},
				}

				it("returns HasChanges as false", func() {
					assert.False(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as empty string", func() {
					assert.Equal(t, "", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as empty string", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					assert.NoError(t, err)
					assert.Equal(t, "", changesStr)
				})
			})

			when("valid", func() {
				change := buildchange.BuildpackChange{
					Old: []buildchange.BuildpackInfo{
						{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
						{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
					},
					New: []buildchange.BuildpackInfo{
						{Id: "some-buildpack-id", Version: "some-buildpack-new-version"},
					},
				}

				it("returns HasChanges as true", func() {
					assert.True(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as BUILDPACK", func() {
					assert.Equal(t, "BUILDPACK", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as a JSON string mapping BUILDPACK to BuildpackChange", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					expectedChangesStr := testhelpers.CompactJSON(`
{
  "BUILDPACK": {
    "old": [
      {
        "id": "another-buildpack-id",
        "version": "another-buildpack-old-version"
      },
      {
        "id": "some-buildpack-id",
        "version": "some-buildpack-old-version"
      }
    ],
    "new": [
      {
        "id": "some-buildpack-id",
        "version": "some-buildpack-new-version"
      }
    ]
  }
}`)

					assert.NoError(t, err)
					assert.Equal(t, expectedChangesStr, changesStr)

					changesMap := map[string]buildchange.BuildpackChange{}
					err = json.Unmarshal([]byte(changesStr), &changesMap)
					assert.NoError(t, err)

					unmarshalledChange, ok := changesMap["BUILDPACK"]
					assert.True(t, ok)
					assert.True(t, cmp.Equal(change, unmarshalledChange))
				})
			})
		})

		when("StackChange", func() {
			when("invalid", func() {
				change := buildchange.StackChange{}

				it("returns HasChanges as false", func() {
					assert.False(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as empty string", func() {
					assert.Equal(t, "", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as empty string", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					assert.NoError(t, err)
					assert.Equal(t, "", changesStr)
				})
			})

			when("valid", func() {
				change := buildchange.StackChange{Old: "old-run-image", New: "new-run-image"}

				it("returns HasChanges as true", func() {
					assert.True(t, cp.Process(change).HasChanges())
				})

				it("returns ReasonsStr as STACK", func() {
					assert.Equal(t, "STACK", cp.Process(change).ReasonsStr())
				})

				it("returns ChangesStr as a JSON string mapping STACK to StackChange", func() {
					changesStr, err := cp.Process(change).ChangesStr()
					expectedChangesStr := `{"STACK":{"old":"old-run-image","new":"new-run-image"}}`

					assert.NoError(t, err)
					assert.Equal(t, expectedChangesStr, changesStr)

					changesMap := map[string]buildchange.StackChange{}
					err = json.Unmarshal([]byte(changesStr), &changesMap)
					assert.NoError(t, err)

					unmarshalledChange, ok := changesMap["STACK"]
					assert.True(t, ok)
					assert.True(t, cmp.Equal(change, unmarshalledChange))
				})
			})
		})
	})

	when("multiple valid changes processed", func() {
		commitChange := buildchange.CommitChange{Old: "old-revision", New: "new-revision"}

		triggerChange := buildchange.TriggerChange{New: "2020-11-20 15:38:15.794105 -0500 EST m=+0.022963826"}

		stackChange := buildchange.StackChange{Old: "old-run-image", New: "new-run-image"}

		buildpackChange := buildchange.BuildpackChange{
			Old: []buildchange.BuildpackInfo{
				{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
				{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
			},
			New: []buildchange.BuildpackInfo{
				{Id: "some-buildpack-id", Version: "some-buildpack-new-version"},
			},
		}

		configChange := buildchange.ConfigChange{
			Old: buildchange.Config{
				Env: []corev1.EnvVar{
					{Name: "env-var-name", Value: "env-var-value"},
					{Name: "another-env-var-name", Value: "another-env-var-value"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resourceQuantity(t, "500m"),
						corev1.ResourceMemory: resourceQuantity(t, "2G"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resourceQuantity(t, "100m"),
						corev1.ResourceMemory: resourceQuantity(t, "512M"),
					},
				},
				Source: v1alpha1.SourceConfig{
					Git: &v1alpha1.Git{
						URL:      "some-git-url",
						Revision: "some-git-revision",
					},
					SubPath: "some-sub-path",
				},
			},
			New: buildchange.Config{
				Env: []corev1.EnvVar{
					{Name: "new-env-var-name", Value: "new-env-var-value"},
					{Name: "another-env-var-name", Value: "another-env-var-value"},
				},
				Resources: corev1.ResourceRequirements{
					Limits: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resourceQuantity(t, "300m"),
						corev1.ResourceMemory: resourceQuantity(t, "1G"),
					},
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceCPU:    resourceQuantity(t, "200m"),
						corev1.ResourceMemory: resourceQuantity(t, "512M"),
					},
				},
				Source: v1alpha1.SourceConfig{
					Blob: &v1alpha1.Blob{URL: "some-blob-url"},
				},
				Bindings: []v1alpha1.Binding{
					{
						Name: "binding-name",
						MetadataRef: &corev1.LocalObjectReference{
							Name: "some-metadata-ref",
						},
						SecretRef: &corev1.LocalObjectReference{
							Name: "some-secret-ref",
						},
					},
				},
			},
		}

		expectedChangesStr := testhelpers.CompactJSON(`{
  "BUILDPACK": {
    "old": [
      {
        "id": "another-buildpack-id",
        "version": "another-buildpack-old-version"
      },
      {
        "id": "some-buildpack-id",
        "version": "some-buildpack-old-version"
      }
    ],
    "new": [
      {
        "id": "some-buildpack-id",
        "version": "some-buildpack-new-version"
      }
    ]
  },
  "COMMIT": {
    "old": "old-revision",
    "new": "new-revision"
  },
  "CONFIG": {
    "old": {
      "env": [
        {
          "name": "env-var-name",
          "value": "env-var-value"
        },
        {
          "name": "another-env-var-name",
          "value": "another-env-var-value"
        }
      ],
      "resources": {
        "limits": {
          "cpu": "500m",
          "memory": "2G"
        },
        "requests": {
          "cpu": "100m",
          "memory": "512M"
        }
      },
      "source": {
        "git": {
          "url": "some-git-url",
          "revision": "some-git-revision"
        },
        "subPath": "some-sub-path"
      }
    },
    "new": {
      "env": [
        {
          "name": "new-env-var-name",
          "value": "new-env-var-value"
        },
        {
          "name": "another-env-var-name",
          "value": "another-env-var-value"
        }
      ],
      "resources": {
        "limits": {
          "cpu": "300m",
          "memory": "1G"
        },
        "requests": {
          "cpu": "200m",
          "memory": "512M"
        }
      },
      "bindings": [
        {
          "name": "binding-name",
          "metadataRef": {
            "name": "some-metadata-ref"
          },
          "secretRef": {
            "name": "some-secret-ref"
          }
        }
      ],
      "source": {
        "blob": {
          "url": "some-blob-url"
        }
      }
    }
  },
  "STACK": {
    "old": "old-run-image",
    "new": "new-run-image"
  },
  "TRIGGER": {
    "new": "2020-11-20 15:38:15.794105 -0500 EST m=+0.022963826"
  }
}`)

		it.Before(func() {
			cp.Process(commitChange)
			cp.Process(triggerChange)
			cp.Process(stackChange)
			cp.Process(buildpackChange)
			cp.Process(configChange)
		})

		it("returns HasChanges as true", func() {
			assert.True(t, cp.HasChanges())
		})

		it("returns ReasonsStr as a comma separated sorted list of reasons", func() {
			assert.Equal(t, v1alpha1.BuildReasonSortIndex, cp.ReasonsStr())
		})

		it("returns ChangesStr as a JSON string mapping reason to change", func() {
			changesStr, err := cp.ChangesStr()
			assert.NoError(t, err)
			assert.Equal(t, expectedChangesStr, changesStr)

			var changesMap map[string]interface{}
			err = json.Unmarshal([]byte(changesStr), &changesMap)
			assert.NoError(t, err)

			changeDecoder := buildchange.ChangeDecoder{}

			change, ok := changesMap["TRIGGER"]
			assert.True(t, ok)
			decodedChange, err := changeDecoder.Decode("TRIGGER", change)
			assert.NoError(t, err)
			assert.True(t, cmp.Equal(triggerChange, decodedChange))

			change, ok = changesMap["COMMIT"]
			assert.True(t, ok)
			decodedChange, err = changeDecoder.Decode("COMMIT", change)
			assert.NoError(t, err)
			assert.True(t, cmp.Equal(commitChange, decodedChange))

			change, ok = changesMap["CONFIG"]
			assert.True(t, ok)
			decodedChange, err = changeDecoder.Decode("CONFIG", change)
			assert.NoError(t, err)
			assert.True(t, cmp.Equal(configChange, decodedChange))

			change, ok = changesMap["BUILDPACK"]
			assert.True(t, ok)
			decodedChange, err = changeDecoder.Decode("BUILDPACK", change)
			assert.NoError(t, err)
			assert.True(t, cmp.Equal(buildpackChange, decodedChange))

			change, ok = changesMap["STACK"]
			assert.True(t, ok)
			decodedChange, err = changeDecoder.Decode("STACK", change)
			assert.NoError(t, err)
			assert.True(t, cmp.Equal(stackChange, decodedChange))
		})
	})
}
