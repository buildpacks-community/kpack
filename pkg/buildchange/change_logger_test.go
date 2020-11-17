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

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestLogBuildInfo(t *testing.T) {
	spec.Run(t, "Log", testLogBuildInfo)
}

func testLogBuildInfo(t *testing.T, when spec.G, it spec.S) {
	diffOptions := testhelpers.DiffOptions{Prefix: "\t", Color: true}
	diffOutBuilder := testhelpers.NewDiffOutBuilder(t).Configure(diffOptions)

	when("reasons are not provided", func() {
		it("errors", func() {
			LogTest{
				reasonsStr:  "",
				changesStr:  "some-change",
				expectedErr: "error validating: build reasons is empty",
			}.execute(t)
		})
	})

	when("changes are not provided", func() {
		it("errors", func() {
			LogTest{
				reasonsStr:  "some-reason",
				changesStr:  "",
				expectedErr: "error validating: build changes is empty",
			}.execute(t)
		})
	})

	when("any of the reasons are invalid", func() {
		it("errors", func() {
			LogTest{
				reasonsStr:  "TRIGGER,INVALID1,BUILDPACK,INVALID2",
				changesStr:  "some-change",
				expectedErr: "error parsing build reasons string 'TRIGGER,INVALID1,BUILDPACK,INVALID2': invalid reason(s): INVALID1,INVALID2",
			}.execute(t)
		})
	})

	when("invalid changes is given for a valid reason", func() {
		when("change is not a JSON string ", func() {
			it("errors", func() {
				LogTest{
					reasonsStr:  "TRIGGER",
					changesStr:  "random-string",
					expectedErr: "error parsing build changes JSON string 'random-string': invalid character 'r' looking for beginning of value",
				}.execute(t)
			})
		})

		when("change JSON has an invalid reason key", func() {
			it("errors", func() {
				LogTest{
					reasonsStr:  "TRIGGER",
					changesStr:  `{"key":"value"}`,
					expectedErr: `error parsing build changes JSON string '{"key":"value"}': error parsing change for reason 'key': unsupported reason`,
				}.execute(t)
			})
		})

		when("change JSON has an valid reason key but invalid change value", func() {
			it("errors", func() {
				LogTest{
					reasonsStr:  "TRIGGER",
					changesStr:  `{"TRIGGER":{"key":"value"}}`,
					expectedErr: `error parsing build changes JSON string '{"TRIGGER":{"key":"value"}}': error parsing change for reason 'TRIGGER': invalid change`,
				}.execute(t)
			})
		})
	})

	when("build reason is TRIGGER and has valid changes", func() {
		it("prints the reason as TRIGGER and the build changes", func() {
			LogTest{
				reasonsStr: "TRIGGER",
				changesStr: changesToStr(t, buildchange.TriggerChange{
					New: "2020-11-20 15:38:15.794105 -0500 EST m=+0.022963826",
				}),
				expectedOut: `Build reason(s): TRIGGER
TRIGGER: A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500
`,
			}.execute(t)
		})
	})

	when("build reason is COMMIT and has valid changes", func() {
		it("prints the reason as COMMIT and the build changes", func() {
			LogTest{
				reasonsStr: "COMMIT",
				changesStr: changesToStr(t, buildchange.CommitChange{
					Old: "old-commit-sha",
					New: "new-commit-sha",
				}),
				expectedOut: diffOutBuilder.Reset().
					Txt("Build reason(s): COMMIT").
					Txt("COMMIT change:").
					Old("Revision: old-commit-sha").
					New("Revision: new-commit-sha").Out(),
			}.execute(t)
		})
	})

	when("build reason is STACK and has valid changes", func() {
		it("prints the reason as STACK and the build changes", func() {
			LogTest{
				reasonsStr: "STACK",
				changesStr: changesToStr(t, buildchange.StackChange{
					Old: "old-run-image",
					New: "new-run-image",
				}),
				expectedOut: diffOutBuilder.Reset().
					Txt("Build reason(s): STACK").
					Txt("STACK change:").
					Old("RunImage: old-run-image").
					New("RunImage: new-run-image").Out(),
			}.execute(t)
		})
	})

	when("build reason is BUILDPACK and has valid changes", func() {
		it("prints the reason as BUILDPACK and the build changes", func() {
			LogTest{
				reasonsStr: "BUILDPACK",
				changesStr: changesToStr(t, buildchange.BuildpackChange{
					Old: []buildchange.BuildpackInfo{
						{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
						{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
					},
					New: []buildchange.BuildpackInfo{
						{Id: "some-buildpack-id", Version: "some-buildpack-new-version"},
					},
				}),
				expectedOut: diffOutBuilder.Reset().
					Txt("Build reason(s): BUILDPACK").
					Txt("BUILDPACK change:").
					Old("another-buildpack-id\tanother-buildpack-old-version").
					Old("some-buildpack-id\tsome-buildpack-old-version").
					New("some-buildpack-id\tsome-buildpack-new-version").Out(),
			}.execute(t)
		})
	})

	when("build reason is CONFIG and has valid changes", func() {
		it("prints the reason as CONFIG and the build changes", func() {
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
				Source: v1alpha1.SourceConfig{
					Git: &v1alpha1.Git{
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
			}

			out := diffOutBuilder.Reset().
				Txt("Build reason(s): CONFIG").
				Txt("CONFIG change:").
				New("bindings:").
				New("- metadataRef:").
				New("    name: some-metadata-ref").
				New("  name: binding-name").
				New("  secretRef:").
				New("    name: some-secret-ref").
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
				NoD("source:").
				Old("  git:").
				Old("    revision: some-git-revision").
				Old("    url: some-git-url").
				Old("  subPath: some-sub-path").
				New("  blob:").
				New("    url: some-blob-url").Out()

			LogTest{
				reasonsStr:  "CONFIG",
				changesStr:  changesToStr(t, buildchange.ConfigChange{Old: oldConfig, New: newConfig}),
				expectedOut: out,
			}.execute(t)
		})
	})

	when("there are multiple reasons and changes", func() {
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
				Source: v1alpha1.SourceConfig{
					Git: &v1alpha1.Git{
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
			}

			out := diffOutBuilder.Reset().
				Txt("Build reason(s): TRIGGER,COMMIT,CONFIG,BUILDPACK,STACK").
				Txt("TRIGGER: A new build was manually triggered on Fri, 20 Nov 2020 15:38:15 -0500").
				Txt("COMMIT change:").
				Old("Revision: old-commit-sha").
				New("Revision: new-commit-sha").
				Txt("CONFIG change:").
				New("bindings:").
				New("- metadataRef:").
				New("    name: some-metadata-ref").
				New("  name: binding-name").
				New("  secretRef:").
				New("    name: some-secret-ref").
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
				NoD("source:").
				Old("  git:").
				Old("    revision: some-git-revision").
				Old("    url: some-git-url").
				Old("  subPath: some-sub-path").
				New("  blob:").
				New("    url: some-blob-url").
				Txt("BUILDPACK change:").
				Old("another-buildpack-id\tanother-buildpack-old-version").
				Old("some-buildpack-id\tsome-buildpack-old-version").
				New("some-buildpack-id\tsome-buildpack-new-version").
				Txt("STACK change:").
				Old("RunImage: old-run-image").
				New("RunImage: new-run-image").Out()

			LogTest{
				reasonsStr: "TRIGGER,COMMIT,CONFIG,BUILDPACK,STACK",
				changesStr: changesToStr(t,
					buildchange.StackChange{
						Old: "old-run-image",
						New: "new-run-image",
					},
					buildchange.CommitChange{
						Old: "old-commit-sha",
						New: "new-commit-sha",
					},
					buildchange.TriggerChange{
						New: "2020-11-20 15:38:15.794105 -0500 EST m=+0.022963826",
					},
					buildchange.BuildpackChange{
						Old: []buildchange.BuildpackInfo{
							{Id: "another-buildpack-id", Version: "another-buildpack-old-version"},
							{Id: "some-buildpack-id", Version: "some-buildpack-old-version"},
						},
						New: []buildchange.BuildpackInfo{
							{Id: "some-buildpack-id", Version: "some-buildpack-new-version"},
						},
					},
					buildchange.ConfigChange{
						Old: oldConfig,
						New: newConfig,
					},
				),
				expectedOut: out,
			}.execute(t)
		})
	})
}

type LogTest struct {
	reasonsStr  string
	changesStr  string
	expectedOut string
	expectedErr string
}

func (l LogTest) execute(t *testing.T) {
	t.Helper()

	fmt.Printf("expected changes: %s\n", l.changesStr)

	out := &bytes.Buffer{}
	logger := log.New(out, "", 0)

	err := buildchange.Log(logger, l.reasonsStr, l.changesStr)

	if l.expectedErr == "" {
		assert.NoError(t, err, "Expected no error\nGot: '%s'\n", err)
	} else {
		assert.NotNil(t, err)
		assert.EqualError(t, err, l.expectedErr, "Error messages do not match\nGot: '%s'\nWant: '%s'\n", err, l.expectedErr)
	}
	assert.Equal(t, l.expectedOut, out.String(), "StdOut messages do not match\nGot: '%s'\nWant: '%s'\n", out.String(), l.expectedOut)
}

func changesToStr(t *testing.T, changes ...buildchange.Change) string {
	t.Helper()

	reasonChangesMap := make(map[v1alpha1.BuildReason]buildchange.Change, len(changes))
	for _, change := range changes {
		reasonChangesMap[change.Reason()] = change
	}

	b, err := json.Marshal(reasonChangesMap)
	assert.NoError(t, err)
	return string(b)
}

func resourceQuantity(t *testing.T, str string) resource.Quantity {
	q, err := resource.ParseQuantity(str)
	assert.NoError(t, err)
	return q
}
