package builder_test

import (
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	rtesting "github.com/knative/pkg/reconciler/testing"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/cnb"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/builder"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/builder/builderfakes"
	"github.com/pivotal/build-service-system/pkg/registry"
)

//go:generate counterfeiter . MetadataRetriever

func TestBuildReconciler(t *testing.T) {
	spec.Run(t, "Builder Reconciler", testBuilderReconciler)
}

func testBuilderReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeMetadataRetriever := &builderfakes.FakeMetadataRetriever{}
	fakeClient := fake.NewSimpleClientset(&v1alpha1.Builder{})

	fakeEnqueuer := &builderfakes.FakeEnqueuer{}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
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

			return r, actionRecorderList, eventList, &rtesting.FakeStatsReporter{}
		})

	const (
		builderName            = "builder-name"
		namespace              = "some-namespace"
		key                    = "some-namespace/builder-name"
		imageName              = "some/builder@sha256acf123"
		initalGeneration int64 = 1
	)

	builder := &v1alpha1.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name:       builderName,
			Namespace:  namespace,
			Generation: initalGeneration,
		},
		Spec: v1alpha1.BuilderSpec{
			Image: imageName,
		},
	}

	when("#Reconcile", func() {
		it.Before(func() {
			_, err := fakeClient.BuildV1alpha1().Builders(namespace).Create(builder)
			require.Nil(t, err)

			fakeMetadataRetriever.GetBuilderBuildpacksReturns(cnb.BuilderMetadata{
				{
					ID:      "buildpack.version",
					Version: "version",
				},
			}, nil)
		})

		it("fetches the metadata for the configured builder", func() {
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
								Status: duckv1alpha1.Status{
									ObservedGeneration: 1,
								},
								BuilderMetadata: []v1alpha1.BuildpackMetadata{
									{
										ID:      "buildpack.version",
										Version: "version",
									},
								},
							},
						},
					},
				},
			})

			require.Equal(t, fakeMetadataRetriever.GetBuilderBuildpacksCallCount(), 1)
			assert.Equal(t, fakeMetadataRetriever.GetBuilderBuildpacksArgsForCall(0), registry.NewNoAuthImageRef(imageName))
		})

		it("schedule next polling when update policy is not set", func() {
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
								Status: duckv1alpha1.Status{
									ObservedGeneration: 1,
								},
								BuilderMetadata: []v1alpha1.BuildpackMetadata{
									{
										ID:      "buildpack.version",
										Version: "version",
									},
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
								Status: duckv1alpha1.Status{
									ObservedGeneration: 1,
								},
								BuilderMetadata: []v1alpha1.BuildpackMetadata{
									{
										ID:      "buildpack.version",
										Version: "version",
									},
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
								Status: duckv1alpha1.Status{
									ObservedGeneration: 1,
								},
								BuilderMetadata: []v1alpha1.BuildpackMetadata{
									{
										ID:      "buildpack.version",
										Version: "version",
									},
								},
							},
						},
					},
				},
			})

			assert.Equal(t, 0, fakeEnqueuer.EnqueueCallCount())
		})

		when("buildpack metadata did not change", func() {
			it("does not update the status", func() {
				builder.Status.BuilderMetadata = []v1alpha1.BuildpackMetadata{
					{
						ID:      "buildpack.version",
						Version: "version",
					},
				}

				rt.Test(rtesting.TableRow{
					Key:     key,
					Objects: []runtime.Object{builder},
					WantErr: false,
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
