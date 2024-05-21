package clusterlifecycle_test

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
	"github.com/pivotal/kpack/pkg/reconciler/clusterlifecycle"
	"github.com/pivotal/kpack/pkg/reconciler/clusterlifecycle/clusterlifecyclefakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestClusterLifecycleReconciler(t *testing.T) {
	spec.Run(t, "Lifecycle Reconciler", testClusterLifecycleReconciler)
}

func testClusterLifecycleReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		clusterLifecycleName       = "some-clusterLifecycle"
		clusterLifecycleKey        = clusterLifecycleName
		initialGeneration    int64 = 1
	)

	var (
		fakeKeyChainFactory = &registryfakes.FakeKeychainFactory{}
	)

	fakeClusterLifecycleReader := &clusterlifecyclefakes.FakeClusterLifecycleReader{}

	testClusterLifecycle := &buildapi.ClusterLifecycle{
		ObjectMeta: metav1.ObjectMeta{
			Name:       clusterLifecycleName,
			Generation: initialGeneration,
		},
		Spec: buildapi.ClusterLifecycleSpec{
			ImageSource: corev1alpha1.ImageSource{
				Image: "some-registry.io/lifecycle-image",
			},
		},
	}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &clusterlifecycle.Reconciler{
				Client:                 fakeClient,
				ClusterLifecycleLister: listers.GetClusterLifecycleLister(),
				ClusterLifecycleReader: fakeClusterLifecycleReader,
				KeychainFactory:        fakeKeyChainFactory,
			}
			return &kreconciler.NetworkErrorReconciler{Reconciler: r}, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	when("#Reconcile", func() {
		it("saves metadata to the status", func() {
			resolvedClusterLifecycle := buildapi.ResolvedClusterLifecycle{
				Version:       "some-version",
				BuildpackAPIs: []string{"0.7", "0.8", "0.9", "0.10", "0.11"},
				PlatformAPIs:  []string{"0.7", "0.8", "0.9", "0.10", "0.11", "0.12", "0.13"},
			}
			fakeClusterLifecycleReader.ReadReturns(resolvedClusterLifecycle, nil)
			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterLifecycleKey,
				Objects: []runtime.Object{
					testClusterLifecycle,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterLifecycle{
							ObjectMeta: testClusterLifecycle.ObjectMeta,
							Spec:       testClusterLifecycle.Spec,
							Status: buildapi.ClusterLifecycleStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								ResolvedClusterLifecycle: resolvedClusterLifecycle,
							},
						},
					},
				},
			})

			require.Equal(t, 1, fakeClusterLifecycleReader.ReadCallCount())
			_, clusterLifecycleSpec := fakeClusterLifecycleReader.ReadArgsForCall(0)
			require.Equal(t, testClusterLifecycle.Spec, clusterLifecycleSpec)
		})

		it("does not update the status with no status change", func() {
			resolvedClusterLifecycle := buildapi.ResolvedClusterLifecycle{
				Version:       "some-version",
				BuildpackAPIs: []string{"0.7", "0.8", "0.9", "0.10", "0.11"},
				PlatformAPIs:  []string{"0.7", "0.8", "0.9", "0.10", "0.11", "0.12", "0.13"},
			}
			fakeClusterLifecycleReader.ReadReturns(resolvedClusterLifecycle, nil)
			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			testClusterLifecycle.Status = buildapi.ClusterLifecycleStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				ResolvedClusterLifecycle: resolvedClusterLifecycle,
			}
			rt.Test(rtesting.TableRow{
				Key: clusterLifecycleKey,
				Objects: []runtime.Object{
					testClusterLifecycle,
				},
				WantErr: false,
			})
		})

		it("sets the status to Ready False if error reading from clusterLifecycle", func() {
			fakeClusterLifecycleReader.ReadReturns(buildapi.ResolvedClusterLifecycle{}, errors.New("invalid mixins on run image"))
			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterLifecycleKey,
				Objects: []runtime.Object{
					testClusterLifecycle,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterLifecycle{
							ObjectMeta: testClusterLifecycle.ObjectMeta,
							Spec:       testClusterLifecycle.Spec,
							Status: buildapi.ClusterLifecycleStatus{
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
			fakeClusterLifecycleReader.ReadReturns(buildapi.ResolvedClusterLifecycle{}, nil)

			testClusterLifecycle.Spec.ServiceAccountRef = &corev1.ObjectReference{Name: "private-account", Namespace: "my-namespace"}
			secretRef := registry.SecretRef{
				ServiceAccount: "private-account",
				Namespace:      "my-namespace",
			}
			expectedKeyChain := &registryfakes.FakeKeychain{Name: "secret"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, secretRef, expectedKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterLifecycleKey,
				Objects: []runtime.Object{
					testClusterLifecycle,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterLifecycle{
							ObjectMeta: testClusterLifecycle.ObjectMeta,
							Spec:       testClusterLifecycle.Spec,
							Status: buildapi.ClusterLifecycleStatus{
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

			assert.Equal(t, 1, fakeClusterLifecycleReader.ReadCallCount())
			actualKeyChain, _ := fakeClusterLifecycleReader.ReadArgsForCall(0)
			assert.Equal(t, expectedKeyChain, actualKeyChain)
		})

	})
}
