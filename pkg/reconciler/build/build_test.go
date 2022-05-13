package build

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
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
		podGenerator = &testPodGenerator{}
		decompressor = &fakeDecompressor{}
		ctx          = context.Background()
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient, k8sfakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &Reconciler{
				K8sClient:            k8sfakeClient,
				Client:               fakeClient,
				Lister:               listers.GetBuildLister(),
				PodLister:            listers.GetPodLister(),
				PodGenerator:         podGenerator,
				metadataDecompressor: decompressor,
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
				Volume: &buildapi.BuildPersistentVolumeCache{
					ClaimName: "some-cache-name",
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
						Name: "step-1",
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
						Name: "step-2",
						State: corev1.ContainerState{
							Running: &corev1.ContainerStateRunning{
								StartedAt: metav1.Time{Time: startTime},
							},
						},
					},
					{
						Name: "step-3",
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
										"step-1",
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
						Name: "step-1",
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
						Name: "step-2",
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
										"step-1",
									},
								},
							},
						},
					},
				})
			})
		})

		when("pod succeeded", func() {
			const identifier = "someimage/name@sha256:1234567"
			buildStatusMetadata := &BuildStatusMetadata{
				BuildpackMetadata: []corev1alpha1.BuildpackMetadata{{
					Id:       "io.buildpack.executed",
					Version:  "1.1",
					Homepage: "mysupercoolsite.com",
				}},
				LatestImage:   identifier,
				StackRunImage: "somerun/123@sha256:12334563ad",
				StackID:       "io.buildpacks.stacks.bionic",
			}
			decompressor.returnVal = buildStatusMetadata

			it("sets the build status to Succeeded", func() {
				pod, err := podGenerator.Generate(ctx, bld)
				require.NoError(t, err)
				pod.Status.Phase = corev1.PodSucceeded
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "step-1",
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
						Name: "step-2",
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
											},
										},
									},
									PodName: "build-name-build-pod",
									BuildMetadata: corev1alpha1.BuildpackMetadataList{{
										Id:       "io.buildpack.executed",
										Version:  "1.1",
										Homepage: "mysupercoolsite.com",
									}},
									LatestImage: identifier,
									Stack: corev1alpha1.BuildStack{
										RunImage: "somerun/123@sha256:12334563ad",
										ID:       "io.buildpacks.stacks.bionic",
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
										"step-1",
										"step-2",
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
		})

		when("pod failed", func() {
			it("sets the build status to Failed", func() {
				pod, err := podGenerator.Generate(ctx, bld)
				require.NoError(t, err)
				pod.Status.Phase = corev1.PodFailed
				pod.Status.InitContainerStatuses = []corev1.ContainerStatus{
					{
						Name: "step-1",
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
						Name: "step-2",
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
										"step-1",
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
									"step-1",
								},
							},
						},
					},
					WantErr: false,
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
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "completion",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							Message: "some-message",
						},
					},
				},
			},
		},
	}, nil
}

type fakeDecompressor struct {
	returnVal *BuildStatusMetadata
	err       error
}

func (c *fakeDecompressor) Decompress(string) (*BuildStatusMetadata, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.returnVal, nil
}
