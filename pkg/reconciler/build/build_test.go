package build_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/config"
	"github.com/pivotal/kpack/pkg/reconciler/build"
	"github.com/pivotal/kpack/pkg/reconciler/build/buildfakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
	"github.com/pivotal/kpack/pkg/slsa"
)

func TestBuildReconciler(t *testing.T) {
	spec.Run(t, "Build Reconciler", testBuildReconciler)
}

func testBuildReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		namespace                = "some-namespace"
		buildName                = "build-name"
		key                      = "some-namespace/build-name"
		serviceAccountName       = "someserviceaccount"
		originalGeneration int64 = 1
		// {"buildpackMetadata":[{"id":"some-id","version":"some-version","homepage":"some-homepage"},{"id":"some-other-id","version":"some-other-version"}],"latestImage":"some-latest-image","latestCacheImage":"some-cache-image","stackRunImage":"some-run-image","stackID":"some-stack-id"}
		compressedBuildMetadata = `H4sIAMLug2IAA32QsQ7CIBCG9z4FYW5fwFWXDi6uxuGEixBbaArt0vTdPSAQGqPLhfv+j7vA1jDGn4se5ATifUUPEjzwE7tTwNgWKylaEuPOjtjRsc14xdlpa0qW+yIoohO8sBgFRGNvf66xXuH8d1kyMk3zqD7CBT6AR+f7sd6dWKcjrKwzCIVHVQRUm87T/9wWc9TmxXxJ/aXEsQ9vaPbmAxPQpvpqAQAA`
	)

	var (
		fakeMetadataRetriever = &buildfakes.FakeMetadataRetriever{}
		fakeAttester          = &buildfakes.FakeSLSAAttester{}
		fakeSecretFetcher     = &buildfakes.FakeSecretFetcher{}
		fakeRegistryClient    = &buildfakes.FakeRegistryClient{}
		keychainFactory       = &registryfakes.FakeKeychainFactory{}
		podGenerator          = &testPodGenerator{}
		podProgressLogger     = &testPodProgressLogger{}
		ctx                   = context.Background()
		featureFlags          = config.FeatureFlags{}
		reactors              = make([]reactor, 0)
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)
			for _, r := range reactors {
				k8sfakeClient.PrependReactor(r.verb, r.resource, r.reactionFunc)
			}
			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient, k8sfakeClient}

			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &build.Reconciler{
				K8sClient:         k8sfakeClient,
				Client:            fakeClient,
				KeychainFactory:   keychainFactory,
				Lister:            listers.GetBuildLister(),
				MetadataRetriever: fakeMetadataRetriever,
				PodLister:         listers.GetPodLister(),
				PodGenerator:      podGenerator,
				PodProgressLogger: podProgressLogger,
				Attester:          fakeAttester,
				SecretFetcher:     fakeSecretFetcher,
				RegistryClient:    fakeRegistryClient,
				FeatureFlags:      featureFlags,
			}

			rtesting.PrependGenerateNameReactor(&fakeClient.Fake)

			return r, actionRecorderList, eventList
		})

	bld := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      buildName,
			Namespace: namespace,
			Labels: map[string]string{
				"some/label": "to-pass-through",
			},
			Generation: originalGeneration,
		},
		Spec: buildapi.BuildSpec{
			Tags:               []string{"someimage/name", "someimage/name:tag2", "someimage/name:tag3"},
			ServiceAccountName: serviceAccountName,
			Builder: corev1alpha1.BuildBuilderSpec{
				Image: "somebuilder/123@sha256:12334563ad",
				ImagePullSecrets: []corev1.LocalObjectReference{
					{Name: "some-image-secret"},
				},
			},
			Env: []corev1.EnvVar{
				{Name: "keyA", Value: "valueA"},
				{Name: "keyB", Value: "valueB"},
			},
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
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "giturl.com/git.git",
					Revision: "gitrev1234",
				},
			},
			Cache: &buildapi.BuildCacheConfig{
				Registry: &buildapi.RegistryCache{
					Tag: "registry-cache",
				},
			},
		},
	}

	when("#Reconcile", func() {
		it("schedules a pod to execute the build", func() {
			buildPod, err := podGenerator.Generate(ctx, bld)
			require.NoError(t, err)

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					bld,
				},
				WantErr: false,
				WantCreates: []runtime.Object{
					buildPod,
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Build{
							ObjectMeta: bld.ObjectMeta,
							Spec:       bld.Spec,
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: corev1alpha1.Conditions{
										{
											Type:               corev1alpha1.ConditionSucceeded,
											Status:             corev1.ConditionUnknown,
											LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
										},
									},
								},
								PodName: "build-name-build-pod",
							},
						},
					},
				},
			})
		})

		it("does not schedule a build if already created", func() {
			buildPod, err := podGenerator.Generate(ctx, bld)
			require.NoError(t, err)

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					bld,
					buildPod,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Build{
							ObjectMeta: bld.ObjectMeta,
							Spec:       bld.Spec,
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionUnknown,
										},
									},
								},
								PodName: "build-name-build-pod",
							},
						},
					},
				},
			})
		})

		it("updates observed generation when processing an update", func() {
			buildPod, err := podGenerator.Generate(ctx, bld)
			require.NoError(t, err)
			bld.Generation = 3

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					bld,
					buildPod,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Build{
							ObjectMeta: bld.ObjectMeta,
							Spec:       bld.Spec,
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 3,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionUnknown,
										},
									},
								},
								PodName: "build-name-build-pod",
							},
						},
					},
				},
			})
		})

		it("does not update status if there is no update", func() {
			buildPod, err := podGenerator.Generate(ctx, bld)
			require.NoError(t, err)

			bld.Status = buildapi.BuildStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionSucceeded,
							Status: corev1.ConditionUnknown,
						},
					},
				},
				PodName: buildPod.Name,
			}

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					bld,
					buildPod,
				},
				WantErr: false,
			})
		})

		it("saves error creating build to status", func() {
			podGenerator.returnErr = errors.New("display me in the status")

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					bld,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Build{
							ObjectMeta: bld.ObjectMeta,
							Spec:       bld.Spec,
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:    corev1alpha1.ConditionSucceeded,
											Status:  corev1.ConditionFalse,
											Message: "display me in the status",
										},
									},
								},
							},
						},
					},
				},
			})
		})

		it("gracefully handles a pod that has already been created", func() {
			buildPod, err := podGenerator.Generate(ctx, bld)
			require.NoError(t, err)

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					bld,
					buildPod,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Build{
							ObjectMeta: bld.ObjectMeta,
							Spec:       bld.Spec,
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: corev1alpha1.Conditions{
										{
											Type:               corev1alpha1.ConditionSucceeded,
											Status:             corev1.ConditionUnknown,
											LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
										},
									},
								},
								PodName: "build-name-build-pod",
							},
						},
					},
				},
			})
		})

		when("pod executing", func() {
			it("updates the status step states with the statuses of the containers", func() {
				pod, err := podGenerator.Generate(ctx, bld)
				require.NoError(t, err)

				startTime := time.Now()
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "prepare",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    0,
								Reason:      "Terminated",
								Message:     "Message",
								ContainerID: "container.ID",
							},
						},
					},
					{
						Name: "analyze",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Time{Time: startTime},
							},
						},
					},
					{
						Name: "detect",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "Waiting",
								Message: "My Turn",
							},
						},
					},
				}

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionSucceeded,
												Status: corev1.ConditionUnknown,
											},
										},
									},
									PodName: "build-name-build-pod",
									StepStates: []corev1.ContainerState{
										{

											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     "Message",
												ContainerID: "container.ID",
											},
										},
										{
											Running: &corev1.ContainerStateRunning{
												StartedAt: metav1.Time{Time: startTime},
											},
										},
										{
											Waiting: &corev1.ContainerStateWaiting{
												Reason:  "Waiting",
												Message: "My Turn",
											},
										},
									},
									StepsCompleted: []string{
										"prepare",
									},
								},
							},
						},
					},
				})
			})

			it("updates the status with the container status when a container is waiting", func() {
				pod, err := podGenerator.Generate(ctx, bld)
				require.NoError(t, err)

				pod.Status.Phase = corev1.PodPending
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "prepare",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    0,
								Reason:      "Terminated",
								Message:     "Message",
								ContainerID: "container.ID",
							},
						},
					},
					{
						Name: "analyze",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "ImagePullBackOff",
								Message: "Can't pull",
							},
						},
					},
				}

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionSucceeded,
												Status:  corev1.ConditionUnknown,
												Reason:  "ImagePullBackOff",
												Message: "Can't pull",
											},
										},
									},
									PodName: "build-name-build-pod",
									StepStates: []corev1.ContainerState{
										{

											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     "Message",
												ContainerID: "container.ID",
											},
										},
										{
											Waiting: &corev1.ContainerStateWaiting{
												Reason:  "ImagePullBackOff",
												Message: "Can't pull",
											},
										},
									},
									StepsCompleted: []string{
										"prepare",
									},
								},
							},
						},
					},
				})
			})
		})

		when("pod succeeded", func() {
			it("sets the build status to Succeeded", func() {
				pod, err := podGenerator.Generate(ctx, bld)
				require.NoError(t, err)
				pod.Status.Phase = corev1.PodSucceeded
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "prepare",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    0,
								Reason:      "Terminated",
								Message:     "Message",
								ContainerID: "container.ID",
							},
						},
					},
					{
						Name: "analyze",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    0,
								Reason:      "Terminated",
								Message:     "Message",
								ContainerID: "container.ID2",
							},
						},
					},
				}
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "completion",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								Message: compressedBuildMetadata,
							},
						},
					},
				}

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionSucceeded,
												Status: corev1.ConditionTrue,
												Reason: build.ReasonCompleted,
											},
										},
									},
									PodName: "build-name-build-pod",
									BuildMetadata: corev1alpha1.BuildpackMetadataList{
										{
											Id:       "some-id",
											Version:  "some-version",
											Homepage: "some-homepage",
										},
										{
											Id:      "some-other-id",
											Version: "some-other-version",
										},
									},
									LatestImage:      "some-latest-image",
									LatestCacheImage: "some-cache-image",
									Stack: corev1alpha1.BuildStack{
										RunImage: "some-run-image",
										ID:       "some-stack-id",
									},
									StepStates: []corev1.ContainerState{
										{
											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     "Message",
												ContainerID: "container.ID",
											},
										},
										{
											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     "Message",
												ContainerID: "container.ID2",
											},
										},
										{
											Terminated: &corev1.ContainerStateTerminated{
												Message: compressedBuildMetadata,
											},
										},
									},
									StepsCompleted: []string{
										"prepare",
										"analyze",
										"completion",
									},
								},
							},
						},
					},
				})
			})

			it("does not recreate pods if build has finished", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: bld.ObjectMeta,
							Spec:       bld.Spec,
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								BuildMetadata: corev1alpha1.BuildpackMetadataList{{
									Id:      "io.buildpack.previouslyfetched",
									Version: "1.1",
								}},
								PodName:     "build-name-build-pod",
								LatestImage: "previously/fetched@sha256:abcd",
								StepStates: []corev1.ContainerState{
									{
										Terminated: &corev1.ContainerStateTerminated{
											ExitCode:    0,
											Reason:      "Terminated",
											Message:     "Message",
											ContainerID: "container.ID",
										},
									},
									{
										Terminated: &corev1.ContainerStateTerminated{
											ExitCode:    0,
											Reason:      "Terminated",
											Message:     "Message",
											ContainerID: "container.ID2",
										},
									},
								},
								StepsCompleted: []string{
									"step-1",
									"step-2",
								},
							},
						},
					},
					WantErr: false,
				})
			})

			when("running windows build pods", func() {
				var pod *corev1.Pod
				it.Before(func() {
					appImageSecretRef := registry.SecretRef{
						ServiceAccount: bld.Spec.ServiceAccountName,
						Namespace:      bld.Namespace,
					}
					appImageKeychain := &registryfakes.FakeKeychain{}
					keychainFactory.AddKeychainForSecretRef(t, appImageSecretRef, appImageKeychain)
					buildMetadata := &cnb.BuildMetadata{
						BuildpackMetadata: corev1alpha1.BuildpackMetadataList{{
							Id:       "io.buildpack.executed",
							Version:  "1.1",
							Homepage: "mysupercoolsite.com",
						}},
						LatestCacheImage: "some-latest-cache",
						LatestImage:      "some-latest-image",
						StackID:          "some-stack-id",
						StackRunImage:    "some-stack-run-image",
					}
					fakeMetadataRetriever.GetBuildMetadataReturns(buildMetadata, nil)
					var err error
					pod, err = podGenerator.Generate(ctx, bld)
					require.NoError(t, err)
					pod.Status.Phase = corev1.PodSucceeded
					pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
						{
							Name: "prepare",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode:    0,
									Reason:      "Terminated",
									Message:     "Message",
									ContainerID: "container.ID",
								},
							},
						},
						{
							Name: "completion",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode:    0,
									Reason:      "Terminated",
									Message:     "Message",
									ContainerID: "container.ID2",
								},
							},
						},
					}
					pod.Spec.NodeSelector = map[string]string{"kubernetes.io/os": "windows"}
				})

				it("retrieves the build metadata from the registry", func() {
					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							bld,
							pod,
						},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &buildapi.Build{
									ObjectMeta: bld.ObjectMeta,
									Spec:       bld.Spec,
									Status: buildapi.BuildStatus{
										Status: corev1alpha1.Status{
											ObservedGeneration: originalGeneration,
											Conditions: corev1alpha1.Conditions{
												{
													Type:   corev1alpha1.ConditionSucceeded,
													Status: corev1.ConditionTrue,
													Reason: build.ReasonCompleted,
												},
											},
										},
										PodName: "build-name-build-pod",
										BuildMetadata: corev1alpha1.BuildpackMetadataList{{
											Id:       "io.buildpack.executed",
											Version:  "1.1",
											Homepage: "mysupercoolsite.com",
										}},
										LatestCacheImage: "some-latest-cache",
										LatestImage:      "some-latest-image",
										Stack: corev1alpha1.BuildStack{
											RunImage: "some-stack-run-image",
											ID:       "some-stack-id",
										},
										StepStates: []corev1.ContainerState{
											{
												Terminated: &corev1.ContainerStateTerminated{
													ExitCode:    0,
													Reason:      "Terminated",
													Message:     "Message",
													ContainerID: "container.ID",
												},
											},
											{
												Terminated: &corev1.ContainerStateTerminated{
													ExitCode:    0,
													Reason:      "Terminated",
													Message:     "Message",
													ContainerID: "container.ID2",
												},
											},
										},
										StepsCompleted: []string{
											"prepare",
											"completion",
										},
									},
								},
							},
						},
					})

					assert.Equal(t, fakeMetadataRetriever.GetBuildMetadataCallCount(), 1)
				})

				it("does not fetch metadata if already retrieved", func() {
					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							&buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionSucceeded,
												Status: corev1.ConditionTrue,
											},
										},
									},
									BuildMetadata: corev1alpha1.BuildpackMetadataList{{
										Id:      "io.buildpack.previouslyfetched",
										Version: "1.1",
									}},
									PodName:     "build-name-build-pod",
									LatestImage: "previously/fetched@sha256:abcd",
									StepStates: []corev1.ContainerState{
										{
											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     "Message",
												ContainerID: "container.ID",
											},
										},
										{
											Waiting: nil,
											Running: nil,
											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     "Message",
												ContainerID: "container.ID2",
											},
										},
									},
									StepsCompleted: []string{
										"step-1",
										"step-2", //todo realistic names
									},
								},
							},
							pod,
						},
						WantErr: false,
					})
					assert.Equal(t, fakeMetadataRetriever.GetBuildMetadataCallCount(), 0)
				})
			})
		})

		when("pod failed", func() {
			it("sets the build status to Failed", func() {
				pod, err := podGenerator.Generate(ctx, bld)
				require.NoError(t, err)
				pod.Status.Phase = corev1.PodFailed
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "prepare",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    1,
								Reason:      "Terminated",
								Message:     "Errors",
								ContainerID: "container.ID",
							},
						},
					},
					{
						Name: "analyze",
						State: corev1.ContainerState{
							Waiting: &corev1.ContainerStateWaiting{
								Reason:  "Waiting",
								Message: "My Turn",
							},
						},
					},
				}

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionSucceeded,
												Status:  corev1.ConditionFalse,
												Reason:  string(corev1.PodFailed),
												Message: "Error:  Fake container logs",
											},
										},
									},
									PodName: "build-name-build-pod",
									StepStates: []corev1.ContainerState{
										{
											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    1,
												Reason:      "Terminated",
												Message:     "Errors",
												ContainerID: "container.ID",
											},
										},
										{
											Waiting: &corev1.ContainerStateWaiting{
												Reason:  "Waiting",
												Message: "My Turn",
											},
										},
									},
									StepsCompleted: []string{},
								},
							},
						},
					},
				})
			})

			it("does not recreate pods if build has finished", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: bld.ObjectMeta,
							Spec:       bld.Spec,
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionFalse,
										},
									},
								},
								PodName: "build-name-build-pod",
								StepStates: []corev1.ContainerState{
									{
										Terminated: &corev1.ContainerStateTerminated{
											ExitCode:    1,
											Reason:      "Terminated",
											Message:     "Errors",
											ContainerID: "container.ID",
										},
									},
									{
										Waiting: &corev1.ContainerStateWaiting{
											Reason:  "Waiting",
											Message: "My Turn",
										},
									},
								},
								StepsCompleted: []string{
									"prepare",
								},
							},
						},
					},
					WantErr: false,
				})
			})

			when("the failed container's status does not include a message", func() {
				it("sets the build's status condition message to the pod's status message", func() {
					pod, err := podGenerator.Generate(ctx, bld)
					require.NoError(t, err)
					pod.Status.Phase = corev1.PodFailed
					pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
						{
							Name: "prepare",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode:    1,
									Reason:      "Terminated",
									Message:     "Container prepare terminated with error",
									ContainerID: "container.ID",
								},
							},
						},
						{
							Name: "analyze",
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason:  "Waiting",
									Message: "My Turn",
								},
							},
						},
					}
					pod.Status.Message = "Something bad happened"

					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							bld,
							pod,
						},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &buildapi.Build{
									ObjectMeta: bld.ObjectMeta,
									Spec:       bld.Spec,
									Status: buildapi.BuildStatus{
										Status: corev1alpha1.Status{
											ObservedGeneration: originalGeneration,
											Conditions: corev1alpha1.Conditions{
												{
													Type:    corev1alpha1.ConditionSucceeded,
													Status:  corev1.ConditionFalse,
													Reason:  string(corev1.PodFailed),
													Message: "Error: Something bad happened Fake container logs",
												},
											},
										},
										PodName: "build-name-build-pod",
										StepStates: []corev1.ContainerState{
											{
												Terminated: &corev1.ContainerStateTerminated{
													ExitCode:    1,
													Reason:      "Terminated",
													Message:     "Container prepare terminated with error",
													ContainerID: "container.ID",
												},
											},
											{
												Waiting: &corev1.ContainerStateWaiting{
													Reason:  "Waiting",
													Message: "My Turn",
												},
											},
										},
										StepsCompleted: []string{},
									},
								},
							},
						},
					})
				})
			})
		})

		when("a build pod cannot be created", func() {
			it("returns a permanent error", func() {
				pod, err := podGenerator.Generate(ctx, bld)
				require.NoError(t, err)

				podName := fmt.Sprintf("%s-build-pod", buildName)
				reactors = append(reactors, reactor{
					verb:     "create",
					resource: "pods",
					reactionFunc: func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						a := action.DeepCopy().(clientgotesting.CreateAction)
						return a.GetObject().(metav1.Object).GetName() == podName, nil, k8serrors.NewInvalid(schema.ParseGroupKind("v1/pod"), podName, field.ErrorList{})
					},
				})

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						bld,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						pod,
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionSucceeded,
												Status:  corev1.ConditionFalse,
												Message: `v1/pod "build-name-build-pod" is invalid`,
											},
										},
									},
								},
							},
						},
					},
				})
			})
		})

		when("pod needs cleanup", func() {
			featureFlags.InjectedSidecarSupport = true
			var startTime = time.Now()

			it("updates activeDeadlineSeconds when a build terminates", func() {
				pod := &corev1.Pod{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      bld.GetName() + "-build-pod",
						Namespace: bld.GetNamespace(),
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "completion",
								Image: "completion-image",
							},
							{
								Name:  "sidecar",
								Image: "sidecar-image",
							},
						},
					},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "completion",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode:    0,
										Reason:      "Terminated",
										Message:     "Message",
										ContainerID: "container.ID",
									},
								},
							},
							{
								Name: "sidecar",
								State: corev1.ContainerState{
									Running: &corev1.ContainerStateRunning{
										StartedAt: metav1.Time{Time: startTime},
									},
								},
							},
						},
						Phase: corev1.PodRunning,
					},
				}

				deadlinePatch, err := json.Marshal(map[string]interface{}{
					"spec": map[string]interface{}{
						"activeDeadlineSeconds": 1,
					},
				})
				require.NoError(t, err)

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						bld,
						pod,
					},
					WantErr: false,
					WantPatches: []clientgotesting.PatchActionImpl{
						{
							Name:      pod.Name,
							PatchType: types.MergePatchType,
							Patch:     deadlinePatch,
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionSucceeded,
												Status: corev1.ConditionUnknown,
											},
										},
									},
									PodName: "build-name-build-pod",
									StepStates: []corev1.ContainerState{
										{
											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     "Message",
												ContainerID: "container.ID",
											},
										},
									},
									StepsCompleted: []string{
										"completion",
									},
								},
							},
						},
					},
				})
			})

			it("marks build as successful if completion completes even if pod fails", func() {
				deadline := int64(1)
				pod := &corev1.Pod{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      bld.GetName() + "-build-pod",
						Namespace: bld.GetNamespace(),
					},
					Spec: corev1.PodSpec{
						ActiveDeadlineSeconds: &deadline,
						Containers: []corev1.Container{
							{
								Name:  "completion",
								Image: "completion-image",
							},
							{
								Name:  "sidecar",
								Image: "sidecar-image",
							},
						},
					},
					Status: corev1.PodStatus{
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "completion",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode:    0,
										Reason:      "Terminated",
										Message:     compressedBuildMetadata,
										ContainerID: "container.ID",
									},
								},
							},
							{
								Name: "sidecar",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode:    0,
										Reason:      "Terminated",
										Message:     "Message",
										ContainerID: "container.ID2",
									},
								},
							},
						},
						Phase:  corev1.PodFailed,
						Reason: "DeadlineExceeded",
					},
				}

				bld.Status = buildapi.BuildStatus{
					Status: corev1alpha1.Status{
						ObservedGeneration: originalGeneration,
						Conditions: corev1alpha1.Conditions{
							{
								Type:   corev1alpha1.ConditionSucceeded,
								Status: corev1.ConditionUnknown,
							},
						},
					},
					PodName: "build-name-build-pod",
					StepStates: []corev1.ContainerState{
						{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    0,
								Reason:      "Terminated",
								Message:     compressedBuildMetadata,
								ContainerID: "container.ID",
							},
						},
					},
					StepsCompleted: []string{
						"completion",
					},
				}

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status: buildapi.BuildStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionSucceeded,
												Status: corev1.ConditionTrue,
												Reason: build.ReasonCompleted,
											},
										},
									},
									PodName: "build-name-build-pod",
									BuildMetadata: corev1alpha1.BuildpackMetadataList{
										{
											Id:       "some-id",
											Version:  "some-version",
											Homepage: "some-homepage",
										},
										{
											Id:      "some-other-id",
											Version: "some-other-version",
										},
									},
									LatestImage:      "some-latest-image",
									LatestCacheImage: "some-cache-image",
									Stack: corev1alpha1.BuildStack{
										RunImage: "some-run-image",
										ID:       "some-stack-id",
									},
									StepStates: []corev1.ContainerState{
										{
											Terminated: &corev1.ContainerStateTerminated{
												ExitCode:    0,
												Reason:      "Terminated",
												Message:     compressedBuildMetadata,
												ContainerID: "container.ID",
											},
										},
									},
									StepsCompleted: []string{
										"completion",
									},
								},
							},
						},
					},
				})
			})
		})

		when("attestation is enabled", func() {
			var (
				startTime = time.Now()
				endTime   = startTime.Add(5 * time.Minute)

				makeSecret = func(t *testing.T, alg string) *corev1.Secret {
					t.Helper()
					data := make(map[string][]byte)
					switch alg {
					case "cosign":
						cosignKey, err := cosign.GenerateKeyPair(func(bool) ([]byte, error) { return nil, nil })
						require.NoError(t, err)
						data["cosign.password"] = []byte("")
						data["cosign.key"] = cosignKey.PrivateBytes
					case "ed25519":
						_, priv, err := ed25519.GenerateKey(rand.Reader)
						require.NoError(t, err)
						key, err := x509.MarshalPKCS8PrivateKey(priv)
						require.NoError(t, err)
						data["ssh-privatekey"] = pem.EncodeToMemory(&pem.Block{
							Type:  "PRIVATE KEY",
							Bytes: key,
						})

					case "rsa":
						fallthrough
					default:
						priv, err := rsa.GenerateKey(rand.Reader, 1024)
						require.NoError(t, err)
						key, err := x509.MarshalPKCS8PrivateKey(priv)
						require.NoError(t, err)
						data["ssh-privatekey"] = pem.EncodeToMemory(&pem.Block{
							Type:  "PRIVATE KEY",
							Bytes: key,
						})

					}

					return &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:            fmt.Sprintf("%v-secret", alg),
							Namespace:       bld.GetNamespace(),
							ResourceVersion: "4",
							Annotations: map[string]string{
								"kpack.io/slsa": "",
							},
						},
						Data: data,
					}
				}

				pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:            bld.GetName() + "-build-pod",
						Namespace:       bld.GetNamespace(),
						ResourceVersion: "1",
					},
					Spec: corev1.PodSpec{
						InitContainers: []corev1.Container{
							{Name: "prepare", Image: "prepare-image"},
						},
						Containers: []corev1.Container{
							{Name: "completion", Image: "completion-image"},
						},
						NodeName: "some-node",
					},
					Status: corev1.PodStatus{
						InitContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "prepare",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode:    0,
										Reason:      "Terminated",
										Message:     "Message",
										ContainerID: "container.ID",
										StartedAt:   metav1.NewTime(startTime),
										FinishedAt:  metav1.NewTime(endTime),
									},
								},
							},
						},
						ContainerStatuses: []corev1.ContainerStatus{
							{
								Name: "completion",
								State: corev1.ContainerState{
									Terminated: &corev1.ContainerStateTerminated{
										ExitCode:    0,
										Reason:      "Terminated",
										Message:     compressedBuildMetadata,
										ContainerID: "container.ID",
										StartedAt:   metav1.NewTime(startTime),
										FinishedAt:  metav1.NewTime(endTime),
									},
								},
							},
						},
						Phase: corev1.PodSucceeded,
					},
				}
				ns = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:            bld.GetNamespace(),
						ResourceVersion: "2",
					},
				}

				rsaSecret     *corev1.Secret
				ed25519Secret *corev1.Secret
				cosignSecret  *corev1.Secret

				sa = &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:            bld.ServiceAccount(),
						Namespace:       bld.GetNamespace(),
						ResourceVersion: "5",
					},
				}

				expectedStatus = buildapi.BuildStatus{
					Status: corev1alpha1.Status{
						ObservedGeneration: originalGeneration,
						Conditions: corev1alpha1.Conditions{
							{
								Type:   corev1alpha1.ConditionSucceeded,
								Status: corev1.ConditionTrue,
								Reason: build.ReasonCompleted,
							},
						},
					},
					PodName: "build-name-build-pod",
					BuildMetadata: corev1alpha1.BuildpackMetadataList{
						{
							Id:       "some-id",
							Version:  "some-version",
							Homepage: "some-homepage",
						},
						{
							Id:      "some-other-id",
							Version: "some-other-version",
						},
					},
					LatestImage:            "some-latest-image",
					LatestCacheImage:       "some-cache-image",
					LatestAttestationImage: "some-attestation-image",
					Stack: corev1alpha1.BuildStack{
						RunImage: "some-run-image",
						ID:       "some-stack-id",
					},
					StepStates: []corev1.ContainerState{
						{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    0,
								Reason:      "Terminated",
								Message:     "Message",
								ContainerID: "container.ID",
								StartedAt:   metav1.NewTime(startTime),
								FinishedAt:  metav1.NewTime(endTime),
							},
						},
						{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:    0,
								Reason:      "Terminated",
								Message:     compressedBuildMetadata,
								ContainerID: "container.ID",
								StartedAt:   metav1.NewTime(startTime),
								FinishedAt:  metav1.NewTime(endTime),
							},
						},
					},
					StepsCompleted: []string{
						"prepare",
						"completion",
					},
				}
			)

			featureFlags.GenerateSlsaAttestation = true
			bld.ResourceVersion = "0"

			it.Before(func() {
				fakeAttester.AttestBuildReturns(in_toto.Statement{}, nil)
				fakeAttester.WriteReturns(nil, "some-attestation-image", nil)
				fakeSecretFetcher.SecretsForServiceAccountReturns([]*corev1.Secret{}, nil)
				fakeSecretFetcher.SecretsForSystemServiceAccountReturns([]*corev1.Secret{}, nil)

				appImageSecretRef := registry.SecretRef{
					ServiceAccount:   bld.ServiceAccount(),
					Namespace:        bld.Namespace,
					ImagePullSecrets: bld.BuilderSpec().ImagePullSecrets,
				}
				appImageKeychain := &registryfakes.FakeKeychain{}
				keychainFactory.AddKeychainForSecretRef(t, appImageSecretRef, appImageKeychain)

				rsaSecret = makeSecret(t, "rsa")
				ed25519Secret = makeSecret(t, "ed25519")
				cosignSecret = makeSecret(t, "cosign")
			})

			it("generates unsigned attestation when there's no secrets", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						ns, sa, rsaSecret, ed25519Secret, cosignSecret,
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status:     expectedStatus,
							},
						},
					},
				})

				require.Equal(t, 1, fakeAttester.AttestBuildCallCount())
				_, _, _, _, id, deps := fakeAttester.AttestBuildArgsForCall(0)
				require.Equal(t, slsa.BuilderID("https://kpack.io/slsa/unsigned-build"), id)
				require.Len(t, deps, 4)

				require.Equal(t, 1, fakeAttester.SignCallCount())
				_, _, signer := fakeAttester.SignArgsForCall(0)
				require.Len(t, signer, 0)

				require.Equal(t, 1, fakeAttester.WriteCallCount())
				_, img, _, _ := fakeAttester.WriteArgsForCall(0)
				require.Equal(t, img, "some-latest-image")
			})

			it("generates signed attestation when there's secrets in builder service account", func() {
				fakeSecretFetcher.SecretsForServiceAccountReturns([]*corev1.Secret{
					rsaSecret,
				}, nil)

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						ns, sa, rsaSecret, ed25519Secret, cosignSecret,
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status:     expectedStatus,
							},
						},
					},
				})

				require.Equal(t, 1, fakeAttester.AttestBuildCallCount())
				_, _, _, _, id, deps := fakeAttester.AttestBuildArgsForCall(0)
				require.Equal(t, slsa.BuilderID("https://kpack.io/slsa/signed-build"), id)
				require.Len(t, deps, 5)

				require.Equal(t, 1, fakeAttester.SignCallCount())
				_, _, signer := fakeAttester.SignArgsForCall(0)
				require.Len(t, signer, 1)

				require.Equal(t, 1, fakeAttester.WriteCallCount())
				_, img, _, _ := fakeAttester.WriteArgsForCall(0)
				require.Equal(t, img, "some-latest-image")
			})
			it("generates signed attestation when there's secrets in system service account", func() {
				fakeSecretFetcher.SecretsForSystemServiceAccountReturns([]*corev1.Secret{
					cosignSecret, ed25519Secret,
				}, nil)

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						ns, sa, rsaSecret, ed25519Secret, cosignSecret,
						bld,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Build{
								ObjectMeta: bld.ObjectMeta,
								Spec:       bld.Spec,
								Status:     expectedStatus,
							},
						},
					},
				})

				require.Equal(t, 1, fakeAttester.AttestBuildCallCount())
				_, _, _, _, id, deps := fakeAttester.AttestBuildArgsForCall(0)
				require.Equal(t, slsa.BuilderID("https://kpack.io/slsa/signed-build"), id)
				require.Len(t, deps, 5)

				require.Equal(t, 1, fakeAttester.SignCallCount())
				_, _, signer := fakeAttester.SignArgsForCall(0)
				require.Len(t, signer, 2)

				require.Equal(t, 1, fakeAttester.WriteCallCount())
				_, img, _, _ := fakeAttester.WriteArgsForCall(0)
				require.Equal(t, img, "some-latest-image")
			})

		})

		when("cascadeDelete is enabled", func() {
			bld.Spec.CascadeDelete = true
			bld.Finalizers = append(bld.GetFinalizers(), build.BuildFinalizer)
			bld.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
			bld.Status.LatestImage = "foo/bar@sha256:123"
			bld.Status.Conditions = corev1alpha1.Conditions{
				{
					Type:   corev1alpha1.ConditionSucceeded,
					Status: corev1.ConditionTrue,
					Reason: build.ReasonCompleted,
				},
			}

			it("deletes the image from the registry", func() {
				keychainFactory.AddKeychainForSecretRef(t, registry.SecretRef{
					ServiceAccount: bld.Spec.ServiceAccountName,
					Namespace:      bld.Namespace,
				}, nil)
				finalizerPatch, err := json.Marshal(map[string]interface{}{
					"metadata": map[string]interface{}{
						"finalizers":      []string{},
						"resourceVersion": bld.ResourceVersion,
					},
				})
				require.NoError(t, err)

				rt.Test(rtesting.TableRow{
					Key:     key,
					WantErr: false,
					Objects: []runtime.Object{
						bld,
					},
					WantPatches: []clientgotesting.PatchActionImpl{
						{
							Name:      bld.Name,
							PatchType: types.MergePatchType,
							Patch:     finalizerPatch,
						},
					},
					CmpOpts: []cmp.Option{
						cmp.FilterPath(func(p cmp.Path) bool {
							t.Log(p.String())
							return strings.HasSuffix(p.String(), "ObjectMeta.Finalizers")
						}, cmp.Ignore()),
					},
				})

				require.Len(t, fakeRegistryClient.Invocations()["Delete"], 1)
				require.Equal(t, fakeRegistryClient.Invocations()["Delete"][0], []interface{}{nil, bld.Status.LatestImage})
			})
		})
	})
}

type testPodGenerator struct {
	returnErr error
}

func (tpg testPodGenerator) Generate(ctx context.Context, build buildpod.BuildPodable) (*corev1.Pod, error) {
	if tpg.returnErr != nil {
		return nil, tpg.returnErr
	}

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      build.GetName() + "-build-pod",
			Namespace: build.GetNamespace(),
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "step-1",
				},
				{
					Name: "step-2",
				},
				{
					Name: "step-3",
				},
			},
			ImagePullSecrets: build.BuilderSpec().ImagePullSecrets,
		},
	}, nil
}

type reactor struct {
	verb         string
	resource     string
	reactionFunc clientgotesting.ReactionFunc
}

type testPodProgressLogger struct {
	returnErr error
}

func (p testPodProgressLogger) GetTerminationMessage(pod *corev1.Pod, s *corev1.ContainerStatus) (string, error) {
	if p.returnErr != nil {
		return "error", p.returnErr
	}
	return " Fake container logs", nil
}
