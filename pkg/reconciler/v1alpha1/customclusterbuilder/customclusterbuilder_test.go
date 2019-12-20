package customclusterbuilder_test

import (
	"errors"
	"testing"

	"github.com/sclevine/spec"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/customclusterbuilder"
	"github.com/pivotal/kpack/pkg/registry"
	regtesthelpers "github.com/pivotal/kpack/pkg/registry/testhelpers"
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
		keychainFactory = &regtesthelpers.FakeKeychainFactory{}
		builderCreator  = &testhelpers.FakeBuilderCreator{}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
			listers := testhelpers.NewListers(row.Objects)
			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			r := &customclusterbuilder.Reconciler{
				Client:                     fakeClient,
				CustomClusterBuilderLister: listers.GetCustomClusterBuilderLister(),
				BuilderCreator:             builderCreator,
				KeychainFactory:            keychainFactory,
			}
			return r, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}, &rtesting.FakeStatsReporter{}
		})

	customBuilder := &expv1alpha1.CustomClusterBuilder{
		ObjectMeta: v1.ObjectMeta{
			Name:       customBuilderName,
			Generation: initialGeneration,
		},
		Spec: expv1alpha1.CustomClusterBuilderSpec{
			CustomBuilderSpec: expv1alpha1.CustomBuilderSpec{
				Tag: customBuilderTag,
				Stack: expv1alpha1.Stack{
					BaseBuilderImage: "example.com/some-base-image",
				},
				Store: expv1alpha1.Store{
					Image: "example.com/some-store-image",
				},
				Order: []expv1alpha1.Group{
					{
						Group: []expv1alpha1.Buildpack{
							{
								ID:       "buildpack.id.1",
								Version:  "1.0.0",
								Optional: false,
							},
							{
								ID:       "buildpack.id.2",
								Version:  "2.0.0",
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
			keychainFactory.AddKeychainForSecretRef(t, secretRef, &regtesthelpers.FakeKeychain{})
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
						ID:      "buildpack.id.1",
						Version: "1.0.0",
					},
					{
						ID:      "buildpack.id.2",
						Version: "2.0.0",
					},
				},
			}

			expectedBuilder := &expv1alpha1.CustomClusterBuilder{
				ObjectMeta: customBuilder.ObjectMeta,
				Spec:       customBuilder.Spec,
				Status: expv1alpha1.CustomBuilderStatus{
					BuilderStatus: v1alpha1.BuilderStatus{
						Status: duckv1alpha1.Status{
							ObservedGeneration: 1,
							Conditions: duckv1alpha1.Conditions{
								{
									Type:   duckv1alpha1.ConditionReady,
									Status: corev1.ConditionTrue,
								},
							},
						},
						BuilderMetadata: []v1alpha1.BuildpackMetadata{
							{
								ID:      "buildpack.id.1",
								Version: "1.0.0",
							},
							{
								ID:      "buildpack.id.2",
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
				Key:     customBuilderKey,
				Objects: []runtime.Object{customBuilder},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: expectedBuilder,
					},
				},
			})
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
						ID:      "buildpack.id.1",
						Version: "1.0.0",
					},
				},
			}

			customBuilder.Status.BuilderStatus = v1alpha1.BuilderStatus{
				Status: duckv1alpha1.Status{
					ObservedGeneration: customBuilder.Generation,
					Conditions: duckv1alpha1.Conditions{
						{
							Type:   duckv1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				BuilderMetadata: []v1alpha1.BuildpackMetadata{
					{
						ID:      "buildpack.id.1",
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
				Key:     customBuilderKey,
				Objects: []runtime.Object{customBuilder},
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
						Status: duckv1alpha1.Status{
							ObservedGeneration: 1,
							Conditions: duckv1alpha1.Conditions{
								{
									Type:    duckv1alpha1.ConditionReady,
									Status:  corev1.ConditionFalse,
									Message: "create error",
								},
							},
						},
					},
				},
			}

			rt.Test(rtesting.TableRow{
				Key:     customBuilderKey,
				Objects: []runtime.Object{customBuilder},
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
