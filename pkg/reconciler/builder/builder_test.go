package builder_test

import (
	"errors"
	"testing"

	"github.com/pivotal/kpack/pkg/secret/secretfakes"

	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/cnb"
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
		testNamespace             = "some-namespace"
		builderName               = "custom-builder"
		builderKey                = testNamespace + "/" + builderName
		builderTag                = "example.com/custom-builder"
		expectedResolvedTag       = "example.com/custom-builder:builder-some-namespace-custom-builder"
		builderIdentifier         = "example.com/custom-builder@sha256:resolved-builder-digest"
		initialGeneration   int64 = 1
	)

	var (
		builderCreator    = &testhelpers.FakeBuilderCreator{}
		keychainFactory   = &registryfakes.FakeKeychainFactory{}
		fakeTracker       = &testhelpers.FakeTracker{}
		fakeSecretFetcher = &secretfakes.FakeFetchSecret{
			FakeSecrets: []*corev1.Secret{},
		}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)
			r := &builder.Reconciler{
				Client:                 fakeClient,
				BuilderLister:          listers.GetBuilderLister(),
				BuilderCreator:         builderCreator,
				KeychainFactory:        keychainFactory,
				Tracker:                fakeTracker,
				ClusterStoreLister:     listers.GetClusterStoreLister(),
				BuildpackLister:        listers.GetBuildpackLister(),
				ClusterBuildpackLister: listers.GetClusterBuildpackLister(),
				ClusterStackLister:     listers.GetClusterStackLister(),
				ClusterLifecycleLister: listers.GetClusterLifecycleLister(),
				SecretFetcher:          fakeSecretFetcher,
			}
			return &kreconciler.NetworkErrorReconciler{Reconciler: r}, rtesting.ActionRecorderList{fakeClient, k8sfakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	signingSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-secret-name",
			Namespace: testNamespace,
		},
	}

	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-sa-name",
			Namespace: signingSecret.Namespace,
		},
		Secrets: []corev1.ObjectReference{
			{
				Name: signingSecret.Name,
			},
		},
	}

	clusterStore := &buildapi.ClusterStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-store",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterStore",
			APIVersion: "kpack.io/v1alpha2",
		},
		Spec:   buildapi.ClusterStoreSpec{},
		Status: buildapi.ClusterStoreStatus{},
	}

	clusterStack := &buildapi.ClusterStack{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-stack",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterStack",
			APIVersion: "kpack.io/v1alpha2",
		},
		Spec: buildapi.ClusterStackSpec{
			ServiceAccountRef: &corev1.ObjectReference{
				Name:      "some-service-account",
				Namespace: testNamespace,
			},
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

	clusterLifecycle := &buildapi.ClusterLifecycle{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-lifecycle",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterLifecycle",
			APIVersion: "kpack.io/v1alpha2",
		},
		Spec: buildapi.ClusterLifecycleSpec{
			ServiceAccountRef: &corev1.ObjectReference{
				Name:      "some-service-account",
				Namespace: testNamespace,
			},
		},
		Status: buildapi.ClusterLifecycleStatus{
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

	buildpack := &buildapi.Buildpack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buildpack.id.3",
			Namespace: testNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Buildpack",
			APIVersion: "kpack.io/v1alpha2",
		},
	}

	clusterBuildpack := &buildapi.ClusterBuildpack{
		ObjectMeta: metav1.ObjectMeta{
			Name: "buildpack.id.4",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "ClusterBuildpack",
			APIVersion: "kpack.io/v1alpha2",
		},
	}

	builder := &buildapi.Builder{
		ObjectMeta: metav1.ObjectMeta{
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
				Lifecycle: corev1.ObjectReference{
					Kind: "Lifecycle",
					Name: "some-lifecycle",
				},
				Store: corev1.ObjectReference{
					Kind: "ClusterStore",
					Name: "some-store",
				},
				Order: []buildapi.BuilderOrderEntry{{
					Group: []buildapi.BuilderBuildpackRef{
						{
							BuildpackRef: corev1alpha1.BuildpackRef{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "buildpack.id.1",
									Version: "1.0.0",
								},
								Optional: false,
							},
						},
						{
							BuildpackRef: corev1alpha1.BuildpackRef{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id:      "buildpack.id.2",
									Version: "2.0.0",
								},
								Optional: false,
							},
						},
					},
				}},
			},
			ServiceAccountName: serviceAccount.Name,
		},
	}

	secretRef := registry.SecretRef{
		ServiceAccount: serviceAccount.Name,
		Namespace:      serviceAccount.Namespace,
	}

	saSecretRef := registry.SecretRef{
		ServiceAccount: "some-service-account",
		Namespace:      testNamespace,
	}

	when("#Reconcile", func() {
		it.Before(func() {
			keychainFactory.AddKeychainForSecretRef(t, secretRef, &registryfakes.FakeKeychain{})
			keychainFactory.AddKeychainForSecretRef(t, saSecretRef, &registryfakes.FakeKeychain{})
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
							{
								Type:   buildapi.ConditionUpToDate,
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

			expectedFetcher := cnb.NewRemoteBuildpackFetcher(keychainFactory, clusterStore, []*buildapi.Buildpack{buildpack}, []*buildapi.ClusterBuildpack{clusterBuildpack})

			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterLifecycle,
					clusterStack,
					clusterStore,
					builder,
					buildpack,
					clusterBuildpack,
					&signingSecret,
					&serviceAccount,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: expectedBuilder,
					},
				},
			})

			assert.Equal(t, []testhelpers.CreateBuilderArgs{{
				Context:            context.Background(),
				BuilderKeychain:    &registryfakes.FakeKeychain{},
				StackKeychain:      &registryfakes.FakeKeychain{},
				Fetcher:            expectedFetcher,
				ClusterStack:       clusterStack,
				ClusterLifecycle:   clusterLifecycle,
				BuilderSpec:        builder.Spec.BuilderSpec,
				SigningSecrets:     []*corev1.Secret{},
				ResolvedBuilderTag: expectedResolvedTag,
			}}, builderCreator.CreateBuilderCalls)
		})

		it("tracks the store and buildpack sources for a custom builder", func() {
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
							{
								Type:   buildapi.ConditionUpToDate,
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
					clusterLifecycle,
					clusterStack,
					clusterStore,
					buildpack,
					clusterBuildpack,
					expectedBuilder,
					&signingSecret,
					&serviceAccount,
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(
				kreconciler.KeyForObject(clusterStore),
				builder.NamespacedName()))
			require.True(t, fakeTracker.IsTracking(
				kreconciler.KeyForObject(clusterStack),
				builder.NamespacedName()))
			require.True(t, fakeTracker.IsTracking(
				kreconciler.KeyForObject(clusterLifecycle),
				builder.NamespacedName()))

			require.True(t, fakeTracker.IsTrackingKind(
				kreconciler.KeyForObject(buildpack).GroupKind,
				builder.NamespacedName()))
			require.True(t, fakeTracker.IsTrackingKind(
				kreconciler.KeyForObject(clusterBuildpack).GroupKind,
				builder.NamespacedName()))
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
						{
							Type:   buildapi.ConditionUpToDate,
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
					clusterLifecycle,
					clusterStack,
					clusterStore,
					builder,
					&signingSecret,
					&serviceAccount,
				},
				WantErr: false,
			})
		})

		it("updates status on creation error", func() {
			builderCreator.CreateErr = errors.New("create error")

			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterLifecycle,
					clusterStack,
					clusterStore,
					builder,
					&signingSecret,
					&serviceAccount,
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
											Reason:  buildapi.NoLatestImageReason,
											Message: buildapi.NoLatestImageMessage,
										},
										{
											Type:    buildapi.ConditionUpToDate,
											Status:  corev1.ConditionFalse,
											Reason:  buildapi.ReconcileFailedReason,
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
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-stack",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "ClusterStack",
					APIVersion: "kpack.io/v1alpha2",
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
					&signingSecret,
					&serviceAccount,
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
											Reason:  buildapi.NoLatestImageReason,
											Message: buildapi.NoLatestImageMessage,
										},
										{
											Type:    buildapi.ConditionUpToDate,
											Status:  corev1.ConditionFalse,
											Reason:  buildapi.ReconcileFailedReason,
											Message: "Error: clusterstack 'some-stack' is not ready",
										},
									},
								},
							},
						},
					},
				},
			})

			// still track resources
			require.True(t, fakeTracker.IsTracking(kreconciler.KeyForObject(notReadyClusterStack), builder.NamespacedName()))
			require.Len(t, builderCreator.CreateBuilderCalls, 0)
		})

		it("updates status and doesn't build builder when the store doesn't exist", func() {
			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterStack,
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
											Reason:  buildapi.NoLatestImageReason,
											Message: buildapi.NoLatestImageMessage,
										},
										{
											Type:    buildapi.ConditionUpToDate,
											Status:  corev1.ConditionFalse,
											Reason:  buildapi.ReconcileFailedReason,
											Message: `clusterstore.kpack.io "some-store" not found`,
										},
									},
								},
							},
						},
					},
				},
			})

			// still track resources
			require.True(t, fakeTracker.IsTracking(kreconciler.KeyForObject(clusterStack), builder.NamespacedName()))
			require.Len(t, builderCreator.CreateBuilderCalls, 0)
		})

		it("updates status and doesn't build builder when the stack doesn't exist", func() {
			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
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
											Reason:  buildapi.NoLatestImageReason,
											Message: buildapi.NoLatestImageMessage,
										},
										{
											Type:    buildapi.ConditionUpToDate,
											Status:  corev1.ConditionFalse,
											Reason:  buildapi.ReconcileFailedReason,
											Message: `clusterstack.kpack.io "some-stack" not found`,
										},
									},
								},
							},
						},
					},
				},
			})

			// still track resources
			require.True(t, fakeTracker.IsTracking(kreconciler.KeyForObject(clusterStack), builder.NamespacedName()))
			require.Len(t, builderCreator.CreateBuilderCalls, 0)
		})

		it("adds a tag when one doesn't exist", func() {
			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterLifecycle,
					clusterStack,
					clusterStore,
					builder,
					clusterBuildpack,
					&signingSecret,
					&serviceAccount,
				},
				WantErr: false,
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
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
										{
											Type:   buildapi.ConditionUpToDate,
											Status: corev1.ConditionTrue,
										},
									},
								},
							},
						},
					},
				},
			})

			assert.Equal(t, expectedResolvedTag, builderCreator.CreateBuilderCalls[0].ResolvedBuilderTag)
		})

		it("uses existing tag if provided", func() {

			builder.Spec.Tag = "example.com/custom-builder:my-tag"
			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterLifecycle,
					clusterStack,
					clusterStore,
					builder,
					clusterBuildpack,
					&signingSecret,
					&serviceAccount,
				},
				WantErr: false,
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
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
										{
											Type:   buildapi.ConditionUpToDate,
											Status: corev1.ConditionTrue,
										},
									},
								},
							},
						},
					},
				},
			})

			assert.Equal(t, "example.com/custom-builder:my-tag", builderCreator.CreateBuilderCalls[0].ResolvedBuilderTag)
		})

		it("fails if spec.tag is not a valid image ref", func() {
			builder.Spec.Tag = "example.com/invalid::builder"
			rt.Test(rtesting.TableRow{
				Key: builderKey,
				Objects: []runtime.Object{
					clusterLifecycle,
					clusterStack,
					clusterStore,
					builder,
					clusterBuildpack,
					&signingSecret,
					&serviceAccount,
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
											Reason:  buildapi.NoLatestImageReason,
											Message: buildapi.NoLatestImageMessage,
										},
										{
											Type:    buildapi.ConditionUpToDate,
											Status:  corev1.ConditionFalse,
											Reason:  buildapi.ReconcileFailedReason,
											Message: "could not parse reference: example.com/invalid::builder",
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
