package clusterstack_test

import (
	"errors"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	kreconciler "github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstack"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstack/clusterstackfakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestClusterStackReconciler(t *testing.T) {
	spec.Run(t, "Stack Reconciler", testClusterStackReconciler)
}

func testClusterStackReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		clusterStackName        = "some-clusterStack"
		clusterStackKey         = clusterStackName
		initialGeneration int64 = 1
	)

	var (
		fakeKeyChainFactory = &registryfakes.FakeKeychainFactory{}
	)

	fakeClusterStackReader := &clusterstackfakes.FakeClusterStackReader{}

	testClusterStack := &buildapi.ClusterStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:       clusterStackName,
			Generation: initialGeneration,
		},
		Spec: buildapi.ClusterStackSpec{
			Id: "some.clusterStack.id",
			BuildImage: buildapi.ClusterStackSpecImage{
				Image: "some-registry.io/build-image",
			},
			RunImage: buildapi.ClusterStackSpecImage{
				Image: "some-registry.io/run-image",
			},
		},
	}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &clusterstack.Reconciler{
				Client:             fakeClient,
				ClusterStackLister: listers.GetClusterStackLister(),
				ClusterStackReader: fakeClusterStackReader,
				KeychainFactory:    fakeKeyChainFactory,
			}
			return &kreconciler.NetworkErrorReconciler{Reconciler: r}, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	when("#Reconcile", func() {
		it("saves metadata to the status", func() {
			resolvedClusterStack := buildapi.ResolvedClusterStack{
				BuildImage: buildapi.ClusterStackStatusImage{
					LatestImage: "some-registry.io/build-image@sha245:123",
				},
				RunImage: buildapi.ClusterStackStatusImage{
					LatestImage: "some-registry.io/run-image@sha245:123",
				},
				Mixins:  []string{"a-nice-mixin"},
				UserID:  1000,
				GroupID: 2000,
			}
			fakeClusterStackReader.ReadReturns(resolvedClusterStack, nil)
			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterStackKey,
				Objects: []runtime.Object{
					testClusterStack,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterStack{
							ObjectMeta: testClusterStack.ObjectMeta,
							Spec:       testClusterStack.Spec,
							Status: buildapi.ClusterStackStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								ResolvedClusterStack: resolvedClusterStack,
							},
						},
					},
				},
			})

			require.Equal(t, 1, fakeClusterStackReader.ReadCallCount())
			_, clusterStackSpec := fakeClusterStackReader.ReadArgsForCall(0)
			require.Equal(t, testClusterStack.Spec, clusterStackSpec)
		})

		it("does not update the status with no status change", func() {
			resolvedClusterStack := buildapi.ResolvedClusterStack{
				BuildImage: buildapi.ClusterStackStatusImage{
					LatestImage: "some-registry.io/build-image@sha245:123",
				},
				RunImage: buildapi.ClusterStackStatusImage{
					LatestImage: "some-registry.io/run-image@sha245:123",
				},
				Mixins:  []string{"a-nice-mixin"},
				UserID:  1000,
				GroupID: 2000,
			}
			fakeClusterStackReader.ReadReturns(resolvedClusterStack, nil)
			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			testClusterStack.Status = buildapi.ClusterStackStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				ResolvedClusterStack: resolvedClusterStack,
			}
			rt.Test(rtesting.TableRow{
				Key: clusterStackKey,
				Objects: []runtime.Object{
					testClusterStack,
				},
				WantErr: false,
			})
		})

		it("sets the status to Ready False if error reading from clusterStack", func() {
			fakeClusterStackReader.ReadReturns(buildapi.ResolvedClusterStack{}, errors.New("invalid mixins on run image"))
			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterStackKey,
				Objects: []runtime.Object{
					testClusterStack,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterStack{
							ObjectMeta: testClusterStack.ObjectMeta,
							Spec:       testClusterStack.Spec,
							Status: buildapi.ClusterStackStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Message: "invalid mixins on run image",
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

		it("uses the keychain of the referenced service account", func() {
			fakeClusterStackReader.ReadReturns(buildapi.ResolvedClusterStack{}, nil)

			testClusterStack.Spec.ServiceAccountRef = &corev1.ObjectReference{Name: "private-account", Namespace: "my-namespace"}
			secretRef := registry.SecretRef{
				ServiceAccount: "private-account",
				Namespace:      "my-namespace",
			}
			expectedKeyChain := &registryfakes.FakeKeychain{Name: "secret"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, secretRef, expectedKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterStackKey,
				Objects: []runtime.Object{
					testClusterStack,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterStack{
							ObjectMeta: testClusterStack.ObjectMeta,
							Spec:       testClusterStack.Spec,
							Status: buildapi.ClusterStackStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
							},
						},
					},
				},
			})

			assert.Equal(t, 1, fakeClusterStackReader.ReadCallCount())
			actualKeyChain, _ := fakeClusterStackReader.ReadArgsForCall(0)
			assert.Equal(t, expectedKeyChain, actualKeyChain)
		})

	})
}
