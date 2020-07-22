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
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler/customclusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
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
		fakeRepoFactory = func(clusterStore *v1alpha1.ClusterStore) cnb.BuildpackRepository {
			return testhelpers.FakeBuildpackRepository{ClusterStore: clusterStore}
		}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &customclusterbuilder.Reconciler{
				Client:                     fakeClient,
				CustomClusterBuilderLister: listers.GetCustomClusterBuilderLister(),
				RepoFactory:                fakeRepoFactory,
				BuilderCreator:             builderCreator,
				KeychainFactory:            keychainFactory,
				Tracker:                    fakeTracker,
				ClusterStoreLister:         listers.GetClusterStoreLister(),
				ClusterStackLister:         listers.GetClusterStackLister(),
			}
			return r, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	clusterStore := &v1alpha1.ClusterStore{
		ObjectMeta: v1.ObjectMeta{
			Name: "some-store",
		},
		Spec:   v1alpha1.ClusterStoreSpec{},
		Status: v1alpha1.ClusterStoreStatus{},
	}

	clusterStack := &v1alpha1.ClusterStack{
		ObjectMeta: v1.ObjectMeta{
			Name: "some-stack",
		},
		Status: v1alpha1.ClusterStackStatus{
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

	customBuilder := &v1alpha1.CustomClusterBuilder{
		ObjectMeta: v1.ObjectMeta{
			Name:       customBuilderName,
			Generation: initialGeneration,
		},
		Spec: v1alpha1.CustomClusterBuilderSpec{
			CustomBuilderSpec: v1alpha1.CustomBuilderSpec{
				Tag: customBuilderTag,
				Stack: corev1.ObjectReference{
					Kind: "Stack",
					Name: "some-stack",
				},
				Store: corev1.ObjectReference{
					Kind: "ClusterStore",
					Name: "some-store",
				},
				Order: []v1alpha1.OrderEntry{
					{
						Group: []v1alpha1.BuildpackRef{
							{
								BuildpackInfo: v1alpha1.BuildpackInfo{
									Id:      "buildpack.id.1",
									Version: "1.0.0",
								},
								Optional: false,
							},
							{
								BuildpackInfo: v1alpha1.BuildpackInfo{
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
						Id:      "buildpack.id.1",
						Version: "1.0.0",
					},
					{
						Id:      "buildpack.id.2",
						Version: "2.0.0",
					},
				},
			}

			expectedBuilder := &v1alpha1.CustomClusterBuilder{
				ObjectMeta: customBuilder.ObjectMeta,
				Spec:       customBuilder.Spec,
				Status: v1alpha1.CustomBuilderStatus{
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
							Id:      "buildpack.id.1",
							Version: "1.0.0",
						},
						{
							Id:      "buildpack.id.2",
							Version: "2.0.0",
						},
					},
					Stack: v1alpha1.BuildStack{
						RunImage: "example.com/run-image@sha256:123456",
						ID:       "fake.stack.id",
					},
					LatestImage: customBuilderIdentifier,
				},
			}

			rt.Test(rtesting.TableRow{
				Key: customBuilderKey,
				Objects: []runtime.Object{
					clusterStack,
					clusterStore,
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
				BuildpackRepository: testhelpers.FakeBuildpackRepository{ClusterStore: clusterStore},
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

			expectedBuilder := &v1alpha1.CustomClusterBuilder{
				ObjectMeta: customBuilder.ObjectMeta,
				Spec:       customBuilder.Spec,
				Status: v1alpha1.CustomBuilderStatus{
					Status: corev1alpha1.Status{
						ObservedGeneration: 1,
						Conditions: corev1alpha1.Conditions{
							{
								Type:   corev1alpha1.ConditionReady,
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
			}

			rt.Test(rtesting.TableRow{
				Key: customBuilderKey,
				Objects: []runtime.Object{
					clusterStack,
					clusterStore,
					expectedBuilder,
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(clusterStore, expectedBuilder.NamespacedName()))
			require.True(t, fakeTracker.IsTracking(clusterStack, customBuilder.NamespacedName()))
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
						Id:      "buildpack.id.1",
						Version: "1.0.0",
					},
				},
			}

			customBuilder.Status = v1alpha1.CustomBuilderStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: customBuilder.Generation,
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				BuilderMetadata: []v1alpha1.BuildpackMetadata{
					{
						Id:      "buildpack.id.1",
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
					clusterStack,
					clusterStore,
					customBuilder,
				},
				WantErr: false,
			})
		})

		it("updates status on creation error", func() {
			builderCreator.CreateErr = errors.New("create error")

			expectedBuilder := &v1alpha1.CustomClusterBuilder{
				ObjectMeta: customBuilder.ObjectMeta,
				Spec:       customBuilder.Spec,
				Status: v1alpha1.CustomBuilderStatus{
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
			}

			rt.Test(rtesting.TableRow{
				Key: customBuilderKey,
				Objects: []runtime.Object{
					clusterStack,
					clusterStore,
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

		it("updates status and doesn't build builder when stack not ready", func() {
			notReadyClusterStack := &v1alpha1.ClusterStack{
				ObjectMeta: v1.ObjectMeta{
					Name: "some-stack",
				},
				Status: v1alpha1.ClusterStackStatus{
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
				Key: customBuilderKey,
				Objects: []runtime.Object{
					notReadyClusterStack,
					clusterStore,
					customBuilder,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &v1alpha1.CustomClusterBuilder{
							ObjectMeta: customBuilder.ObjectMeta,
							Spec:       customBuilder.Spec,
							Status: v1alpha1.CustomBuilderStatus{
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
			require.True(t, fakeTracker.IsTracking(clusterStore, customBuilder.NamespacedName()))
			require.True(t, fakeTracker.IsTracking(notReadyClusterStack, customBuilder.NamespacedName()))
			require.Len(t, builderCreator.CreateBuilderCalls, 0)
		})

	})
}
