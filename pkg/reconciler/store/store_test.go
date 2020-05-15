package store_test

import (
	"fmt"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/store"
	"github.com/pivotal/kpack/pkg/reconciler/store/storefakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestStoreReconciler(t *testing.T) {
	spec.Run(t, "Store Reconciler", testStoreReconciler)
}

func testStoreReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		storeName               = "some-store"
		storeKey                = storeName
		initialGeneration int64 = 1
	)
	var (
		fakeStoreReader = &storefakes.FakeStoreReader{}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)

			r := &store.Reconciler{
				Client:      fakeClient,
				StoreReader: fakeStoreReader,
				StoreLister: listers.GetStoreLister(),
			}
			return r, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	store := &expv1alpha1.Store{
		ObjectMeta: metav1.ObjectMeta{
			Name:       storeName,
			Generation: initialGeneration,
		},
		Spec: expv1alpha1.StoreSpec{
			Sources: []expv1alpha1.StoreImage{
				{
					Image: "some.registry/some-image-1",
				},
				{
					Image: "some.registry/some-image-2",
				},
			},
		},
	}

	when("#Reconcile", func() {
		readBuildpacks := []expv1alpha1.StoreBuildpack{
			{
				BuildpackInfo: expv1alpha1.BuildpackInfo{
					Id:      "paketo-buildpacks/node-engine",
					Version: "0.0.116",
				},
				DiffId: "sha256:d57937f5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf4",
				StoreImage: expv1alpha1.StoreImage{
					Image: "some.registry/some-image-1",
				},
				Order: nil,
			},
			{
				BuildpackInfo: expv1alpha1.BuildpackInfo{
					Id:      "paketo-buildpacks/npm",
					Version: "0.0.71",
				},
				DiffId: "sha256:c67840e5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf5",
				StoreImage: expv1alpha1.StoreImage{
					Image: "some.registry/some-image-2",
				},
				Order: nil,
			},
		}

		it("saves metadata to the status", func() {
			fakeStoreReader.ReadReturns(readBuildpacks, nil)

			rt.Test(rtesting.TableRow{
				Key: storeKey,
				Objects: []runtime.Object{
					store,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &expv1alpha1.Store{
							ObjectMeta: store.ObjectMeta,
							Spec:       store.Spec,
							Status: expv1alpha1.StoreStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								Buildpacks: readBuildpacks,
							},
						},
					},
				},
			})

			assert.Equal(t, 1, fakeStoreReader.ReadCallCount())

			assert.Equal(t, store.Spec.Sources, fakeStoreReader.ReadArgsForCall(0))
		})

		it("does not update the status with no status change", func() {
			fakeStoreReader.ReadReturns(readBuildpacks, nil)

			store.Status = expv1alpha1.StoreStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				Buildpacks: readBuildpacks,
			}
			rt.Test(rtesting.TableRow{
				Key: storeKey,
				Objects: []runtime.Object{
					store,
				},
				WantErr: false,
			})
		})

		it("sets the status to Ready False if error reading buildpacks", func() {
			fakeStoreReader.ReadReturns(nil, fmt.Errorf("no buildpacks left"))

			rt.Test(rtesting.TableRow{
				Key: storeKey,
				Objects: []runtime.Object{
					store,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &expv1alpha1.Store{
							ObjectMeta: store.ObjectMeta,
							Spec:       store.Spec,
							Status: expv1alpha1.StoreStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Message: "no buildpacks left",
											Type:    corev1alpha1.ConditionReady,
											Status:  corev1.ConditionFalse,
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
}
