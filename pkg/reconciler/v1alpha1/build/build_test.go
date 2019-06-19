package build_test

import (
	"context"
	"testing"
	"time"

	lcyclemd "github.com/buildpack/lifecycle/metadata"
	knv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knfake "github.com/knative/build/pkg/client/clientset/versioned/fake"
	knexternalversions "github.com/knative/build/pkg/client/informers/externalversions"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/kmeta"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build/buildfakes"
	"github.com/pivotal/build-service-system/pkg/registry"
)

//go:generate counterfeiter . MetadataRetriever

func TestBuildReconciler(t *testing.T) {
	spec.Run(t, "Build Reconciler", testBuildReconciler)
}

func testBuildReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeKNClient := knfake.NewSimpleClientset(&knv1alpha1.Build{})
	fakeBuildClient := fake.NewSimpleClientset(&v1alpha1.Build{})

	informerFactory := externalversions.NewSharedInformerFactory(fakeBuildClient, time.Second)
	buildInformer := informerFactory.Build().V1alpha1().Builds()

	knInformerFactory := knexternalversions.NewSharedInformerFactory(fakeKNClient, time.Second)
	knbuildInformer := knInformerFactory.Build().V1alpha1().Builds()

	fakeMetadataRetriever := &buildfakes.FakeMetadataRetriever{}

	reconciler := testhelpers.SyncWaitingReconciler(
		informerFactory,
		&build.Reconciler{
			KNClient:          fakeKNClient,
			Client:            fakeBuildClient,
			Lister:            buildInformer.Lister(),
			KnLister:          knbuildInformer.Lister(),
			MetadataRetriever: fakeMetadataRetriever,
		},
		buildInformer.Informer().HasSynced,
		knbuildInformer.Informer().HasSynced,
	)
	stopChan := make(chan struct{})

	it.Before(func() {
		informerFactory.Start(stopChan)
		knInformerFactory.Start(stopChan)
	})

	it.After(func() {
		close(stopChan)
	})

	const (
		namespace = "some-namespace"
		buildName = "build-name"
		key       = "some-namespace/build-name"
	)

	Build := &v1alpha1.Build{
		ObjectMeta: v1.ObjectMeta{
			Name: buildName,
		},
		Spec: v1alpha1.BuildSpec{
			Image:          "someimage/name",
			ServiceAccount: "someserviceaccount",
			Builder:        "somebuilder/123",
			Source: v1alpha1.Source{
				Git: v1alpha1.Git{
					URL:      "giturl.com/git.git",
					Revision: "gitrev1234",
				},
			},
		},
	}

	when("#Reconcile", func() {
		it.Before(func() {
			_, err := fakeBuildClient.BuildV1alpha1().Builds(namespace).Create(Build)
			require.Nil(t, err)
		})

		when("a build hasn't been created", func() {
			it("creates a knative build", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.Nil(t, err)

				build, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.Nil(t, err)

				assert.Equal(t, build.ObjectMeta, v1.ObjectMeta{
					Name:      buildName,
					Namespace: namespace,
					OwnerReferences: []v1.OwnerReference{
						*kmeta.NewControllerRef(Build),
					},
				})
				assert.Equal(t, build.Spec.ServiceAccountName, "someserviceaccount")
				assert.Equal(t, build.Spec.Source, &knv1alpha1.SourceSpec{
					Git: &knv1alpha1.GitSourceSpec{
						Url:      "giturl.com/git.git",
						Revision: "gitrev1234",
					},
				})
				assert.Nil(t, build.Spec.Template)
				require.Len(t, build.Spec.Steps, 7)
				assert.Equal(t, build.Spec.Steps[1].Image, "somebuilder/123")
				assert.Contains(t, build.Spec.Steps[5].Args, "someimage/name")
			})
		})

		when("a build already created", func() {
			it("does not create or update knative builds", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				_, err = fakeBuildClient.BuildV1alpha1().Builds(namespace).Update(&v1alpha1.Build{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name: buildName,
					},
					Spec: v1alpha1.BuildSpec{
						Image:          "updatedsomeimage/name",
						ServiceAccount: "updatedsomeserviceaccount",
						Source: v1alpha1.Source{
							Git: v1alpha1.Git{
								URL:      "updatedgiturl.com/git.git",
								Revision: "updated1234",
							},
						},
					},
				})
				require.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				build, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.Nil(t, err)

				assert.NotEqual(t, build.Spec.ServiceAccountName, "updatedsomeserviceaccount")
				assert.NotEqual(t, build.Spec.Source.Git.Url, "updatedgiturl.com/git.git")
				assert.NotEqual(t, build.Spec.Source.Git.Revision, "updated1234")
			})

			it("updates the build with the status of knative build", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

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
				require.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.Nil(t, err)

				assert.Equal(t, build.Status.Conditions,
					duckv1alpha1.Conditions{
						{
							Type:   duckv1alpha1.ConditionSucceeded,
							Status: corev1.ConditionTrue,
						},
					},
				)
			})

			it("updates the observed generation", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				const generationToHaveObserved int64 = 1234

				_, err = fakeBuildClient.BuildV1alpha1().Builds(namespace).Update(&v1alpha1.Build{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name:       buildName,
						Generation: generationToHaveObserved,
					},
				})
				require.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.Nil(t, err)

				assert.Equal(t, build.Generation, build.Status.ObservedGeneration)
				assert.Equal(t, generationToHaveObserved, build.Status.ObservedGeneration)
			})

			it("updates the build metadata on successful completion", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				const sha = "sha:1234567"
				builtImage := registry.BuiltImage{
					SHA:         sha,
					CompletedAt: time.Time{},
					BuildpackMetadata: []lcyclemd.BuildpackMetadata{{
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
				require.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.Nil(t, err)

				assert.Equal(t, build.Status.BuildMetadata,
					v1alpha1.BuildpackMetadataList{{
						ID:      "1",
						Version: "foo",
					}})
				assert.Equal(t, build.Status.SHA, sha)

				assert.Equal(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 1)
				assert.Equal(t, fakeMetadataRetriever.GetBuiltImageArgsForCall(0), build)
			})

			it("does not update the build metadata if the build fails", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

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
				require.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.Nil(t, err)

				assert.Equal(t, len(build.Status.BuildMetadata), 0)

				assert.Equal(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 0)
			})

			it("does not update the build metadata if the build metadata has already been retrieved", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				builtImage := registry.BuiltImage{
					SHA:         "",
					CompletedAt: time.Time{},
					BuildpackMetadata: []lcyclemd.BuildpackMetadata{{
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
				require.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				//subsequent call
				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.Nil(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.Nil(t, err)

				assert.Equal(t, len(build.Status.BuildMetadata), 1)

				assert.Equal(t, build.Status.BuildMetadata,
					v1alpha1.BuildpackMetadataList{{
						ID:      "1",
						Version: "foo",
					}})

				assert.Equal(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 1)
			})
		})

		when("a build no longer exists", func() {
			it("does not return an error", func() {
				err := fakeBuildClient.BuildV1alpha1().Builds(namespace).Delete(buildName, &v1.DeleteOptions{})
				require.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				require.Nil(t, err)
			})
		})
	})
}
