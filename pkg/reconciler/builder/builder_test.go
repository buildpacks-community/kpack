package builder_test

import (
	"errors"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/builder"
	"github.com/pivotal/kpack/pkg/reconciler/builder/builderfakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestBuilderReconciler(t *testing.T) {
	spec.Run(t, "Builder Reconciler", testBuilderReconciler)
}

func testBuilderReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeMetadataRetriever := &builderfakes.FakeMetadataRetriever{}

	fakeEnqueuer := &builderfakes.FakeEnqueuer{}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}
			r := &builder.Reconciler{
				Client:            fakeClient,
				BuilderLister:     listers.GetBuilderLister(),
				MetadataRetriever: fakeMetadataRetriever,
				Enqueuer:          fakeEnqueuer,
			}

			return r, actionRecorderList, eventList
		})

	const (
		builderName             = "builder-name"
		namespace               = "some-namespace"
		key                     = "some-namespace/builder-name"
		imageName               = "some/builder"
		builderIdentifier       = "some/builder@sha256:resolved-builder-digest"
		runImgIdentifier        = "some/runImage@sha256:resolved-builder-digest"
		initalGeneration  int64 = 1
	)

	builder := &v1alpha1.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name:       builderName,
			Namespace:  namespace,
			Generation: initalGeneration,
		},
		Spec: v1alpha1.BuilderWithSecretsSpec{
			BuilderSpec: v1alpha1.BuilderSpec{
				Image:        imageName,
				UpdatePolicy: v1alpha1.Polling,
			},
			ImagePullSecrets: nil,
		},
	}

	when("#Reconcile", func() {
		when("metadata is available", func() {
			fakeMetadataRetriever.GetBuilderImageReturns(v1alpha1.BuilderRecord{
				Image: builderIdentifier,
				Stack: v1alpha1.BuildStack{
					RunImage: runImgIdentifier,
					ID:       "io.buildpacks.stacks.bionic",
				},
				Buildpacks: v1alpha1.BuildpackMetadataList{
					{
						Id:      "buildpack.version",
						Version: "version",
					},
				},
			}, nil)

			it("saves metadata to the status", func() {
				testBuilder := &v1alpha1.Builder{
					ObjectMeta: builder.ObjectMeta,
					Spec:       builder.Spec,
					Status: v1alpha1.BuilderStatus{
						Status: corev1alpha1.Status{
							ObservedGeneration: 1,
							Conditions: corev1alpha1.Conditions{
								{
									Type:               corev1alpha1.ConditionReady,
									Status:             corev1.ConditionTrue,
									LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
								},
							},
						},
						BuilderMetadata: []v1alpha1.BuildpackMetadata{
							{
								Id:      "buildpack.version",
								Version: "version",
							},
						},
						LatestImage: builderIdentifier,
						Stack: v1alpha1.BuildStack{
							RunImage: runImgIdentifier,
							ID:       "io.buildpacks.stacks.bionic",
						},
					},
				}
				rt.Test(rtesting.TableRow{
					Key:     key,
					Objects: []runtime.Object{builder},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: testBuilder,
						},
					},
				})

				require.Equal(t, fakeMetadataRetriever.GetBuilderImageCallCount(), 1)
			})

			it("schedule next polling when update policy is set to polling", func() {
				rt.Test(rtesting.TableRow{
					Key:     key,
					Objects: []runtime.Object{builder},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Builder{
								ObjectMeta: builder.ObjectMeta,
								Spec:       builder.Spec,
								Status: v1alpha1.BuilderStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: 1,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
										},
									},
									BuilderMetadata: []v1alpha1.BuildpackMetadata{
										{
											Id:      "buildpack.version",
											Version: "version",
										},
									},
									LatestImage: builderIdentifier,
									Stack: v1alpha1.BuildStack{
										RunImage: runImgIdentifier,
										ID:       "io.buildpacks.stacks.bionic",
									},
								},
							},
						},
					},
				})
				assert.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
			})

			it("does schedule polling when update policy is set to polling", func() {
				builder.Spec.UpdatePolicy = v1alpha1.Polling
				rt.Test(rtesting.TableRow{
					Key:     key,
					Objects: []runtime.Object{builder},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Builder{
								ObjectMeta: builder.ObjectMeta,
								Spec:       builder.Spec,
								Status: v1alpha1.BuilderStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: 1,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
										},
									},
									BuilderMetadata: []v1alpha1.BuildpackMetadata{
										{
											Id:      "buildpack.version",
											Version: "version",
										},
									},
									LatestImage: builderIdentifier,
									Stack: v1alpha1.BuildStack{
										RunImage: runImgIdentifier,
										ID:       "io.buildpacks.stacks.bionic",
									},
								},
							},
						},
					},
				})
				assert.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
			})

			it("does not schedule polling when update policy is set to external", func() {
				builder.Spec.UpdatePolicy = v1alpha1.External
				rt.Test(rtesting.TableRow{
					Key:     key,
					Objects: []runtime.Object{builder},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Builder{
								ObjectMeta: builder.ObjectMeta,
								Spec:       builder.Spec,
								Status: v1alpha1.BuilderStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: 1,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
										},
									},
									BuilderMetadata: []v1alpha1.BuildpackMetadata{
										{
											Id:      "buildpack.version",
											Version: "version",
										},
									},
									LatestImage: builderIdentifier,
									Stack: v1alpha1.BuildStack{
										RunImage: runImgIdentifier,
										ID:       "io.buildpacks.stacks.bionic",
									},
								},
							},
						},
					},
				})

				assert.Equal(t, 0, fakeEnqueuer.EnqueueCallCount())
			})

			it("does not update the status with no status change", func() {
				builder.Status = v1alpha1.BuilderStatus{
					Status: corev1alpha1.Status{
						ObservedGeneration: builder.Generation,
						Conditions: corev1alpha1.Conditions{
							{
								Type:   corev1alpha1.ConditionReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
					BuilderMetadata: []v1alpha1.BuildpackMetadata{
						{
							Id:      "buildpack.version",
							Version: "version",
						},
					},
					LatestImage: builderIdentifier,
					Stack: v1alpha1.BuildStack{
						RunImage: runImgIdentifier,
						ID:       "io.buildpacks.stacks.bionic",
					},
				}

				rt.Test(rtesting.TableRow{
					Key:     key,
					Objects: []runtime.Object{builder},
					WantErr: false,
				})
			})
		})

		when("metadata is not available", func() {
			fakeMetadataRetriever.GetBuilderImageReturns(v1alpha1.BuilderRecord{}, errors.New("unavailable metadata"))

			it("saves not ready to the builder status", func() {
				rt.Test(rtesting.TableRow{
					Key:     key,
					Objects: []runtime.Object{builder},
					WantErr: true,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Builder{
								ObjectMeta: builder.ObjectMeta,
								Spec:       builder.Spec,
								Status: v1alpha1.BuilderStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: 1,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionReady,
												Status:  corev1.ConditionFalse,
												Message: "unavailable metadata",
											},
										},
									},
								},
							},
						},
					},
				})

				assert.Equal(t, fakeEnqueuer.EnqueueCallCount(), 1)
			})
		})

		it("does not return error on nonexistent builder", func() {
			rt.Test(rtesting.TableRow{
				Key:     key,
				WantErr: false,
			})
		})
	})
}
