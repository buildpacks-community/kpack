package builder_test

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

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	kreconciler "github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/reconciler/builder"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestBuilderReconciler(t *testing.T) {
	spec.Run(t, "Custom Builder Reconciler", testBuilderReconciler)
}

func testBuilderReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		testNamespace           = "some-namespace"
		builderName             = "custom-builder"
		builderKey              = testNamespace + "/" + builderName
		builderTag              = "example.com/custom-builder"
		builderIdentifier       = "example.com/custom-builder@sha256:resolved-builder-digest"
		initialGeneration int64 = 1
	)

	var (
		builderCreator  = &testhelpers.FakeBuilderCreator{}
		keychainFactory = &registryfakes.FakeKeychainFactory{}
		fakeTracker     = testhelpers.FakeTracker{}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &builder.Reconciler{
				Client:             fakeClient,
				BuilderLister:      listers.GetBuilderLister(),
				BuilderCreator:     builderCreator,
				KeychainFactory:    keychainFactory,
				Tracker:            fakeTracker,
				ClusterStoreLister: listers.GetClusterStoreLister(),
				ClusterStackLister: listers.GetClusterStackLister(),
			}
			return &kreconciler.NetworkErrorReconciler{Reconciler: r}, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	clusterStore := &buildapi.ClusterStore{
		ObjectMeta: v1.ObjectMeta{
			Name: "some-store",
		},
		Spec:   buildapi.ClusterStoreSpec{},
		Status: buildapi.ClusterStoreStatus{},
	}

	clusterStack := &buildapi.ClusterStack{
		ObjectMeta: v1.ObjectMeta{
			Name: "some-stack",
		},
		Status: buildapi.ClusterStackStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: 0,
				Conditions: []corev1alpha1.Condition{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	builder := &buildapi.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name:       builderName,
			Generation: initialGeneration,
			Namespace:  testNamespace,
		},
		Spec: buildapi.NamespacedBuilderSpec{
			BuilderSpec: buildapi.BuilderSpec{
				Tag: builderTag,
				Stack: corev1.ObjectReference{
					Kind: "Stack",
					Name: "some-stack",
				},
				Store: corev1.ObjectReference{
					Kind: "ClusterStore",
					Name: "some-store",
				},
				Order: []corev1alpha1.OrderEntry{
					{
						Group: []corev1alpha1.BuildpackRef{
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "buildpack.id.1",
									Version: "1.0.0",
								},
								Optional: false,
							},
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "buildpack.id.2",
									Version: "2.0.0",
								},
								Optional: false,
							},
						},
					},
				},
			},
			ServiceAccountName: "some-service-account",
		},
	}

	secretRef := registry.SecretRef{
		ServiceAccount: builder.Spec.ServiceAccount(),
		Namespace:      builder.Namespace,
	}

	when("#Reconcile", func() {
		it.Before(func() {
			keychainFactory.AddKeychainForSecretRef(t, secretRef, &registryfakes.FakeKeychain{})
		})

		it("saves metadata to the status", func() {
			builderCreator.Record = buildapi.BuilderRecord{
				Image: builderIdentifier,
				Stack: corev1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				Buildpacks: corev1alpha1.BuildpackMetadataList{
					{
						Id:      "buildpack.id.1",
						Version: "1.0.0",
					},
					{
						Id:      "buildpack.id.2",
						Version: "2.0.0",
					},
				},
				ObservedStoreGeneration: 10,
				ObservedStackGeneration: 11,
			}

			expectedBuilder := &buildapi.Builder{
				ObjectMeta: builder.ObjectMeta,
				Spec:       builder.Spec,
				Status: buildapi.BuilderStatus{
					Status: corev1alpha1.Status{
						ObservedGeneration: 1,
						Conditions: corev1alpha1.Conditions{
							{
								Type:   corev1alpha1.ConditionReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
					BuilderMetadata: []corev1alpha1.BuildpackMetadata{
						{
							Id:      "buildpack.id.1",
							Version: "1.0.0",
						},
						{
							Id:      "buildpack.id.2",
							Version: "2.0.0",
						},
					},
					Stack: corev1alpha1.BuildStack{
						RunImage: "example.com/run-image@sha256:123456",
						ID:       "fake.stack.id",
					},
					LatestImage:             builderIdentifier,
					ObservedStoreGeneration: 10,
					ObservedStackGeneration: 11,
				},
			}

			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterStack,
					clusterStore,
					builder,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: expectedBuilder,
					},
				},
			})

			assert.Equal(t, []testhelpers.CreateBuilderArgs{{
				Keychain:     &registryfakes.FakeKeychain{},
				ClusterStore: clusterStore,
				ClusterStack: clusterStack,
				BuilderSpec:  builder.Spec.BuilderSpec,
			}}, builderCreator.CreateBuilderCalls)
		})

		it("tracks the stack and store for a custom builder", func() {
			builderCreator.Record = buildapi.BuilderRecord{
				Image: builderIdentifier,
				Stack: corev1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				Buildpacks: corev1alpha1.BuildpackMetadataList{},
			}

			expectedBuilder := &buildapi.Builder{
				ObjectMeta: builder.ObjectMeta,
				Spec:       builder.Spec,
				Status: buildapi.BuilderStatus{
					Status: corev1alpha1.Status{
						ObservedGeneration: 1,
						Conditions: corev1alpha1.Conditions{
							{
								Type:   corev1alpha1.ConditionReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
					BuilderMetadata: []corev1alpha1.BuildpackMetadata{},
					Stack: corev1alpha1.BuildStack{
						RunImage: "example.com/run-image@sha256:123456",
						ID:       "fake.stack.id",
					},
					LatestImage: builderIdentifier,
				},
			}

			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterStack,
					clusterStore,
					expectedBuilder,
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(clusterStore, builder.NamespacedName()))
			require.True(t, fakeTracker.IsTracking(clusterStack, builder.NamespacedName()))
		})

		it("does not update the status with no status change", func() {
			builderCreator.Record = buildapi.BuilderRecord{
				Image: builderIdentifier,
				Stack: corev1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				Buildpacks: corev1alpha1.BuildpackMetadataList{
					{
						Id:      "buildpack.id.1",
						Version: "1.0.0",
					},
				},
			}

			builder.Status = buildapi.BuilderStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: builder.Generation,
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				BuilderMetadata: []corev1alpha1.BuildpackMetadata{
					{
						Id:      "buildpack.id.1",
						Version: "1.0.0",
					},
				},
				Stack: corev1alpha1.BuildStack{
					RunImage: "example.com/run-image@sha256:123456",
					ID:       "fake.stack.id",
				},
				LatestImage: builderIdentifier,
			}

			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterStack,
					clusterStore,
					builder,
				},
				WantErr: false,
			})
		})

		it("updates status on creation error", func() {
			builderCreator.CreateErr = errors.New("create error")

			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterStack,
					clusterStore,
					builder,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Builder{
							ObjectMeta: builder.ObjectMeta,
							Spec:       builder.Spec,
							Status: buildapi.BuilderStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:    corev1alpha1.ConditionReady,
											Status:  corev1.ConditionFalse,
											Message: "create error",
										},
									},
								},
							},
						},
					},
				},
			})

		})

		it("updates status and doesn't build builder when stack not ready", func() {
			notReadyClusterStack := &buildapi.ClusterStack{
				ObjectMeta: v1.ObjectMeta{
					Name: "some-stack",
				},
				Status: buildapi.ClusterStackStatus{
					Status: corev1alpha1.Status{
						ObservedGeneration: 0,
						Conditions: []corev1alpha1.Condition{
							{
								Type:   corev1alpha1.ConditionReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				},
			}
			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					notReadyClusterStack,
					clusterStore,
					builder,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Builder{
							ObjectMeta: builder.ObjectMeta,
							Spec:       builder.Spec,
							Status: buildapi.BuilderStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:    corev1alpha1.ConditionReady,
											Status:  corev1.ConditionFalse,
											Message: "stack some-stack is not ready",
										},
									},
								},
							},
						},
					},
				},
			})

			//still track resources
			require.True(t, fakeTracker.IsTracking(clusterStore, builder.NamespacedName()))
			require.True(t, fakeTracker.IsTracking(notReadyClusterStack, builder.NamespacedName()))
			require.Len(t, builderCreator.CreateBuilderCalls, 0)
		})

	})
}
