package store_test

import (
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1"
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
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/store"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestStoreReconciler(t *testing.T) {
	spec.Run(t, "Custom Cluster Builder Reconciler", testStoreReconciler)
}

func testStoreReconciler(t *testing.T, when spec.G, it spec.S) {
	const (
		storeName               = "some-store"
		storeKey                = storeName
		initialGeneration int64 = 1
	)

	var (
		fakeBuildpackage1, fakeBuildpackage2 v1.Image
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
			var err error

			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)

			fakeBuildpackageClient := registryfakes.NewFakeClient()

			fakeBuildpackage1, err = random.Image(0, 0)
			require.NoError(t, err)

			fakeBuildpackage1, err = imagehelpers.SetStringLabels(fakeBuildpackage1, map[string]string{"io.buildpacks.buildpack.layers": `{
  "org.cloudfoundry.node-engine": {
    "0.0.116": {
      "api": "0.2",
      "stacks": [
        {
          "id": "io.buildpacks.stacks.bionic"
        }
      ],
      "layerDiffID": "sha256:d57937f5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf4"
    }
  }
}`})
			require.NoError(t, err)

			fakeBuildpackageClient.AddImage("some.registry/some-image-1", fakeBuildpackage1, "", authn.DefaultKeychain)

			fakeBuildpackage2, err = random.Image(0, 0)
			require.NoError(t, err)

			fakeBuildpackage2, err = imagehelpers.SetStringLabels(fakeBuildpackage2, map[string]string{"io.buildpacks.buildpack.layers": `{
  "org.cloudfoundry.npm": {
    "0.0.71": {
      "api": "0.2",
      "stacks": [
        {
          "id": "io.buildpacks.stacks.bionic"
        }
      ],
      "layerDiffID": "sha256:c67840e5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf5"
    }
  }
}`})
			require.NoError(t, err)

			fakeBuildpackageClient.AddImage("some.registry/some-image-2", fakeBuildpackage2, "", authn.DefaultKeychain)

			r := &store.Reconciler{
				Client:             fakeClient,
				BuildPackageClient: fakeBuildpackageClient,
				StoreLister:        listers.GetStoreLister(),
			}
			return r, rtesting.ActionRecorderList{fakeClient}, rtesting.EventList{Recorder: record.NewFakeRecorder(10)}, &rtesting.FakeStatsReporter{}
		})

	subject := &expv1alpha1.Store{
		ObjectMeta: metav1.ObjectMeta{
			Name:       storeName,
			Generation: initialGeneration,
		},
		Spec: expv1alpha1.StoreSpec{
			Sources: []expv1alpha1.BuildPackage{
				{
					Image: "some.registry/some-image-1",
				},
				{
					Image: "some.registry/some-image-2",
				},
			},
			ServiceAccountRef: corev1.ObjectReference{
				Name:      "some-service-account",
				Namespace: "some-namespace",
			},
		},
	}

	when("#Reconcile", func() {
		it("saves metadata to the status", func() {
			expectedStore := &expv1alpha1.Store{
				ObjectMeta: subject.ObjectMeta,
				Spec:       subject.Spec,
				Status: expv1alpha1.StoreStatus{
					Status: v1alpha1.Status{
						ObservedGeneration: 1,
						Conditions: v1alpha1.Conditions{
							{
								Type:   v1alpha1.ConditionReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
					Buildpacks: []expv1alpha1.StoreBuildpack{
						{
							ID:          "org.cloudfoundry.node-engine",
							Version:     "0.0.116",
							LayerDiffID: "sha256:d57937f5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf4",
							BuildPackage: expv1alpha1.BuildPackage{
								Image: "some.registry/some-image-1",
							},
							Order: nil,
						},
						{
							ID:          "org.cloudfoundry.npm",
							Version:     "0.0.71",
							LayerDiffID: "sha256:c67840e5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf5",
							BuildPackage: expv1alpha1.BuildPackage{
								Image: "some.registry/some-image-2",
							},
							Order: nil,
						},
					},
				},
			}

			rt.Test(rtesting.TableRow{
				Key: storeKey,
				Objects: []runtime.Object{
					subject,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: expectedStore,
					},
				},
			})
		})

		it("does not update the status with no status change", func() {
			subject.Status = expv1alpha1.StoreStatus{
				Status: v1alpha1.Status{
					ObservedGeneration: 1,
					Conditions: v1alpha1.Conditions{
						{
							Type:   v1alpha1.ConditionReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
				Buildpacks: []expv1alpha1.StoreBuildpack{
					{
						ID:          "org.cloudfoundry.node-engine",
						Version:     "0.0.116",
						LayerDiffID: "sha256:d57937f5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf4",
						BuildPackage: expv1alpha1.BuildPackage{
							Image: "some.registry/some-image-1",
						},
						Order: nil,
					},
					{
						ID:          "org.cloudfoundry.npm",
						Version:     "0.0.71",
						LayerDiffID: "sha256:c67840e5ccb6f524afa02dd95224e1914c94a02483d37b07aa668e560dcb3bf5",
						BuildPackage: expv1alpha1.BuildPackage{
							Image: "some.registry/some-image-2",
						},
						Order: nil,
					},
				},
			}
			rt.Test(rtesting.TableRow{
				Key: storeKey,
				Objects: []runtime.Object{
					subject,
				},
				WantErr: false,
			})
		})
	})
}
