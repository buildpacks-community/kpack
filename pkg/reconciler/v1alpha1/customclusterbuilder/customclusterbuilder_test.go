package customclusterbuilder_test

import (
	"errors"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	kpackcore "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/customclusterbuilder"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestCustomClusterBuilderReconciler(t *testing.T) {
	spec.Run(t, "Custom Cluster Builder Reconciler", testCustomClusterBuilderReconciler)
}

func testCustomClusterBuilderReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		customBuilderName             = "custom-builder"
		customBuilderKey              = customBuilderName
		customBuilderTag              = "example.com/custom-builder"
		customBuilderIdentifier       = "example.com/custom-builder@sha256:resolved-builder-digest"
		initialGeneration       int64 = 1
	)

	var (
		builderCreator  = &testhelpers.FakeBuilderCreator{}
		keychainFactory = &registryfakes.FakeKeychainFactory{}
		fakeTracker     = testhelpers.FakeTracker{}
		fakeRepoFactory = func(store *expv1alpha1.Store) cnb.BuildpackRepository {
			return testhelpers.FakeBuildpackRepository{Store: store}
		}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &customclusterbuilder.Reconciler{
				Client:                     fakeClient,
				CustomClusterBuilderLister: listers.GetCustomClusterBuilderLister(),
				RepoFactory:                fakeRepoFactory,
				BuilderCreator:             builderCreator,
				KeychainFactory:            keychainFactory,
				Tracker:                    fakeTracker,
				StoreLister:                listers.GetStoreLister(),
				StackLister:                listers.GetStackLister(),
			}
			return r, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}, &rtesting.FakeStatsReporter{}
		})

	store := &expv1alpha1.Store{
		ObjectMeta: v1.ObjectMeta{
			Name: "some-store",
		},
		Spec:   expv1alpha1.StoreSpec{},
		Status: expv1alpha1.StoreStatus{},
	}

	stack := &expv1alpha1.Stack{
		ObjectMeta: v1.ObjectMeta{
			Name: "some-stack",
		},
	}

	customBuilder := &expv1alpha1.CustomClusterBuilder{
		ObjectMeta: v1.ObjectMeta{
			Name:       customBuilderName,
			Generation: initialGeneration,
		},
		Spec: expv1alpha1.CustomClusterBuilderSpec{
			CustomBuilderSpec: expv1alpha1.CustomBuilderSpec{
				Tag:   customBuilderTag,
				Stack: "some-stack",
				Store: "some-store",
				Order: []expv1alpha1.OrderEntry{
					{
						Group: []expv1alpha1.BuildpackRef{
							{
								BuildpackInfo: expv1alpha1.BuildpackInfo{
									Id:      "buildpack.id.1",
									Version: "1.0.0",
								},
								Optional: false,
							},
							{
								BuildpackInfo: expv1alpha1.BuildpackInfo{
									Id:      "buildpack.id.2",
									Version: "2.0.0",
								},
								Optional: false,
							},
						},
					},
				},
			},
			ServiceAccountRef: corev1.ObjectReference{
				Namespace: "some-sa-namespace",
				Name:      "some-sa-name",
			},
		},
	}

	secretRef := registry.SecretRef{
		ServiceAccount: customBuilder.Spec.ServiceAccountRef.Name,
		Namespace:      customBuilder.Spec.ServiceAccountRef.Namespace,
	}

	when("#Reconcile", func() {
		it.Before(func() {
			keychainFactory.AddKeychainForSecretRef(t, secretRef, &registryfakes.FakeKeychain{})
		})

		it("saves metadata to the status", func() {
			builderCreator.Record = v1alpha1.BuilderRecord{
				Image: customBuilderIdentifier,
				Stack: v1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				Buildpacks: v1alpha1.BuildpackMetadataList{
					{
						Key:     "buildpack.id.1",
						Version: "1.0.0",
					},
					{
						Key:     "buildpack.id.2",
						Version: "2.0.0",
					},
				},
			}

			expectedBuilder := &expv1alpha1.CustomClusterBuilder{
				ObjectMeta: customBuilder.ObjectMeta,
				Spec:       customBuilder.Spec,
				Status: expv1alpha1.CustomBuilderStatus{
					BuilderStatus: v1alpha1.BuilderStatus{
						Status: kpackcore.Status{
							ObservedGeneration: 1,
							Conditions: kpackcore.Conditions{
								{
									Type:   kpackcore.ConditionReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
						BuilderMetadata: []v1alpha1.BuildpackMetadata{
							{
								Key:     "buildpack.id.1",
								Version: "1.0.0",
							},
							{
								Key:     "buildpack.id.2",
								Version: "2.0.0",
							},
						},
						Stack: v1alpha1.BuildStack{
							RunImage: "example.com/run-image@sha256:123456",
							ID:       "fake.stack.id",
						},
						LatestImage: customBuilderIdentifier,
					},
				},
			}

			rt.Test(rtesting.TableRow{
				Key: customBuilderKey,
				Objects: []runtime.Object{
					stack,
					store,
					customBuilder,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: expectedBuilder,
					},
				},
			})

			assert.Equal(t, []testhelpers.CreateBuilderArgs{{
				Keychain:            &registryfakes.FakeKeychain{},
				BuildpackRepository: testhelpers.FakeBuildpackRepository{Store: store},
				CustomBuilderSpec:   customBuilder.Spec.CustomBuilderSpec,
			}}, builderCreator.CreateBuilderCalls)
		})

		it("tracks the stack and store for a custom builder", func() {
			builderCreator.Record = v1alpha1.BuilderRecord{
				Image: customBuilderIdentifier,
				Stack: v1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				Buildpacks: v1alpha1.BuildpackMetadataList{},
			}

			expectedBuilder := &expv1alpha1.CustomClusterBuilder{
				ObjectMeta: customBuilder.ObjectMeta,
				Spec:       customBuilder.Spec,
				Status: expv1alpha1.CustomBuilderStatus{
					BuilderStatus: v1alpha1.BuilderStatus{
						Status: kpackcore.Status{
							ObservedGeneration: 1,
							Conditions: kpackcore.Conditions{
								{
									Type:   kpackcore.ConditionReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
						BuilderMetadata: []v1alpha1.BuildpackMetadata{},
						Stack: v1alpha1.BuildStack{
							RunImage: "example.com/run-image@sha256:123456",
							ID:       "fake.stack.id",
						},
						LatestImage: customBuilderIdentifier,
					},
				},
			}

			rt.Test(rtesting.TableRow{
				Key: customBuilderKey,
				Objects: []runtime.Object{
					stack,
					store,
					expectedBuilder,
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(store, expectedBuilder.NamespacedName()))
			require.True(t, fakeTracker.IsTracking(stack, customBuilder.NamespacedName()))
		})

		it("does not update the status with no status change", func() {
			builderCreator.Record = v1alpha1.BuilderRecord{
				Image: customBuilderIdentifier,
				Stack: v1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				Buildpacks: v1alpha1.BuildpackMetadataList{
					{
						Key:     "buildpack.id.1",
						Version: "1.0.0",
					},
				},
			}

			customBuilder.Status.BuilderStatus = v1alpha1.BuilderStatus{
				Status: kpackcore.Status{
					ObservedGeneration: customBuilder.Generation,
					Conditions: kpackcore.Conditions{
						{
							Type:   kpackcore.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				BuilderMetadata: []v1alpha1.BuildpackMetadata{
					{
						Key:     "buildpack.id.1",
						Version: "1.0.0",
					},
				},
				Stack: v1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				LatestImage: customBuilderIdentifier,
			}

			rt.Test(rtesting.TableRow{
				Key: customBuilderKey,
				Objects: []runtime.Object{
					stack,
					store,
					customBuilder,
				},
				WantErr: false,
			})
		})

		it("updates status on creation error", func() {
			builderCreator.CreateErr = errors.New("create error")

			expectedBuilder := &expv1alpha1.CustomClusterBuilder{
				ObjectMeta: customBuilder.ObjectMeta,
				Spec:       customBuilder.Spec,
				Status: expv1alpha1.CustomBuilderStatus{
					BuilderStatus: v1alpha1.BuilderStatus{
						Status: kpackcore.Status{
							ObservedGeneration: 1,
							Conditions: kpackcore.Conditions{
								{
									Type:    kpackcore.ConditionReady,
									Status:  corev1.ConditionFalse,
									Message: "create error",
								},
							},
						},
					},
				},
			}

			rt.Test(rtesting.TableRow{
				Key: customBuilderKey,
				Objects: []runtime.Object{
					stack,
					store,
					customBuilder,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: expectedBuilder,
					},
				},
			})
		})
	})
}
