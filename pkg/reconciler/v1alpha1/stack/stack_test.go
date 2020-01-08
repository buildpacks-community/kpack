package stack_test

import (
	"errors"
	"testing"

	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/stack"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestStackReconciler(t *testing.T) {
	spec.Run(t, "Stack Reconciler", testStackReconciler)
}

func testStackReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		stackName               = "some-stack"
		stackKey                = stackName
		initialGeneration int64 = 1
	)

	var (
		fakeKeychainFactory = &registryfakes.FakeKeychainFactory{}
		expectedKeychain    = &registryfakes.FakeKeychain{Name: "Expected Keychain"}
		fakeRegistryClient  = registryfakes.NewFakeClient()
		buildImage          ggcrv1.Image
		buildImageSha       string
		runImage            ggcrv1.Image
		runImageSha         string
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &stack.Reconciler{
				ImageFetcher:    fakeRegistryClient,
				Client:          fakeClient,
				KeychainFactory: fakeKeychainFactory,
				StackLister:     listers.GetStackLister(),
			}
			return r, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}, &rtesting.FakeStatsReporter{}
		})

	stack := &expv1alpha1.Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name:       stackName,
			Generation: initialGeneration,
		},
		Spec: expv1alpha1.StackSpec{
			Id: "some.stack.id",
			BuildImage: expv1alpha1.StackSpecImage{
				Image: "some-registry.io/build-image",
			},
			RunImage: expv1alpha1.StackSpecImage{
				Image: "some-registry.io/run-image",
			},
		},
	}

	it.Before(func() {
		fakeKeychainFactory.AddKeychainForSecretRef(t, registry.SecretRef{}, expectedKeychain)

		buildImage, buildImageSha = randomImage(t)
		fakeRegistryClient.AddImage("some-registry.io/build-image", buildImage, expectedKeychain)

		runImage, runImageSha = randomImage(t)
		fakeRegistryClient.AddImage("some-registry.io/run-image", runImage, expectedKeychain)
	})

	when("#Reconcile", func() {
		it("saves metadata to the status", func() {
			rt.Test(rtesting.TableRow{
				Key: stackKey,
				Objects: []runtime.Object{
					stack,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &expv1alpha1.Stack{
							ObjectMeta: stack.ObjectMeta,
							Spec:       stack.Spec,
							Status: expv1alpha1.StackStatus{
								Status: v1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: v1alpha1.Conditions{
										{
											Type:   v1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								BuildImage: expv1alpha1.StackStatusImage{LatestImage: "some-registry.io/build-image@" + buildImageSha},
								RunImage:   expv1alpha1.StackStatusImage{LatestImage: "some-registry.io/run-image@" + runImageSha},
							},
						},
					},
				},
			})
		})

		it("does not update the status with no status change", func() {
			stack.Status = expv1alpha1.StackStatus{
				Status: v1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: v1alpha1.Conditions{
						{
							Type:   v1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				BuildImage: expv1alpha1.StackStatusImage{LatestImage: "some-registry.io/build-image@" + buildImageSha},
				RunImage:   expv1alpha1.StackStatusImage{LatestImage: "some-registry.io/run-image@" + runImageSha},
			}
			rt.Test(rtesting.TableRow{
				Key: stackKey,
				Objects: []runtime.Object{
					stack,
				},
				WantErr: false,
			})
		})

		it("sets the status to Ready False if error reading from registry", func() {
			fakeRegistryClient.SetFetchError(errors.New("some fetch error"))

			rt.Test(rtesting.TableRow{
				Key: stackKey,
				Objects: []runtime.Object{
					stack,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &expv1alpha1.Stack{
							ObjectMeta: stack.ObjectMeta,
							Spec:       stack.Spec,
							Status: expv1alpha1.StackStatus{
								Status: v1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: v1alpha1.Conditions{
										{
											Message: "some fetch error",
											Type:    v1alpha1.ConditionReady,
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

func randomImage(t *testing.T) (ggcrv1.Image, string) {
	image, err := random.Image(5, 10)
	require.NoError(t, err)

	hash, err := image.Digest()
	require.NoError(t, err)

	return image, hash.String()
}
