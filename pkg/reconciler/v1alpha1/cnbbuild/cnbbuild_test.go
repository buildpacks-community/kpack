package cnbbuild_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/buildpack/lifecycle"
	knv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knfake "github.com/knative/build/pkg/client/clientset/versioned/fake"
	knexternalversions "github.com/knative/build/pkg/client/informers/externalversions"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/kmeta"
	"github.com/sclevine/spec"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbbuild"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbbuild/cnbbuildfakes"
	"github.com/pivotal/build-service-system/pkg/registry"
)

//go:generate counterfeiter . MetadataRetriever

func TestCNBBuildReconciler(t *testing.T) {
	spec.Run(t, "CNBBuild Reconciler", testCNBBuildReconciler)
}

func testCNBBuildReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeKNClient := knfake.NewSimpleClientset(&knv1alpha1.Build{})
	fakeCnbBuildClient := fake.NewSimpleClientset(&v1alpha1.CNBBuild{})

	cnbbuildInformer := externalversions.NewSharedInformerFactory(fakeCnbBuildClient, time.Millisecond).Build().V1alpha1().CNBBuilds()
	knbuildInformer := knexternalversions.NewSharedInformerFactory(fakeKNClient, time.Millisecond).Build().V1alpha1().Builds()

	fakeMetadataRetriever := &cnbbuildfakes.FakeMetadataRetriever{}

	reconciler := testhelpers.SyncWaitingReconciler(
		&cnbbuild.Reconciler{
			KNClient:          fakeKNClient,
			CNBBuildClient:    fakeCnbBuildClient,
			CNBLister:         cnbbuildInformer.Lister(),
			KnLister:          knbuildInformer.Lister(),
			MetadataRetriever: fakeMetadataRetriever,
		},
		cnbbuildInformer.Informer().HasSynced,
		knbuildInformer.Informer().HasSynced,
	)
	stopChan := make(chan struct{})

	it.Before(func() {
		go func() {
			cnbbuildInformer.Informer().Run(stopChan)
		}()
		go func() {
			knbuildInformer.Informer().Run(stopChan)
		}()
	})

	it.After(func() {
		close(stopChan)
	})

	const namespace = "some-namespace"
	const buildName = "cnb-build-name"
	const key = "some-namespace/cnb-build-name"

	cnbBuild := &v1alpha1.CNBBuild{
		TypeMeta: v1.TypeMeta{},
		ObjectMeta: v1.ObjectMeta{
			Name: buildName,
		},
		Spec: v1alpha1.CNBBuildSpec{
			Image:          "someimage/name",
			ServiceAccount: "someserviceaccount",
			GitURL:         "giturl.com/git.git",
			GitRevision:    "gitrev1234",
			Builder:        "somebuilder/123",
		},
	}

	when("#Reconcile", func() {
		it.Before(func() {
			_, err := fakeCnbBuildClient.BuildV1alpha1().CNBBuilds(namespace).Create(cnbBuild)
			assertNil(t, err)
		})

		when("a build hasn't been created", func() {
			it("creates a knative build", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				assertNil(t, err)

				build, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				assertNil(t, err)

				assertEqual(t, build, &knv1alpha1.Build{
					ObjectMeta: v1.ObjectMeta{
						Name:      buildName,
						Namespace: namespace,
						OwnerReferences: []v1.OwnerReference{
							*kmeta.NewControllerRef(cnbBuild),
						},
					},
					Spec: knv1alpha1.BuildSpec{
						ServiceAccountName: "someserviceaccount",
						Source: &knv1alpha1.SourceSpec{
							Git: &knv1alpha1.GitSourceSpec{
								Url:      "giturl.com/git.git",
								Revision: "gitrev1234",
							},
						},
						Template: &knv1alpha1.TemplateInstantiationSpec{
							Name: "buildpacks-cnb",
							Arguments: []knv1alpha1.ArgumentSpec{
								{Name: "IMAGE", Value: "someimage/name"},
								{Name: "BUILDER_IMAGE", Value: "somebuilder/123"},
							},
						},
					},
				})
			})
		})

		when("a build already created", func() {
			it("does not create or update knative builds", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				_, err = fakeCnbBuildClient.BuildV1alpha1().CNBBuilds(namespace).Update(&v1alpha1.CNBBuild{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: buildName,
					},
					Spec: v1alpha1.CNBBuildSpec{
						Image:          "updatedsomeimage/name",
						ServiceAccount: "updatedsomeserviceaccount",
						GitURL:         "updatedgiturl.com/git.git",
						GitRevision:    "updated1234",
					},
				})
				assertNil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				build, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				assertNil(t, err)

				assertNotEqual(t, build.Spec.ServiceAccountName, "updatedsomeserviceaccount")
				assertNotEqual(t, build.Spec.Source.Git.Url, "updatedgiturl.com/git.git")
				assertNotEqual(t, build.Spec.Source.Git.Revision, "updated1234")
			})

			it("updates the build with the status of knative build", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				_, err = fakeKNClient.BuildV1alpha1().Builds(namespace).UpdateStatus(
					&knv1alpha1.Build{
						ObjectMeta: v1.ObjectMeta{
							Name: buildName,
						},
						Status: knv1alpha1.BuildStatus{
							Status: duckv1alpha1.Status{
								Conditions: duckv1alpha1.Conditions{
									{
										Type:   duckv1alpha1.ConditionSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
				)
				assertNil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				build, err := fakeCnbBuildClient.Build().CNBBuilds(namespace).Get(buildName, v1.GetOptions{})
				assertNil(t, err)

				assertEqual(t, build.Status.Conditions,
					duckv1alpha1.Conditions{
						{
							Type:   duckv1alpha1.ConditionSucceeded,
							Status: corev1.ConditionTrue,
						},
					},
				)
			})

			it("updates the observed generation", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				const generationToHaveObserved int64 = 1234

				_, err = fakeCnbBuildClient.BuildV1alpha1().CNBBuilds(namespace).Update(&v1alpha1.CNBBuild{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name:       buildName,
						Generation: generationToHaveObserved,
					},
				})
				assertNil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				build, err := fakeCnbBuildClient.Build().CNBBuilds(namespace).Get(buildName, v1.GetOptions{})
				assertNil(t, err)

				assertEqual(t, build.Generation, build.Status.ObservedGeneration)
				assertEqual(t, generationToHaveObserved, build.Status.ObservedGeneration)
			})

			it("updates the build metadata on successful completion", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				builtImage := registry.BuiltImage{
					Sha:         "",
					CompletedAt: time.Time{},
					BuildpackMetadata: []lifecycle.BuildpackMetadata{{
						ID:      "1",
						Version: "foo",
						Layers:  nil,
					}},
				}
				fakeMetadataRetriever.GetBuiltImageReturns(builtImage, nil)

				_, err = fakeKNClient.BuildV1alpha1().Builds(namespace).UpdateStatus(
					&knv1alpha1.Build{
						ObjectMeta: v1.ObjectMeta{
							Name: buildName,
						},
						Status: knv1alpha1.BuildStatus{
							Status: duckv1alpha1.Status{
								Conditions: duckv1alpha1.Conditions{
									{
										Type:   duckv1alpha1.ConditionSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
				)
				assertNil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				build, err := fakeCnbBuildClient.Build().CNBBuilds(namespace).Get(buildName, v1.GetOptions{})
				assertNil(t, err)

				assertEqual(t, build.Status.BuildMetadata,
					v1alpha1.CNBBuildpackMetadataList{{
						ID:      "1",
						Version: "foo",
					}})

				assertEqual(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 1)
				assertEqual(t, fakeMetadataRetriever.GetBuiltImageArgsForCall(0), build)
			})

			it("does not update the build metadata if the build fails", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				_, err = fakeKNClient.BuildV1alpha1().Builds(namespace).UpdateStatus(
					&knv1alpha1.Build{
						ObjectMeta: v1.ObjectMeta{
							Name: buildName,
						},
						Status: knv1alpha1.BuildStatus{
							Status: duckv1alpha1.Status{
								Conditions: duckv1alpha1.Conditions{
									{
										Type:   duckv1alpha1.ConditionSucceeded,
										Status: corev1.ConditionFalse,
									},
								},
							},
						},
					},
				)
				assertNil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				build, err := fakeCnbBuildClient.Build().CNBBuilds(namespace).Get(buildName, v1.GetOptions{})
				assertNil(t, err)

				assertEqual(t, len(build.Status.BuildMetadata), 0)

				assertEqual(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 0)
			})

			it("does not update the build metadata if the build metadata has already been retrieved", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				builtImage := registry.BuiltImage{
					Sha:         "",
					CompletedAt: time.Time{},
					BuildpackMetadata: []lifecycle.BuildpackMetadata{{
						ID:      "1",
						Version: "foo",
						Layers:  nil,
					}},
				}
				fakeMetadataRetriever.GetBuiltImageReturns(builtImage, nil)
				_, err = fakeKNClient.BuildV1alpha1().Builds(namespace).UpdateStatus(
					&knv1alpha1.Build{
						ObjectMeta: v1.ObjectMeta{
							Name: buildName,
						},
						Status: knv1alpha1.BuildStatus{
							Status: duckv1alpha1.Status{
								Conditions: duckv1alpha1.Conditions{
									{
										Type:   duckv1alpha1.ConditionSucceeded,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					},
				)
				assertNil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				//subsequent call
				err = reconciler.Reconcile(context.TODO(), "some-namespace/cnb-build-name")
				assertNil(t, err)

				build, err := fakeCnbBuildClient.Build().CNBBuilds(namespace).Get(buildName, v1.GetOptions{})
				assertNil(t, err)

				assertEqual(t, len(build.Status.BuildMetadata), 1)

				assertEqual(t, build.Status.BuildMetadata,
					v1alpha1.CNBBuildpackMetadataList{{
						ID:      "1",
						Version: "foo",
					}})

				assertEqual(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 1)
			})
		})

		when("a cnb build no longer exists", func() {
			it("does not return an error", func() {
				err := fakeCnbBuildClient.BuildV1alpha1().CNBBuilds(namespace).Delete(buildName, &v1.DeleteOptions{})
				assertNil(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				assertNil(t, err)
			})
		})
	})
}

func assertNotEqual(t *testing.T, actual interface{}, not interface{}) {
	t.Helper()
	if reflect.DeepEqual(actual, not) {
		t.Fatalf("Expected %+v\n not to equal %+v", actual, not)
	}
}

func assertEqual(t *testing.T, actual interface{}, expected interface{}) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("Expected %+v\n to equal %+v", actual, expected)
	}
}

func assertNil(t *testing.T, obj interface{}) {
	t.Helper()
	if obj != nil {
		t.Fatalf("Unexpected %+v", obj)
	}
}
