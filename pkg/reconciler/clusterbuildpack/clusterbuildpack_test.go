package clusterbuildpack_test

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

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	kreconciler "github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/reconciler/clusterbuildpack"
	"github.com/pivotal/kpack/pkg/reconciler/clusterbuildpack/clusterbuildpackfakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestClusterBuildpackReconciler(t *testing.T) {
	spec.Run(t, "ClusterBuildpack Reconciler", testClusterBuildpackReconciler)
}

func testClusterBuildpackReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		clusterBpName           = "some-cluster-buildpack"
		clusterBpKey            = clusterBpName
		initialGeneration int64 = 1
	)
	var (
		fakeStoreReader     = &clusterbuildpackfakes.FakeStoreReader{}
		fakeKeyChainFactory = &registryfakes.FakeKeychainFactory{}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(_ *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)

			r := &clusterbuildpack.Reconciler{
				Client:                 fakeClient,
				StoreReader:            fakeStoreReader,
				ClusterBuildpackLister: listers.GetClusterBuildpackLister(),
				KeychainFactory:        fakeKeyChainFactory,
			}
			return &kreconciler.NetworkErrorReconciler{Reconciler: r}, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	cbp := &buildapi.ClusterBuildpack{
		ObjectMeta: metav1.ObjectMeta{
			Name:       clusterBpName,
			Generation: initialGeneration,
		},
		Spec: buildapi.ClusterBuildpackSpec{
			Source: corev1alpha1.ImageSource{
				Image: "some.registry/some-image-2",
			},
		},
	}

	when("#Reconcile", func() {
		readBuildpacks := []corev1alpha1.BuildpackStatus{
			{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "paketo-buildpacks/node-engine",
					Version: "0.0.116",
				},
				DiffId: "sha256:d57937f5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf4",
				StoreImage: corev1alpha1.ImageSource{
					Image: "some.registry/some-image-1",
				},
				Order: nil,
			},
			{
				BuildpackInfo: corev1alpha1.BuildpackInfo{
					Id:      "paketo-buildpacks/npm",
					Version: "0.0.71",
				},
				DiffId: "sha256:c67840e5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf5",
				StoreImage: corev1alpha1.ImageSource{
					Image: "some.registry/some-image-2",
				},
				Order: nil,
			},
		}

		it("saves metadata to the status", func() {
			fakeStoreReader.ReadReturns(readBuildpacks, nil)

			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterBpKey,
				Objects: []runtime.Object{
					cbp,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterBuildpack{
							ObjectMeta: cbp.ObjectMeta,
							Spec:       cbp.Spec,
							Status: buildapi.ClusterBuildpackStatus{
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

			_, clusterStoreSpec := fakeStoreReader.ReadArgsForCall(0)
			assert.Equal(t, []corev1alpha1.ImageSource{cbp.Spec.Source}, clusterStoreSpec)
		})

		it("uses the keychain of the referenced service account", func() {
			fakeStoreReader.ReadReturns(readBuildpacks, nil)

			cbp.Spec.ServiceAccountRef = &corev1.ObjectReference{Name: "private-account", Namespace: "my-namespace"}
			secretRef := registry.SecretRef{
				ServiceAccount: "private-account",
				Namespace:      "my-namespace",
			}
			expectedKeyChain := &registryfakes.FakeKeychain{Name: "secret"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, secretRef, expectedKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterBpKey,
				Objects: []runtime.Object{
					cbp,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterBuildpack{
							ObjectMeta: cbp.ObjectMeta,
							Spec:       cbp.Spec,
							Status: buildapi.ClusterBuildpackStatus{
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
			actualKeyChain, _ := fakeStoreReader.ReadArgsForCall(0)
			assert.Equal(t, expectedKeyChain, actualKeyChain)
		})

		it("does not update the status with no status change", func() {
			fakeStoreReader.ReadReturns(readBuildpacks, nil)

			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			cbp.Status = buildapi.ClusterBuildpackStatus{
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
				Key: clusterBpKey,
				Objects: []runtime.Object{
					cbp,
				},
				WantErr: false,
			})
		})

		it("sets the status to Ready False if error reading buildpacks", func() {
			fakeStoreReader.ReadReturns(nil, fmt.Errorf("no buildpacks left"))

			emptySecretRef := registry.SecretRef{}
			defaultKeyChain := &registryfakes.FakeKeychain{Name: "default"}
			fakeKeyChainFactory.AddKeychainForSecretRef(t, emptySecretRef, defaultKeyChain)

			rt.Test(rtesting.TableRow{
				Key: clusterBpKey,
				Objects: []runtime.Object{
					cbp,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.ClusterBuildpack{
							ObjectMeta: cbp.ObjectMeta,
							Spec:       cbp.Spec,
							Status: buildapi.ClusterBuildpackStatus{
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
