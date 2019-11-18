package clusterbuilder_test

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
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/clusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/clusterbuilder/clusterbuilderfakes"
)

func TestCluterBuilderReconciler(t *testing.T) {
	spec.Run(t, "Cluster Builder Reconciler", testClusterBuilderReconciler)
}

func testClusterBuilderReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeMetadataRetriever := &clusterbuilderfakes.FakeMetadataRetriever{}

	fakeEnqueuer := &clusterbuilderfakes.FakeEnqueuer{}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}
			r := &clusterbuilder.Reconciler{
				Client:               fakeClient,
				ClusterBuilderLister: listers.GetClusterBuilderLister(),
				MetadataRetriever:    fakeMetadataRetriever,
				Enqueuer:             fakeEnqueuer,
			}

			return r, actionRecorderList, eventList, &rtesting.FakeStatsReporter{}
		})

	const (
		clusterBuilderName             = "cluster-builder-name"
		key                            = "some-namespace/builder-name"
		clusterBuilderKey              = "cluster-builder-name"
		clusterImageName               = "some/cluster-builder"
		clusterBuilderIdentifier       = "some/cluster-builder@sha256:resolved-builder-digest"
		initalGeneration         int64 = 1
	)

	clusterBuilder := &v1alpha1.ClusterBuilder{
		ObjectMeta: v1.ObjectMeta{
			Name:       clusterBuilderName,
			Generation: initalGeneration,
		},
		Spec: v1alpha1.BuilderSpec{
			Image: clusterImageName,
		},
	}

	when("#Reconcile", func() {
		when("cluster builder", func() {
			when("metadata is available", func() {
				fakeMetadataRetriever.GetBuilderImageReturns(v1alpha1.BuilderRecord{
					Image: clusterBuilderIdentifier,
					Stack: v1alpha1.BuildStack{
						RunImage: "",
						ID:       "",
					},
					Buildpacks: v1alpha1.BuildpackMetadataList{
						{
							ID:      "buildpack.version",
							Version: "version",
						},
					},
				}, nil)

				it("saves metadata to the status", func() {
					testBuilder := &v1alpha1.ClusterBuilder{
						ObjectMeta: clusterBuilder.ObjectMeta,
						Spec:       clusterBuilder.Spec,
						Status: v1alpha1.BuilderStatus{
							Status: duckv1alpha1.Status{
								ObservedGeneration: 1,
								Conditions: duckv1alpha1.Conditions{
									{
										Type:   duckv1alpha1.ConditionReady,
										Status: corev1.ConditionTrue,
									},
								},
							},
							BuilderMetadata: []v1alpha1.BuildpackMetadata{
								{
									ID:      "buildpack.version",
									Version: "version",
								},
							},
							LatestImage: clusterBuilderIdentifier,
						},
					}
					rt.Test(rtesting.TableRow{
						Key:     clusterBuilderKey,
						Objects: []runtime.Object{clusterBuilder},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: testBuilder,
							},
						},
					})

					require.Equal(t, fakeMetadataRetriever.GetBuilderImageCallCount(), 1)
				})

				it("schedule next polling when update policy is not set", func() {
					rt.Test(rtesting.TableRow{
						Key:     clusterBuilderKey,
						Objects: []runtime.Object{clusterBuilder},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &v1alpha1.ClusterBuilder{
									ObjectMeta: clusterBuilder.ObjectMeta,
									Spec:       clusterBuilder.Spec,
									Status: v1alpha1.BuilderStatus{
										Status: duckv1alpha1.Status{
											ObservedGeneration: 1,
											Conditions: duckv1alpha1.Conditions{
												{
													Type:   duckv1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
											},
										},
										BuilderMetadata: []v1alpha1.BuildpackMetadata{
											{
												ID:      "buildpack.version",
												Version: "version",
											},
										},
										LatestImage: clusterBuilderIdentifier,
									},
								},
							},
						},
					})
					assert.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
				})

				it("does schedule polling when update policy is set to polling", func() {
					clusterBuilder.Spec.UpdatePolicy = v1alpha1.Polling
					rt.Test(rtesting.TableRow{
						Key:     clusterBuilderKey,
						Objects: []runtime.Object{clusterBuilder},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &v1alpha1.ClusterBuilder{
									ObjectMeta: clusterBuilder.ObjectMeta,
									Spec:       clusterBuilder.Spec,
									Status: v1alpha1.BuilderStatus{
										Status: duckv1alpha1.Status{
											ObservedGeneration: 1,
											Conditions: duckv1alpha1.Conditions{
												{
													Type:   duckv1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
											},
										},
										BuilderMetadata: []v1alpha1.BuildpackMetadata{
											{
												ID:      "buildpack.version",
												Version: "version",
											},
										},
										LatestImage: clusterBuilderIdentifier,
									},
								},
							},
						},
					})
					assert.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
				})

				it("does not schedule polling when update policy is set to external", func() {
					clusterBuilder.Spec.UpdatePolicy = v1alpha1.External
					rt.Test(rtesting.TableRow{
						Key:     clusterBuilderKey,
						Objects: []runtime.Object{clusterBuilder},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &v1alpha1.ClusterBuilder{
									ObjectMeta: clusterBuilder.ObjectMeta,
									Spec:       clusterBuilder.Spec,
									Status: v1alpha1.BuilderStatus{
										Status: duckv1alpha1.Status{
											ObservedGeneration: 1,
											Conditions: duckv1alpha1.Conditions{
												{
													Type:   duckv1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
											},
										},
										BuilderMetadata: []v1alpha1.BuildpackMetadata{
											{
												ID:      "buildpack.version",
												Version: "version",
											},
										},
										LatestImage: clusterBuilderIdentifier,
									},
								},
							},
						},
					})

					assert.Equal(t, 0, fakeEnqueuer.EnqueueCallCount())
				})

				it("does not update the status with no status change", func() {
					clusterBuilder.Status = v1alpha1.BuilderStatus{
						Status: duckv1alpha1.Status{
							ObservedGeneration: clusterBuilder.Generation,
							Conditions: duckv1alpha1.Conditions{
								{
									Type:   duckv1alpha1.ConditionReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
						BuilderMetadata: []v1alpha1.BuildpackMetadata{
							{
								ID:      "buildpack.version",
								Version: "version",
							},
						},
						LatestImage: clusterBuilderIdentifier,
					}

					rt.Test(rtesting.TableRow{
						Key:     clusterBuilderKey,
						Objects: []runtime.Object{clusterBuilder},
						WantErr: false,
					})
				})
			})

			when("metadata is not available", func() {
				fakeMetadataRetriever.GetBuilderImageReturns(v1alpha1.BuilderRecord{}, errors.New("unavailable metadata"))

				it("saves not ready to the builder status", func() {
					rt.Test(rtesting.TableRow{
						Key:     clusterBuilderKey,
						Objects: []runtime.Object{clusterBuilder},
						WantErr: true,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &v1alpha1.ClusterBuilder{
									ObjectMeta: clusterBuilder.ObjectMeta,
									Spec:       clusterBuilder.Spec,
									Status: v1alpha1.BuilderStatus{
										Status: duckv1alpha1.Status{
											ObservedGeneration: 1,
											Conditions: duckv1alpha1.Conditions{
												{
													Type:    duckv1alpha1.ConditionReady,
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
		})

		it("does not return error on nonexistent builder", func() {
			rt.Test(rtesting.TableRow{
				Key:     key,
				WantErr: false,
			})
		})
	})
}
