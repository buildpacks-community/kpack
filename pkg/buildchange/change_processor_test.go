package buildchange_test

import (
	"encoding/json"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestChangeProcessor(t *testing.T) {
	spec.Run(t, "ChangeProcessor", testChangeProcessor)
}

func testChangeProcessor(t *testing.T, when spec.G, it spec.S) {
	cp := buildchange.NewChangeProcessor()

	when("no changes processed", func() {
		it("returns the correct ChangeSummary and does not error", func() {
			summary, err := cp.Summarize()
			assert.NoError(t, err)
			assert.False(t, summary.HasChanges)
			assert.Empty(t, summary.ReasonsStr)
			assert.Empty(t, summary.ChangesStr)
		})
	})

	when("nil change is processed", func() {
		it("returns the correct ChangeSummary and does not error", func() {
			summary, err := cp.Process(nil).Summarize()
			assert.NoError(t, err)
			assert.False(t, summary.HasChanges)
			assert.Empty(t, summary.ReasonsStr)
			assert.Empty(t, summary.ChangesStr)
		})
	})

	when("single change processed", func() {
		when("TRIGGER", func() {
			change := buildchange.NewTriggerChange("Fri, 20 Nov 2020 15:38:15 -0500")
			expectedReasonsStr := "TRIGGER"
			expectedChangesStr := testhelpers.CompactJSON(`
[
  {
    "reason": "TRIGGER",
    "old": "",
    "new": "A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500"
  }
]`)

			it("returns the correct ChangeSummary and does not error", func() {
				summary, err := cp.Process(change).Summarize()
				assert.NoError(t, err)
				assert.True(t, summary.HasChanges)
				assert.Equal(t, expectedReasonsStr, summary.ReasonsStr)
				assert.Equal(t, expectedChangesStr, summary.ChangesStr)
				assert.Equal(t, buildapi.BuildPriorityHigh, summary.Priority)
				var changes []buildchange.GenericChange
				err = json.Unmarshal([]byte(summary.ChangesStr), &changes)
				assert.NoError(t, err)
			})
		})

		when("COMMIT", func() {
			when("has no difference", func() {
				change := buildchange.NewCommitChange("same-revision", "same-revision")

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.False(t, summary.HasChanges)
					assert.Empty(t, summary.ReasonsStr)
					assert.Empty(t, summary.ChangesStr)
					assert.Equal(t, buildapi.BuildPriorityNone, summary.Priority)
				})
			})

			when("has difference", func() {
				change := buildchange.NewCommitChange("old-revision", "new-revision")
				expectedReasonsStr := "COMMIT"
				expectedChangesStr := testhelpers.CompactJSON(`
[
  {
    "reason": "COMMIT",
    "old": "old-revision",
    "new": "new-revision"
  }
]`)

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.True(t, summary.HasChanges)
					assert.Equal(t, expectedReasonsStr, summary.ReasonsStr)
					assert.Equal(t, expectedChangesStr, summary.ChangesStr)
					assert.Equal(t, buildapi.BuildPriorityHigh, summary.Priority)

					var changes []buildchange.GenericChange
					err = json.Unmarshal([]byte(summary.ChangesStr), &changes)
					assert.NoError(t, err)
				})
			})
		})

		when("CONFIG", func() {
			when("has no difference", func() {
				oldConfig := buildchange.Config{
					Source: corev1alpha1.SourceConfig{
						Git: &corev1alpha1.Git{
							URL:      "some-git-url",
							Revision: "some-git-revision",
						},
					},
				}
				newConfig := oldConfig
				change := buildchange.NewConfigChange(oldConfig, newConfig)

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.Equal(t, buildapi.BuildPriorityNone, summary.Priority)
					assert.False(t, summary.HasChanges)
					assert.Empty(t, summary.ReasonsStr)
					assert.Empty(t, summary.ChangesStr)
				})
			})

			when("has difference", func() {
				change := buildchange.NewConfigChange(
					buildchange.Config{
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
						Source: corev1alpha1.SourceConfig{
							Git: &corev1alpha1.Git{
								URL:      "some-git-url",
								Revision: "some-git-revision",
							},
							SubPath: "some-sub-path",
						},
					},
					buildchange.Config{
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
						Source: corev1alpha1.SourceConfig{
							Blob: &corev1alpha1.Blob{URL: "some-blob-url"},
						},
						Services: buildapi.Services{
							{
								Name: "some-secret-ref",
							},
						},
					})
				expectedReasonsStr := "CONFIG"
				expectedChangesStr := testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
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
      "services": [
        {
          "name": "some-secret-ref"
        }
      ],
      "source": {
        "blob": {
          "url": "some-blob-url"
        }
      }
    }
  }
]`)

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.True(t, summary.HasChanges)
					assert.Equal(t, expectedReasonsStr, summary.ReasonsStr)
					assert.Equal(t, expectedChangesStr, summary.ChangesStr)
					assert.Equal(t, buildapi.BuildPriorityHigh, summary.Priority)

					var changes []buildchange.GenericChange
					err = json.Unmarshal([]byte(summary.ChangesStr), &changes)
					assert.NoError(t, err)
				})
			})
		})

		when("BUILDPACK", func() {
			when("has no difference", func() {
				oldBuildpacks := []corev1alpha1.BuildpackInfo{
					{Id: "some-buildpack-id", Version: "some-buildpack-version"},
				}
				newBuildpacks := oldBuildpacks
				change := buildchange.NewBuildpackChange(oldBuildpacks, newBuildpacks)

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.False(t, summary.HasChanges)
					assert.Equal(t, buildapi.BuildPriorityNone, summary.Priority)
					assert.Empty(t, summary.ReasonsStr)
					assert.Empty(t, summary.ChangesStr)
				})
			})

			when("has difference", func() {
				change := buildchange.NewBuildpackChange(
					[]corev1alpha1.BuildpackInfo{
						{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
						{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
					},
					[]corev1alpha1.BuildpackInfo{
						{Id: "some-buildpack-id", Version: "some-buildpack-new-version"},
					})

				expectedReasonsStr := "BUILDPACK"
				expectedChangesStr := testhelpers.CompactJSON(`
