package build_test

import (
	"testing"
	"time"

	lcyclemd "github.com/buildpack/lifecycle/metadata"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	rtesting "github.com/knative/pkg/reconciler/testing"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/cnb"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build/buildfakes"
)

//go:generate counterfeiter . MetadataRetriever

func TestBuildReconciler(t *testing.T) {
	spec.Run(t, "Build Reconciler", testBuildReconciler)
}

func testBuildReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		namespace                = "some-namespace"
		buildName                = "build-name"
		builderName              = "builder-name"
		key                      = "some-namespace/build-name"
		serviceAccountName       = "someserviceaccount"
		originalGeneration int64 = 1
	)

	var (
		fakeMetadataRetriever = &buildfakes.FakeMetadataRetriever{}
	)

	podGenerator := &testPodGenerator{}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient, k8sfakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &build.Reconciler{
				K8sClient:         k8sfakeClient,
				Client:            fakeClient,
				Lister:            listers.GetBuildLister(),
				PodLister:         listers.GetPodLister(),
				MetadataRetriever: fakeMetadataRetriever,
				PodGenerator:      podGenerator,
				BuilderLister:     listers.GetBuilderLister(),
			}

			rtesting.PrependGenerateNameReactor(&fakeClient.Fake)

			return r, actionRecorderList, eventList, &rtesting.FakeStatsReporter{}
		})

	builder := &v1alpha1.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name:      builderName,
			Namespace: namespace,
		},
		Spec: v1alpha1.BuilderSpec{
			ImagePullSecrets: []corev1.LocalObjectReference{
				{Name: "some-image-secret"},
			},
		},
		Status: v1alpha1.BuilderStatus{
			Status: duckv1alpha1.Status{
				Conditions: duckv1alpha1.Conditions{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
			LatestImage: "somebuilder/123@sha256:12334563ad",
		},
	}

	build := &v1alpha1.Build{
		ObjectMeta: v1.ObjectMeta{
			Name:      buildName,
			Namespace: namespace,
			Labels: map[string]string{
				"some/label": "to-pass-through",
			},
			Generation: originalGeneration,
		},
		Spec: v1alpha1.BuildSpec{
			Tag:            "someimage/name",
			ServiceAccount: serviceAccountName,
			BuilderRef:     builderName,
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
			Source: v1alpha1.SourceConfig{
				Git: &v1alpha1.Git{
					URL:      "giturl.com/git.git",
					Revision: "gitrev1234",
				},
			},
			CacheName:            "some-cache-name",
			AdditionalImageNames: []string{"someimage/name:tag2", "someimage/name:tag3"},
		},
	}

	when("#Reconcile", func() {
		it("schedules a pod to execute the build", func() {
			buildPod, err := podGenerator.Generate(build, builder)
			require.NoError(t, err)

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					builder,
					build,
				},
				WantErr: false,
				WantCreates: []runtime.Object{
					buildPod,
				},
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &v1alpha1.Build{
							ObjectMeta: build.ObjectMeta,
							Spec:       build.Spec,
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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

		it("does not schedule a build if already created", func() {
			buildPod, err := podGenerator.Generate(build, builder)
			require.NoError(t, err)

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					builder,
					build,
					buildPod,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &v1alpha1.Build{
							ObjectMeta: build.ObjectMeta,
							Spec:       build.Spec,
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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

		it("does not schedule the build if builder is not found", func() {
			buildPod, err := podGenerator.Generate(build, builder)
			require.NoError(t, err)

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					build,
					buildPod,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &v1alpha1.Build{
							ObjectMeta: build.ObjectMeta,
							Spec:       build.Spec,
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: duckv1alpha1.Conditions{
										{
											Type:    duckv1alpha1.ConditionSucceeded,
											Status:  corev1.ConditionFalse,
											Reason:  v1alpha1.BuilderNotFound,
											Message: "Unable to find builder builder-name.",
										},
									},
								},
							},
						},
					},
				},
			})
		})

		it("does not schedule the build if builder failed to resolved", func() {
			builder.Status.LatestImage = ""
			builder.Status.Conditions = []duckv1alpha1.Condition{
				{
					Type:   duckv1alpha1.ConditionReady,
					Status: corev1.ConditionFalse,
				},
			}
			buildPod, err := podGenerator.Generate(build, builder)
			require.NoError(t, err)

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					builder,
					build,
					buildPod,
				},
				WantErr: true,
			})
		})

		it("updates observed generation when processing an update", func() {
			buildPod, err := podGenerator.Generate(build, builder)
			require.NoError(t, err)
			build.Generation = 3

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					builder,
					build,
					buildPod,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &v1alpha1.Build{
							ObjectMeta: build.ObjectMeta,
							Spec:       build.Spec,
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: 3,
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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
			buildPod, err := podGenerator.Generate(build, builder)
			require.NoError(t, err)

			build.Status = v1alpha1.BuildStatus{
				Status: duckv1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1alpha1.Conditions{
						{
							Type:   duckv1alpha1.ConditionSucceeded,
							Status: corev1.ConditionUnknown,
						},
					},
				},
				PodName: buildPod.Name,
			}

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					builder,
					build,
					buildPod,
				},
				WantErr: false,
			})
		})

		when("pod executing", func() {
			it("updates the status with the status of the pod", func() {
				pod, err := podGenerator.Generate(build, builder)
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
								StartedAt: v1.Time{Time: startTime},
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
						builder,
						build,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Build{
								ObjectMeta: build.ObjectMeta,
								Spec:       build.Spec,
								Status: v1alpha1.BuildStatus{
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: duckv1alpha1.Conditions{
											{
												Type:   duckv1alpha1.ConditionSucceeded,
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
												StartedAt: v1.Time{Time: startTime},
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
		})

		when("pod succeeded", func() {
			const identifier = "someimage/name@sha256:1234567"
			builtImage := cnb.BuiltImage{
				Identifier:  identifier,
				CompletedAt: time.Now(),
				BuildpackMetadata: []lcyclemd.BuildpackMetadata{{
					ID:      "io.buildpack.executed",
					Version: "1.1",
				}},
			}
			fakeMetadataRetriever.GetBuiltImageReturns(builtImage, nil)

			it("sets the build status to Succeeded", func() {
				pod, err := podGenerator.Generate(build, builder)
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
						builder,
						build,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Build{
								ObjectMeta: build.ObjectMeta,
								Spec:       build.Spec,
								Status: v1alpha1.BuildStatus{
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: duckv1alpha1.Conditions{
											{
												Type:   duckv1alpha1.ConditionSucceeded,
												Status: corev1.ConditionTrue,
											},
										},
									},
									PodName: "build-name-build-pod",
									BuildMetadata: v1alpha1.BuildpackMetadataList{{
										ID:      "io.buildpack.executed",
										Version: "1.1",
									}},
									Builder:     "somebuilder/123@sha256:12334563ad",
									LatestImage: identifier,
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

				assert.Equal(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 1)
			})

			it("does not fetch metadata if already retrieved", func() {
				pod, err := podGenerator.Generate(build, builder)
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
						builder,
						&v1alpha1.Build{
							ObjectMeta: build.ObjectMeta,
							Spec:       build.Spec,
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								BuildMetadata: v1alpha1.BuildpackMetadataList{{
									ID:      "io.buildpack.previouslyfetched",
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
				assert.Equal(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 0)
			})

			it("does not recreate pods if build has finished", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						builder,
						&v1alpha1.Build{
							ObjectMeta: build.ObjectMeta,
							Spec:       build.Spec,
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								BuildMetadata: v1alpha1.BuildpackMetadataList{{
									ID:      "io.buildpack.previouslyfetched",
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
				pod, err := podGenerator.Generate(build, builder)
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
						builder,
						build,
						pod,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Build{
								ObjectMeta: build.ObjectMeta,
								Spec:       build.Spec,
								Status: v1alpha1.BuildStatus{
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: duckv1alpha1.Conditions{
											{
												Type:   duckv1alpha1.ConditionSucceeded,
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
						builder,
						&v1alpha1.Build{
							ObjectMeta: build.ObjectMeta,
							Spec:       build.Spec,
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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
}

func (testPodGenerator) Generate(build *v1alpha1.Build, builder *v1alpha1.Builder) (*corev1.Pod, error) {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name:      build.PodName(),
			Namespace: build.Namespace(),
			Labels:    build.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(build),
			},
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
			ImagePullSecrets: builder.Spec.ImagePullSecrets,
		},
	}, nil
}
