package slsa

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrfake "github.com/google/go-containerregistry/pkg/v1/fake"
	slsav1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1alpha2 "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/config"
)

func TestAttester(t *testing.T) {
	spec.Run(t, "Test SLSA generation", testAttester)
}

func testAttester(t *testing.T, when spec.G, it spec.S) {
	build := &buildv1alpha2.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-1",
			Namespace: "default",
		},
		Spec: buildv1alpha2.BuildSpec{
			Builder: corev1alpha1.BuildBuilderSpec{
				Image: "some-registry.io/builder-image@sha256:de9964b5f501a77b8cf549659f81e29dbac4f8df7f1890ddc2b568dbed428b73",
			},
			Cache: &buildv1alpha2.BuildCacheConfig{
				Volume: &buildv1alpha2.BuildPersistentVolumeCache{
					ClaimName: "test-cache",
				},
			},
			RunImage: buildv1alpha2.BuildSpecImage{
				Image: "some-registry.io/run-image@sha256:e817bca35911221677b678bf8bf29a18c17ce867b29bd9d0b0c3342c063854e5",
			},
			ServiceAccountName: "default",
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					Revision: "82cb521d636b282340378d80a6307a08e3d4a4c4",
					URL:      "https://some-git.com/org/repo.git",
				},
			},
			Tags: []string{
				"some-registry.io/some/repo",
				"some-registry.io/some/repo:b1.20231108.210915",
			},
		},
	}

	buildMetadata := &cnb.BuildMetadata{
		LatestImage: "some-registry.io/some/repo@sha256:27227f3eaf20afcd527f31bcaaa1a10d14f30c2a99b313c86b981906c54c07b9",
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-build-1-build-pod",
		},
		Spec: corev1.PodSpec{
			NodeName: "some-node",
		},
		Status: corev1.PodStatus{
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "prepare",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   0,
							Reason:     "Completed",
							StartedAt:  metav1.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC),
							FinishedAt: metav1.Date(2023, time.January, 1, 1, 0, 0, 0, time.UTC),
						},
					},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "completion",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   0,
							Reason:     "Completed",
							StartedAt:  metav1.Date(2023, time.January, 1, 1, 0, 0, 0, time.UTC),
							FinishedAt: metav1.Date(2023, time.January, 1, 2, 0, 0, 0, time.UTC),
						},
					},
				}},
		},
	}

	builderImage := ggcrfake.FakeImage{}
	builderImage.ConfigFileReturns(&ggcrv1.ConfigFile{
		Config: ggcrv1.Config{
			Labels: map[string]string{
				"io.buildpacks.buildpack.order": `[{"group":[{"id":"paketo-buildpacks/java-native-image","version":"8.23.0"}]},{"group":[{"id":"paketo-buildpacks/java","version":"10.4.0"}]},{"group":[{"id":"paketo-buildpacks/go","version":"4.6.1"}]},{"group":[{"id":"paketo-buildpacks/procfile","version":"5.6.7"}]}]`,
				"io.buildpacks.stack.id":        "io.buildpacks.stacks.jammy.tiny",
			},
		},
	}, nil)

	appImage := ggcrfake.FakeImage{}
	appImage.ConfigFileReturns(&ggcrv1.ConfigFile{
		Config: ggcrv1.Config{
			Labels: map[string]string{
				"io.buildpacks.project.metadata": `{"source":{"type":"git","version":{"commit":"some-commitsh"},"metadata":{"repository":"https://some-git.repo","revision":"some-branch"}}}`,
			},
		},
	}, nil)

	r := NewImageReader(&fakeFetcher{
		images: map[string]ggcrv1.Image{
			"some-registry.io/builder-image@sha256:de9964b5f501a77b8cf549659f81e29dbac4f8df7f1890ddc2b568dbed428b73": &builderImage,
			"some-registry.io/some/repo@sha256:27227f3eaf20afcd527f31bcaaa1a10d14f30c2a99b313c86b981906c54c07b9":     &appImage,
		},
	})

	attester := &Attester{
		Version: "v0.0.0",

		LifecycleProvider: &fakeLifecycleProvider{},
		ImageReader:       r,

		Images: config.Images{
			BuildInitImage: "build-init-image", BuildInitWindowsImage: "build-init-windows-image",
			BuildWaiterImage: "build-waiter-image",
			CompletionImage:  "completion-image", CompletionWindowsImage: "completion-windows-image",
			RebaseImage: "rebase-image",
		},
		Config:   config.Config{EnablePriorityClasses: false, MaximumPlatformApiVersion: "", SshTrustUnknownHosts: true},
		Features: config.FeatureFlags{InjectedSidecarSupport: false},
	}

	it("", func() {
		stmt, err := attester.AttestBuild(build, buildMetadata, pod, authn.DefaultKeychain, UnsignedBuildID)
		require.NoError(t, err)

		expected := `{
  "_type": "https://in-toto.io/Statement/v0.1",
  "predicateType": "https://slsa.dev/provenance/v1",
  "subject": [
    {
      "name": "some-registry.io/some/repo",
      "digest": {
        "sha256": "27227f3eaf20afcd527f31bcaaa1a10d14f30c2a99b313c86b981906c54c07b9"
      }
    }
  ],
  "predicate": {
    "buildDefinition": {
      "buildType": "https://github.com/buildpacks-community/kpack/blob/vv0.0.0/docs/slsa.md",
      "externalParameters": {
        "tags": [
          "some-registry.io/some/repo",
          "some-registry.io/some/repo:b1.20231108.210915"
        ],
        "builder": {
          "image": "some-registry.io/builder-image@sha256:de9964b5f501a77b8cf549659f81e29dbac4f8df7f1890ddc2b568dbed428b73"
        },
        "serviceAccountName": "default",
        "source": {
          "git": {
            "url": "https://some-git.com/org/repo.git",
            "revision": "82cb521d636b282340378d80a6307a08e3d4a4c4"
          }
        },
        "cache": {
          "volume": {
            "persistentVolumeClaimName": "test-cache"
          }
        },
        "runImage": {
          "image": "some-registry.io/run-image@sha256:e817bca35911221677b678bf8bf29a18c17ce867b29bd9d0b0c3342c063854e5"
        },
        "resources": {}
      },
      "internalParameters": {
        "builderImage": "some-registry.io/builder-image@sha256:de9964b5f501a77b8cf549659f81e29dbac4f8df7f1890ddc2b568dbed428b73",
        "systemNamespace": "",
        "systemServiceAccount": "",
        "enablePriorityClasses": false,
        "maximumPlatformApiVersion": "",
        "sshTrustUnknownHosts": true,
        "scalingFactor": 0,
        "buildInitImage": "build-init-image",
        "buildInitWindowsImage": "build-init-windows-image",
        "buildWaiterImage": "build-waiter-image",
        "completionImage": "completion-image",
        "completionWindowsImage": "completion-windows-image",
        "rebaseImage": "rebase-image",
        "injectedSidecarSupport": false,
        "generateSlsaAttestation": false
      },
      "resolvedDependencies": [
        {
          "uri": "https://some-git.repo",
          "digest": {
            "sha1": "some-commitsh"
          },
          "name": "source"
        },
        {
          "uri": "some-registry.io/builder-image",
          "digest": {
            "sha256": "de9964b5f501a77b8cf549659f81e29dbac4f8df7f1890ddc2b568dbed428b73"
          },
          "name": "builder-image",
          "annotations": {
            "io.buildpacks.buildpack.order": "[{\"group\":[{\"id\":\"paketo-buildpacks/java-native-image\",\"version\":\"8.23.0\"}]},{\"group\":[{\"id\":\"paketo-buildpacks/java\",\"version\":\"10.4.0\"}]},{\"group\":[{\"id\":\"paketo-buildpacks/go\",\"version\":\"4.6.1\"}]},{\"group\":[{\"id\":\"paketo-buildpacks/procfile\",\"version\":\"5.6.7\"}]}]",
            "io.buildpacks.stack.id": "io.buildpacks.stacks.jammy.tiny"
          }
        }
      ]
    },
    "runDetails": {
      "builder": {
        "id": "https://kpack.io/slsa/unsigned-build",
        "version": {
          "kpack": "v0.0.0",
          "lifecycle": "1.2.3"
        }
      },
      "metadata": {
        "invocationID": "https://kpack.io/default/test-build-1/test-build-1-build-pod@some-node",
        "startedOn": "2023-01-01T00:00:00Z",
        "finishedOn": "2023-01-01T02:00:00Z"
      }
    }
  }
}`

		actual, err := json.MarshalIndent(stmt, "", "  ")
		require.NoError(t, err)

		require.Equal(t, expected, string(actual))
	})

	when("using the builder dependency fn", func() {
		it("records single object", func() {
			fn := WithVersionedObject("Namespace", &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "some-namespace",
					ResourceVersion: "5",
				},
			})

			actual, err := fn()
			require.NoError(t, err)

			require.Equal(t,
				slsav1.ResourceDescriptor{
					Name:      "Namespace",
					Content:   []byte(`{"name":"some-namespace","resourceVersion":"5"}`),
					MediaType: "application/json",
				},
				actual,
			)
		})

		it("records multiple objects", func() {
			fn := WithVersionedObjects("Secret", []K8sObject{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "some-secret-1",
						ResourceVersion: "4",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "some-secret-2",
						ResourceVersion: "10",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "some-secret-3",
						ResourceVersion: "1041",
					},
				},
			})

			actual, err := fn()
			require.NoError(t, err)

			require.Equal(t,
				slsav1.ResourceDescriptor{
					Name:      "Secret",
					Content:   []byte(`[{"name":"some-secret-1","resourceVersion":"4"},{"name":"some-secret-2","resourceVersion":"10"},{"name":"some-secret-3","resourceVersion":"1041"}]`),
					MediaType: "application/json",
				},
				actual,
			)
		})
	})
}

type fakeLifecycleProvider struct {
}

func (l *fakeLifecycleProvider) Metadata() (cnb.LifecycleMetadata, error) {
	return cnb.LifecycleMetadata{
		LifecycleInfo: cnb.LifecycleInfo{
			Version: "1.2.3",
		},
	}, nil
}
