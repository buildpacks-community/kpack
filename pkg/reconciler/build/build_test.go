package build_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sclevine/spec"
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
	"github.com/pivotal/kpack/pkg/reconciler/build"
	"github.com/pivotal/kpack/pkg/reconciler/build/buildfakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
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
	)

	var (
		fakeMetadataRetriever  = &buildfakes.FakeMetadataRetriever{}
		keychainFactory        = &registryfakes.FakeKeychainFactory{}
		podGenerator           = &testPodGenerator{}
		ctx                    = context.Background()
		injectedSidecarSupport = false
		reactors               = make([]reactor, 0)
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
				K8sClient:              k8sfakeClient,
				Client:                 fakeClient,
				KeychainFactory:        keychainFactory,
				Lister:                 listers.GetBuildLister(),
				MetadataRetriever:      fakeMetadataRetriever,
				PodLister:              listers.GetPodLister(),
				PodGenerator:           podGenerator,
				InjectedSidecarSupport: injectedSidecarSupport,
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
				compressedBuildMetadata, err := ioutil.ReadFile(filepath.Join("testdata", "metadata"))
				require.NoError(t, err)

				pod.Status.ContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "completion",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								Message: string(compressedBuildMetadata),
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
												Message: string(compressedBuildMetadata),
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
												Message: "prepare failed: Errors",
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
													Message: "Something bad happened",
												},
											},
										},
										PodName: "build-name-build-pod",
										StepStates: []corev1.ContainerState{
											{
												Terminated: &corev1.ContainerStateTerminated{
													ExitCode:    1,
													Reason:      "Terminated",
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
			injectedSidecarSupport = true
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
				compressedBuildMetadata, err := os.ReadFile(filepath.Join("testdata", "metadata"))
				require.NoError(t, err)

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
										Message:     string(compressedBuildMetadata),
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
								Message:     string(compressedBuildMetadata),
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
												Message:     string(compressedBuildMetadata),
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
