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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/cnb"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build/buildfakes"
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
			BuildInitImage:    "some/build-init-image",
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

	build := &v1alpha1.Build{
		ObjectMeta: v1.ObjectMeta{
			Name: buildName,
			Labels: map[string]string{
				"some/label": "to-pass-through",
			},
		},
		Spec: v1alpha1.BuildSpec{
			Image:          "someimage/name",
			ServiceAccount: "someserviceaccount",
			Builder:        "somebuilder/123",
			Env: []corev1.EnvVar{
				{Name: "keyA", Value: "valueA"},
				{Name: "keyB", Value: "valueB"},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("256M"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1"),
					corev1.ResourceMemory: resource.MustParse("128M"),
				},
			},
			Source: v1alpha1.Source{
				Git: v1alpha1.Git{
					URL:      "giturl.com/git.git",
					Revision: "gitrev1234",
				},
			},
			CacheName:            "some-cache-name",
			AdditionalImageNames: []string{"someimage/name:tag2", "someimage/name:tag3"},
		},
	}

	when("#Reconcile", func() {
		it.Before(func() {
			_, err := fakeBuildClient.BuildV1alpha1().Builds(namespace).Create(build)
			require.NoError(t, err)
		})

		when("a build hasn't been created", func() {
			it("creates a knative build with a persistent volume cache", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, knbuild.ObjectMeta, v1.ObjectMeta{
					Name:      buildName,
					Namespace: namespace,
					OwnerReferences: []v1.OwnerReference{
						*kmeta.NewControllerRef(build),
					},
					Labels: map[string]string{
						"some/label": "to-pass-through",
					},
				})
				assert.Equal(t, knbuild.Spec.ServiceAccountName, "someserviceaccount")
				assert.Equal(t, knbuild.Spec.Source, &knv1alpha1.SourceSpec{
					Git: &knv1alpha1.GitSourceSpec{
						Url:      "giturl.com/git.git",
						Revision: "gitrev1234",
					},
				})
				assert.Nil(t, knbuild.Spec.Template)
				require.Len(t, knbuild.Spec.Steps, 7)
				assert.Equal(t, knbuild.Spec.Steps[0].Image, "some/build-init-image")
				assert.Len(t, knbuild.Spec.Steps[0].Env, 2)
				assert.Equal(t, knbuild.Spec.Steps[0].Env[0], corev1.EnvVar{
					Name:  "BUILDER",
					Value: "somebuilder/123",
				})
				const root int64 = 0
				assert.Equal(t, *knbuild.Spec.Steps[0].SecurityContext.RunAsUser, root)
				assert.Equal(t, *knbuild.Spec.Steps[0].SecurityContext.RunAsGroup, root)
				assert.Equal(t, knbuild.Spec.Steps[1].Image, "somebuilder/123")
				assert.Contains(t, knbuild.Spec.Steps[5].Args, "someimage/name")
				require.Len(t, knbuild.Spec.Volumes, 3)
				assert.Equal(t, corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "some-cache-name"},
				}, knbuild.Spec.Volumes[0].VolumeSource)
			})

			it("creates a knative build with multiple image names on the export step", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Contains(t, knbuild.Spec.Steps[5].Args, "someimage/name")
				assert.Contains(t, knbuild.Spec.Steps[5].Args, "someimage/name:tag2")
				assert.Contains(t, knbuild.Spec.Steps[5].Args, "someimage/name:tag3")
			})

			it("creates a knative build with analyzed path on analyze", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Contains(t, knbuild.Spec.Steps[3].Args, "-analyzed=/layers/analyzed.toml")
			})

			it("creates a knative build with analyzed path on export", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Contains(t, knbuild.Spec.Steps[5].Args, "-analyzed=/layers/analyzed.toml")
			})

			it("creates a knative build with only one image name on the export step", func() {
				build.Spec.AdditionalImageNames = nil
				_, err := fakeBuildClient.BuildV1alpha1().Builds(namespace).Update(build)
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Contains(t, knbuild.Spec.Steps[5].Args, "someimage/name")
			})

			it("when cache name is empty, creates a knative build with no cache", func() {
				build.Spec.CacheName = ""
				_, err := fakeBuildClient.BuildV1alpha1().Builds(namespace).Update(build)
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

				require.Len(t, knbuild.Spec.Volumes, 3)
				assert.Equal(t, corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}, knbuild.Spec.Volumes[0].VolumeSource)
			})

			it("passes through build time env vars and platform volume", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

				require.Len(t, knbuild.Spec.Steps[0].Env, 2)
				assert.JSONEq(t, `[{"name": "keyA", "value": "valueA"}, {"name": "keyB", "value": "valueB"}]`, knbuild.Spec.Steps[0].Env[1].Value)

				// init
				require.Len(t, knbuild.Spec.Steps[0].VolumeMounts, 3)
				assert.Equal(t, knbuild.Spec.Steps[0].VolumeMounts[2].Name, "platform-dir")
				assert.Equal(t, knbuild.Spec.Steps[0].VolumeMounts[2].MountPath, "/platform")

				// detect
				require.Len(t, knbuild.Spec.Steps[1].VolumeMounts, 2)
				assert.Equal(t, knbuild.Spec.Steps[1].VolumeMounts[1].Name, "platform-dir")
				assert.Equal(t, knbuild.Spec.Steps[1].VolumeMounts[1].MountPath, "/platform")

				// build
				require.Len(t, knbuild.Spec.Steps[4].VolumeMounts, 2)
				assert.Equal(t, knbuild.Spec.Steps[4].VolumeMounts[1].Name, "platform-dir")
				assert.Equal(t, knbuild.Spec.Steps[4].VolumeMounts[1].MountPath, "/platform")

				require.Len(t, knbuild.Spec.Volumes, 3)
				assert.Equal(t, knbuild.Spec.Volumes[2].Name, "platform-dir")
			})

			it("passes through build resources", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

				for _, kb := range knbuild.Spec.Steps {
					assert.Equal(t, build.Spec.Resources, kb.Resources)
				}
			})
		})

		when("a build already created", func() {
			it("does not create or update knative builds", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

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
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				knbuild, err := fakeKNClient.BuildV1alpha1().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

				assert.NotEqual(t, knbuild.Spec.ServiceAccountName, "updatedsomeserviceaccount")
				assert.NotEqual(t, knbuild.Spec.Source.Git.Url, "updatedgiturl.com/git.git")
				assert.NotEqual(t, knbuild.Spec.Source.Git.Revision, "updated1234")
			})

			it("updates the build with the status of knative build", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

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
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

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
				require.NoError(t, err)

				const generationToHaveObserved int64 = 1234

				_, err = fakeBuildClient.BuildV1alpha1().Builds(namespace).Update(&v1alpha1.Build{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name:       buildName,
						Generation: generationToHaveObserved,
					},
				})
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, build.Generation, build.Status.ObservedGeneration)
				assert.Equal(t, generationToHaveObserved, build.Status.ObservedGeneration)
			})

			it("updates the build metadata on successful completion", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				const sha = "sha:1234567"
				builtImage := cnb.BuiltImage{
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
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

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
				require.NoError(t, err)

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
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, len(build.Status.BuildMetadata), 0)

				assert.Equal(t, fakeMetadataRetriever.GetBuiltImageCallCount(), 0)
			})

			it("does not update the build metadata if the build metadata has already been retrieved", func() {
				err := reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				builtImage := cnb.BuiltImage{
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
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				// subsequent call
				err = reconciler.Reconcile(context.TODO(), "some-namespace/build-name")
				require.NoError(t, err)

				build, err := fakeBuildClient.Build().Builds(namespace).Get(buildName, v1.GetOptions{})
				require.NoError(t, err)

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
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)
			})
		})
	})
}
