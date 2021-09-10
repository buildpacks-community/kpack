package buildchange_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

func TestLogBuildReasonAndChanges(t *testing.T) {
	spec.Run(t, "TestLogBuildReasonAndChanges", testLog)
}

func testLog(t *testing.T, when spec.G, it spec.S) {
	diffOptions := testhelpers.DiffOptions{Prefix: "\t", Color: true}
	diffOutBuilder := testhelpers.NewDiffOutBuilder(t).Configure(diffOptions)

	when("changes string is empty", func() {
		it("does not error", func() {
			LogTest{
				changesStr:  "",
				expectedOut: "",
			}.execute(t)
		})
	})

	when("change is not a JSON string", func() {
		it("errors", func() {
			LogTest{
				changesStr:  "random-string",
				expectedErr: "error parsing build changes JSON string 'random-string': invalid character 'r' looking for beginning of value",
			}.execute(t)
		})
	})

	when("change is not a valid JSON string", func() {
		it("errors", func() {
			LogTest{
				changesStr:  `{"key":"value"}`,
				expectedErr: `error parsing build changes JSON string '{"key":"value"}': json: cannot unmarshal object into Go value of type []buildchange.GenericChange`,
			}.execute(t)
		})
	})

	when("single change", func() {
		when("TRIGGER", func() {
			it("prints build reason as TRIGGER and the message", func() {
				LogTest{
					changesStr: changesToStr(t, buildchange.GenericChange{
						Reason: "TRIGGER",
						New:    "A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500",
					}),
					expectedOut: diffOutBuilder.Reset().
						Txt("Build reason(s): TRIGGER").
						Txt("TRIGGER:").
						New("A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500").Out(),
				}.execute(t)
			})
		})

		when("COMMIT", func() {
			it("prints build reason as COMMIT and the changes", func() {
				LogTest{
					changesStr: changesToStr(t, buildchange.GenericChange{
						Reason: "COMMIT",
						Old:    "old-commit-sha",
						New:    "new-commit-sha",
					}),
					expectedOut: diffOutBuilder.Reset().
						Txt("Build reason(s): COMMIT").
						Txt("COMMIT:").
						Old("old-commit-sha").
						New("new-commit-sha").Out(),
				}.execute(t)
			})
		})

		when("CONFIG", func() {
			it("prints build reason as CONFIG and the changes", func() {
				oldConfig := buildchange.Config{
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
				}

				newConfig := buildchange.Config{
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
				}

				out := diffOutBuilder.Reset().
					Txt("Build reason(s): CONFIG").
					Txt("CONFIG:").
					NoD("env:").
					Old("- name: env-var-name").
					Old("  value: env-var-value").
					New("- name: new-env-var-name").
					New("  value: new-env-var-value").
					NoD("- name: another-env-var-name").
					NoD("  value: another-env-var-value").
					NoD("resources:").
					NoD("  limits:").
					Old("    cpu: 500m").
					Old("    memory: 2G").
					New("    cpu: 300m").
					New("    memory: 1G").
					NoD("  requests:").
					Old("    cpu: 100m").
					New("    cpu: 200m").
					NoD("    memory: 512M").
					New("services:").
					New("- name: some-secret-ref").
					NoD("source:").
					Old("  git:").
					Old("    revision: some-git-revision").
					Old("    url: some-git-url").
					Old("  subPath: some-sub-path").
					New("  blob:").
					New("    url: some-blob-url").Out()

				LogTest{
					changesStr: changesToStr(t, buildchange.GenericChange{
						Reason: "CONFIG",
						Old:    oldConfig,
						New:    newConfig,
					}),
					expectedOut: out,
				}.execute(t)
			})
		})

		when("BUILDPACK", func() {
			it("prints build reason as BUILDPACK and the build changes", func() {
				LogTest{
					changesStr: changesToStr(t, buildchange.GenericChange{
						Reason: "BUILDPACK",
						Old: []corev1alpha1.BuildpackInfo{
							{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
							{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
						},
					}),
					expectedOut: diffOutBuilder.Reset().
						Txt("Build reason(s): BUILDPACK").
						Txt("BUILDPACK:").
						Old("- id: another-buildpack-id").
						Old("  version: another-buildpack-old-version").
						Old("- id: some-buildpack-id").
						Old("  version: some-buildpack-old-version").Out(),
				}.execute(t)
			})
		})

		when("STACK", func() {
			it("prints build reason as STACK and the build changes", func() {
				LogTest{
					changesStr: changesToStr(t, buildchange.GenericChange{
						Reason: "STACK",
						Old:    "old-run-image",
						New:    "new-run-image",
					}),
					expectedOut: diffOutBuilder.Reset().
						Txt("Build reason(s): STACK").
						Txt("STACK:").
						Old("old-run-image").
						New("new-run-image").Out(),
				}.execute(t)
			})
		})
	})

	when("multiple changes", func() {
		it("prints all the valid reasons and changes", func() {
			oldConfig := buildchange.Config{
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
			}

			newConfig := buildchange.Config{
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
			}

			out := diffOutBuilder.Reset().
				Txt("Build reason(s): TRIGGER,COMMIT,CONFIG,BUILDPACK,STACK").
				Txt("TRIGGER:").
				New("A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500").
				Txt("COMMIT:").
				Old("old-commit-sha").
				New("new-commit-sha").
				Txt("CONFIG:").
				NoD("env:").
				Old("- name: env-var-name").
				Old("  value: env-var-value").
				New("- name: new-env-var-name").
				New("  value: new-env-var-value").
				NoD("- name: another-env-var-name").
				NoD("  value: another-env-var-value").
				NoD("resources:").
				NoD("  limits:").
				Old("    cpu: 500m").
				Old("    memory: 2G").
				New("    cpu: 300m").
				New("    memory: 1G").
				NoD("  requests:").
				Old("    cpu: 100m").
				New("    cpu: 200m").
				NoD("    memory: 512M").
				New("services:").
				New("- name: some-secret-ref").
				NoD("source:").
				Old("  git:").
				Old("    revision: some-git-revision").
				Old("    url: some-git-url").
				Old("  subPath: some-sub-path").
				New("  blob:").
				New("    url: some-blob-url").
				Txt("BUILDPACK:").
				Old("- id: another-buildpack-id").
				Old("  version: another-buildpack-old-version").
				NoD("- id: some-buildpack-id").
				Old("  version: some-buildpack-old-version").
				New("  version: some-buildpack-new-version").
				Txt("STACK:").
				Old("old-run-image").
				New("new-run-image").Out()

			LogTest{
				changesStr: changesToStr(t,
					buildchange.GenericChange{
						Reason: "TRIGGER",
						New:    "A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500",
					},
					buildchange.GenericChange{
						Reason: "COMMIT",
						Old:    "old-commit-sha",
						New:    "new-commit-sha",
					},
					buildchange.GenericChange{
						Reason: "CONFIG",
						Old:    oldConfig,
						New:    newConfig,
					},
					buildchange.GenericChange{
						Reason: "BUILDPACK",
						Old: []corev1alpha1.BuildpackInfo{
							{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
							{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
						},
						New: []corev1alpha1.BuildpackInfo{
							{Id: "some-buildpack-id", Version: "some-buildpack-new-version"},
						},
					},
					buildchange.GenericChange{
						Reason: "STACK",
						Old:    "old-run-image",
						New:    "new-run-image",
					},
				),
				expectedOut: out,
			}.execute(t)
		})
	})
}

type LogTest struct {
	changesStr  string
	expectedOut string
	expectedErr string
}

func (l LogTest) execute(t *testing.T) {
	t.Helper()

	fmt.Printf("expected changes: %s\n", l.changesStr)

	out := &bytes.Buffer{}
	logger := log.New(out, "", 0)

	err := buildchange.Log(logger, l.changesStr)

	if l.expectedErr == "" {
		assert.NoError(t, err, "Expected no error\nGot: '%s'\n", err)
	} else {
		assert.NotNil(t, err)
		assert.EqualError(t, err, l.expectedErr, "Error messages do not match\nGot: '%s'\nWant: '%s'\n", err, l.expectedErr)
	}
	assert.Equal(t, l.expectedOut, out.String(), "StdOut messages do not match\nGot: '%s'\nWant: '%s'\n", out.String(), l.expectedOut)
}

func changesToStr(t *testing.T, changes ...buildchange.GenericChange) string {
	t.Helper()

	b, err := json.Marshal(changes)
	assert.NoError(t, err)
	return string(b)
}

func resourceQuantity(t *testing.T, str string) resource.Quantity {
	q, err := resource.ParseQuantity(str)
	assert.NoError(t, err)
	return q
}