[
  {
    "reason": "BUILDPACK",
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
]`)

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.True(t, summary.HasChanges)
					assert.Equal(t, expectedReasonsStr, summary.ReasonsStr)
					assert.Equal(t, expectedChangesStr, summary.ChangesStr)
					assert.Equal(t, buildapi.BuildPriorityLow, summary.Priority)

					var changes []buildchange.GenericChange
					err = json.Unmarshal([]byte(summary.ChangesStr), &changes)
					assert.NoError(t, err)
				})
			})
		})

		when("STACK", func() {
			when("invalid values are given", func() {
				change := buildchange.NewStackChange("invalid-oldRunImageRef", "invalid-newRunImageRef")
				expectedErrorStr := `error determining if build is required for reason 'STACK': could not parse reference: invalid-oldRunImageRef; could not parse reference: invalid-newRunImageRef`

				it("errors for Summarize", func() {
					_, err := cp.Process(change).Summarize()
					assert.Error(t, err)
					assert.Equal(t, expectedErrorStr, err.Error())
				})
			})

			when("has no difference", func() {
				oldRunImage := "gcr.io/some-project/repo/run@sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ad"
				newRunImage := oldRunImage
				change := buildchange.NewStackChange(oldRunImage, newRunImage)

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.False(t, summary.HasChanges)
					assert.Empty(t, summary.ReasonsStr)
					assert.Empty(t, summary.ChangesStr)
					assert.Equal(t, buildapi.BuildPriorityNone, summary.Priority)
				})
			})

			when("has difference", func() {
				oldRunImageRef := "gcr.io/some-project/repo/run@sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ad"
				newRunImageRef := "gcr.io/some-project/repo/run@sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ae"
				change := buildchange.NewStackChange(oldRunImageRef, newRunImageRef)
				expectedReasonsStr := "STACK"
				expectedChangesStr := testhelpers.CompactJSON(`
[
  {
    "reason": "STACK",
    "old": "sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ad",
    "new": "sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ae"
  }
]`)

				it("returns the correct ChangeSummary and does not error", func() {
					summary, err := cp.Process(change).Summarize()
					assert.NoError(t, err)
					assert.True(t, summary.HasChanges)
					assert.Equal(t, expectedReasonsStr, summary.ReasonsStr)
					assert.Equal(t, expectedChangesStr, summary.ChangesStr)
					assert.Equal(t, buildapi.BuildPriorityLow, summary.Priority)

					var changes []buildchange.GenericChange
					err = json.Unmarshal([]byte(summary.ChangesStr), &changes)
					assert.NoError(t, err)
				})
			})
		})
	})

	when("multiple changes with difference are processed", func() {
		when("they are all valid", func() {
			commitChange := buildchange.NewCommitChange("old-revision", "new-revision")

			triggerChange := buildchange.NewTriggerChange("Fri, 20 Nov 2020 15:38:15 -0500")

			oldRunImageRef := "gcr.io/some-project/repo/run@sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ad"
			newRunImageRef := "gcr.io/some-project/repo/run@sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ae"
			stackChange := buildchange.NewStackChange(oldRunImageRef, newRunImageRef)

			buildpackChange := buildchange.NewBuildpackChange(
				[]corev1alpha1.BuildpackInfo{
					{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
					{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
				},
				[]corev1alpha1.BuildpackInfo{
					{Id: "some-buildpack-id", Version: "some-buildpack-new-version"},
				})

			configChange := buildchange.NewConfigChange(
				buildchange.Config{
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
					Source: corev1alpha1.SourceConfig{
						Git: &corev1alpha1.Git{
							URL:      "some-git-url",
							Revision: "some-git-revision",
						},
						SubPath: "some-sub-path",
					},
				},
				buildchange.Config{
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
					Source: corev1alpha1.SourceConfig{
						Blob: &corev1alpha1.Blob{URL: "some-blob-url"},
					},
					Services: buildapi.Services{
						{
							Name: "some-secret-ref",
						},
					},
				})

			expectedChangesStr := testhelpers.CompactJSON(`
[
  {
    "reason": "TRIGGER",
    "old": "",
    "new": "A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500"
  },
  {
    "reason": "COMMIT",
    "old": "old-revision",
    "new": "new-revision"
  },
  {
    "reason": "CONFIG",
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
      "services": [
        {
          "name": "some-secret-ref"
        }
      ],
      "source": {
        "blob": {
          "url": "some-blob-url"
        }
      }
    }
  },
  {
    "reason": "BUILDPACK",
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
  {
    "reason": "STACK",
    "old": "sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ad",
    "new": "sha256:87302783be0a0cab9fde5b68c9954b7e9150ca0d514ba542e9810c3c6f2984ae"
  }
]`)

			it("returns the correct ChangeSummary and does not error", func() {
				summary, err := cp.Process(triggerChange).Process(commitChange).Process(configChange).Process(buildpackChange).Process(stackChange).Summarize()
				assert.NoError(t, err)
				assert.True(t, summary.HasChanges)
				assert.Equal(t, "TRIGGER,COMMIT,CONFIG,BUILDPACK,STACK", summary.ReasonsStr)
				assert.Equal(t, buildapi.BuildPriorityHigh, summary.Priority)
				assert.Equal(t, expectedChangesStr, summary.ChangesStr)

				var changes []buildchange.GenericChange
				err = json.Unmarshal([]byte(summary.ChangesStr), &changes)
				assert.NoError(t, err)
				assert.Len(t, changes, 5)
				assert.Equal(t, "TRIGGER", changes[0].Reason)
				assert.Equal(t, "COMMIT", changes[1].Reason)
				assert.Equal(t, "CONFIG", changes[2].Reason)
				assert.Equal(t, "BUILDPACK", changes[3].Reason)
				assert.Equal(t, "STACK", changes[4].Reason)
			})
		})

		when("some are invalid", func() {
			triggerChange := buildchange.NewTriggerChange("Fri, 20 Nov 2020 15:38:15 -0500")
			stackChange := buildchange.NewStackChange("invalid-oldRunImageRef", "invalid-newRunImageRef")
			expectedErrorStr := `error determining if build is required for reason 'STACK': could not parse reference: invalid-oldRunImageRef; could not parse reference: invalid-newRunImageRef`

			it("errors for Summarize", func() {
				_, err := cp.Process(triggerChange).Process(stackChange).Summarize()
				assert.Error(t, err)
				assert.Equal(t, expectedErrorStr, err.Error())
			})
		})
	})
}
