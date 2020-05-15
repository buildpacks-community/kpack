package stack_test

import (
	"errors"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
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
	"github.com/pivotal/kpack/pkg/reconciler/stack"
	"github.com/pivotal/kpack/pkg/reconciler/stack/stackfakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
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

	fakeStackReader := &stackfakes.FakeStackReader{}

	testStack := &expv1alpha1.Stack{
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

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &stack.Reconciler{
				Client:      fakeClient,
				StackLister: listers.GetStackLister(),
				StackReader: fakeStackReader,
			}
			return r, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}
		})

	when("#Reconcile", func() {
		it("saves metadata to the status", func() {
			resolvedStack := expv1alpha1.ResolvedStack{
				BuildImage: expv1alpha1.StackStatusImage{
					LatestImage: "some-registry.io/build-image@sha245:123",
				},
				RunImage: expv1alpha1.StackStatusImage{
					LatestImage: "some-registry.io/run-image@sha245:123",
				},
				Mixins:  []string{"a-nice-mixin"},
				UserID:  1000,
				GroupID: 2000,
			}
			fakeStackReader.ReadReturns(resolvedStack, nil)

			rt.Test(rtesting.TableRow{
				Key: stackKey,
				Objects: []runtime.Object{
					testStack,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &expv1alpha1.Stack{
							ObjectMeta: testStack.ObjectMeta,
							Spec:       testStack.Spec,
							Status: expv1alpha1.StackStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: 1,
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								ResolvedStack: resolvedStack,
							},
						},
					},
				},
			})

			require.Equal(t, 1, fakeStackReader.ReadCallCount())
			require.Equal(t, testStack.Spec, fakeStackReader.ReadArgsForCall(0))
		})

		it("does not update the status with no status change", func() {
			resolvedStack := expv1alpha1.ResolvedStack{
				BuildImage: expv1alpha1.StackStatusImage{
					LatestImage: "some-registry.io/build-image@sha245:123",
				},
				RunImage: expv1alpha1.StackStatusImage{
					LatestImage: "some-registry.io/run-image@sha245:123",
				},
				Mixins:  []string{"a-nice-mixin"},
				UserID:  1000,
				GroupID: 2000,
			}
			fakeStackReader.ReadReturns(resolvedStack, nil)

			testStack.Status = expv1alpha1.StackStatus{
				Status: corev1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: corev1alpha1.Conditions{
						{
							Type:   corev1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				ResolvedStack: resolvedStack,
			}
			rt.Test(rtesting.TableRow{
				Key: stackKey,
				Objects: []runtime.Object{
					testStack,
				},
				WantErr: false,
			})
		})

		it("sets the status to Ready False if error reading from stack", func() {
			fakeStackReader.ReadReturns(expv1alpha1.ResolvedStack{}, errors.New("invalid mixins on run image"))

			rt.Test(rtesting.TableRow{
				Key: stackKey,
				Objects: []runtime.Object{
					testStack,
				},
				WantErr: true,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &expv1alpha1.Stack{
							ObjectMeta: testStack.ObjectMeta,
							Spec:       testStack.Spec,
							Status: expv1alpha1.StackStatus{
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
	})
}
