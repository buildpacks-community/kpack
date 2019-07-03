package image_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knCtrl "github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	v1build "github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/image"
)

func TestImageReconciler(t *testing.T) {
	spec.Run(t, "Image Reconciler", testImageReconciler)
}

func testImageReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeClient := fake.NewSimpleClientset(&v1alpha1.Image{}, &v1alpha1.Builder{})

	k8sfakeClient := k8sfake.NewSimpleClientset(&corev1.PersistentVolumeClaim{})
	k8sInformerFactory := informers.NewSharedInformerFactory(k8sfakeClient, time.Second)
	pvcInformer := k8sInformerFactory.Core().V1().PersistentVolumeClaims()
	pvcLister := pvcInformer.Lister()

	fakeTracker := fakeTracker{}

	reconciler := testhelpers.RebuildingReconciler(func() knCtrl.Reconciler {
		informerFactory := externalversions.NewSharedInformerFactory(fakeClient, time.Second)
		imageInformer := informerFactory.Build().V1alpha1().Images()
		buildInformer := informerFactory.Build().V1alpha1().Builds()
		builderInformer := informerFactory.Build().V1alpha1().Builders()
		sourceResolverInformer := informerFactory.Build().V1alpha1().SourceResolvers()

		return testhelpers.SyncWaitingReconciler(
			informerFactory,
			&image.Reconciler{
				K8sClient:            k8sfakeClient,
				Client:               fakeClient,
				ImageLister:          imageInformer.Lister(),
				BuildLister:          buildInformer.Lister(),
				BuilderLister:        builderInformer.Lister(),
				SourceResolverLister: sourceResolverInformer.Lister(),
				PvcLister:            pvcLister,
				Tracker:              fakeTracker,
			},
			imageInformer.Informer().HasSynced,
			buildInformer.Informer().HasSynced,
			builderInformer.Informer().HasSynced,
			sourceResolverInformer.Informer().HasSynced,
			pvcInformer.Informer().HasSynced,
		)
	})

	const (
		imageName                = "image-name"
		builderName              = "builder-name"
		serviceAccount           = "service-account"
		namespace                = "some-namespace"
		key                      = "some-namespace/image-name"
		originalGeneration int64 = 1
	)
	var (
		failedBuildHistoryLimit  int64 = 4
		successBuildHistoryLimit int64 = 4
		quantity                       = resource.MustParse("1.5")
	)

	image := &v1alpha1.Image{
		ObjectMeta: v1.ObjectMeta{
			Name:       imageName,
			Namespace:  namespace,
			Generation: originalGeneration,
		},
		Spec: v1alpha1.ImageSpec{
			Image:          "some/image",
			ServiceAccount: serviceAccount,
			BuilderRef:     builderName,
			Source: v1alpha1.Source{
				Git: v1alpha1.Git{
					URL:      "https://some.git/url",
					Revision: "revision",
				},
			},
			CacheSize:                   &quantity,
			FailedBuildHistoryLimit:     &failedBuildHistoryLimit,
			SuccessBuildHistoryLimit:    &successBuildHistoryLimit,
			DisableAdditionalImageNames: true,
		},
	}

	stopChan := make(chan struct{})

	defaultBuildMetadata := v1alpha1.BuildpackMetadataList{
		{
			ID:      "buildpack.version",
			Version: "version",
		},
	}

	builder := &v1alpha1.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name: builderName,
		},
		Spec: v1alpha1.BuilderSpec{
			Image: "some/builder@sha256acf123",
		},
		Status: v1alpha1.BuilderStatus{
			BuilderMetadata: defaultBuildMetadata,
		},
	}

	it.Before(func() {
		k8sInformerFactory.Start(stopChan)
		_, err := fakeClient.BuildV1alpha1().Builders(namespace).Create(builder)
		require.NoError(t, err)
	})

	it.After(func() {
		close(stopChan)
	})

	when("#Reconcile", func() {
		when("new image", func() {
			it.Before(func() {
				_, err := fakeClient.BuildV1alpha1().Images(namespace).Create(image)
				require.NoError(t, err)
			})

			it("creates a source resolver for the image", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				list, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).List(v1.ListOptions{})
				require.NoError(t, err)

				require.Len(t, list.Items, 1)

				sourceResolver := list.Items[0]
				require.Equal(t, sourceResolver.Spec, v1alpha1.SourceResolverSpec{
					ServiceAccount: serviceAccount,
					Source: v1alpha1.Source{
						Git: v1alpha1.Git{
							URL:      "https://some.git/url",
							Revision: "revision",
						},
					},
				})
			})

			it("updates the observed generation with the new spec", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				updatedImage, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, updatedImage.Status.ObservedGeneration, originalGeneration)
			})

			it("creates a cache for the build", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				_, err = k8sfakeClient.CoreV1().PersistentVolumeClaims(namespace).Get(image.CacheName(), v1.GetOptions{})
				require.NoError(t, err)
			})

			it("tracks the builder", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				updatedImage, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
				require.NoError(t, err)

				gvk := builder.GetGroupVersionKind()
				isTracking := fakeTracker.IsTracking(corev1.ObjectReference{
					APIVersion: gvk.GroupVersion().String(),
					Kind:       gvk.Kind,
					Namespace:  namespace,
					Name:       builderName,
				}, updatedImage)

				assert.True(t, isTracking)
			})

			it("does not schedule builds until a source resolver has completed", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				build, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
				require.NoError(t, err)
				require.Len(t, build.Items, 0)
			})
		})

		when("source has resolved for a new image", func() {
			it.Before(func() {
				_, err := fakeClient.BuildV1alpha1().Images(namespace).Create(image)
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				resolveImageSource(t, fakeClient, namespace)
			})

			it("creates an initial Build with a cache", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				build, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
				require.NoError(t, err)
				require.Equal(t, len(build.Items), 1)

				buildName := build.Items[0].ObjectMeta.Name
				assert.Equal(t, v1alpha1.Build{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name:      buildName,
						Namespace: namespace,
						OwnerReferences: []v1.OwnerReference{
							*kmeta.NewControllerRef(image),
						},
						Labels: map[string]string{
							v1alpha1.BuildNumberLabel: "1",
							v1alpha1.ImageLabel:       imageName,
						},
					},
					Spec: v1alpha1.BuildSpec{
						Image:          "some/image",
						Builder:        "some/builder@sha256acf123",
						ServiceAccount: "service-account",
						CacheName:      imageName + "-cache",
						Source: v1alpha1.Source{
							Git: v1alpha1.Git{
								URL:      "https://some.git/url",
								Revision: "revision",
							},
						},
					},
				}, build.Items[0])
				updatedImage, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, updatedImage.Status.LastBuildRef, buildName)
			})

			it("updates the build count", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				updatedImage, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, updatedImage.Status.BuildCounter, int32(1))
			})
		})

		when("a build has already been created", func() {
			it.Before(func() {
				_, err := fakeClient.BuildV1alpha1().Images(namespace).Create(image)
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				resolveImageSource(t, fakeClient, namespace)

				err = reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)
			})

			when("a new spec is applied", func() {
				const newGeneration int64 = 2

				var newQuantity = resource.MustParse("2")

				var updatedImage *v1alpha1.Image

				it.Before(func() {
					img, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
					require.NoError(t, err)

					updatedImage, err = fakeClient.BuildV1alpha1().Images(namespace).Update(&v1alpha1.Image{
						ObjectMeta: v1.ObjectMeta{
							Name:       imageName,
							Generation: newGeneration,
						},
						Spec: v1alpha1.ImageSpec{
							Image:          "different/image",
							ServiceAccount: "different/service-account",
							BuilderRef:     builderName,
							Source: v1alpha1.Source{
								Git: v1alpha1.Git{
									URL:      "https://different.git/url",
									Revision: "differentrevision",
								},
							},
							CacheSize:                   &newQuantity,
							FailedBuildHistoryLimit:     &failedBuildHistoryLimit,
							SuccessBuildHistoryLimit:    &successBuildHistoryLimit,
							DisableAdditionalImageNames: true,
						},
						Status: img.Status, // fake client overwrites status :(
					})
					require.NoError(t, err)
				})

				it("updates the image's source resolver", func() {
					updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					require.NoError(t, err)

					list, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).List(v1.ListOptions{})
					require.NoError(t, err)
					require.Len(t, list.Items, 1)

					sourceResolver := list.Items[0]
					require.Equal(t, sourceResolver.Spec, v1alpha1.SourceResolverSpec{
						ServiceAccount: "different/service-account",
						Source: v1alpha1.Source{
							Git: v1alpha1.Git{
								URL:      "https://different.git/url",
								Revision: "differentrevision",
							},
						},
					})
				})

				when("image source resolved", func() {
					it.Before(func() {
						sourceResolvers, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).List(v1.ListOptions{})
						require.NoError(t, err)
						require.Len(t, sourceResolvers.Items, 1)
						resolver := sourceResolvers.Items[0]

						_, err = fakeClient.BuildV1alpha1().SourceResolvers(namespace).UpdateStatus(&v1alpha1.SourceResolver{
							ObjectMeta: resolver.ObjectMeta,
							Spec:       resolver.Spec, // fake client overwrites spec :(
							Status: v1alpha1.SourceResolverStatus{
								Status: alpha1.Status{
									Conditions: alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								ResolvedSource: v1alpha1.ResolvedSource{
									Git: v1alpha1.ResolvedGitSource{
										URL:      updatedImage.Spec.Source.Git.URL,
										Revision: updatedImage.Spec.Source.Git.Revision,
										Type:     v1alpha1.Commit,
									},
								},
							},
						})
						require.NoError(t, err)

					})

					it("does not create a build when a build is running", func() {
						updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
							Type:   duckv1alpha1.ConditionSucceeded,
							Status: corev1.ConditionUnknown,
						})

						err := reconciler.Reconcile(context.TODO(), key)
						require.NoError(t, err)

						builds, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
						require.NoError(t, err)
						assert.Equal(t, len(builds.Items), 1)

						updatedImage, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
						require.NoError(t, err)
						assert.Equal(t, updatedImage.Status.ObservedGeneration, originalGeneration)
					})

					it("does create a build when the last build is successful", func() {
						updateStatusOfLastBuild(t, fakeClient, namespace, defaultBuildMetadata, duckv1alpha1.Condition{
							Type:   duckv1alpha1.ConditionSucceeded,
							Status: corev1.ConditionTrue,
						})

						err := reconciler.Reconcile(context.TODO(), key)
						require.NoError(t, err)

						builds, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
						require.NoError(t, err)
						assert.Equal(t, 2, len(builds.Items))

						updatedImage, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
						require.NoError(t, err)
						assert.Equal(t, updatedImage.Status.ObservedGeneration, newGeneration)

						newBuild, err := fakeClient.BuildV1alpha1().Builds(namespace).Get(updatedImage.Status.LastBuildRef, v1.GetOptions{})
						require.NoError(t, err)
						assert.Equal(t, newBuild, &v1alpha1.Build{
							TypeMeta: v1.TypeMeta{},
							ObjectMeta: v1.ObjectMeta{
								Name:      updatedImage.Status.LastBuildRef,
								Namespace: namespace,
								OwnerReferences: []v1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "2",
									v1alpha1.ImageLabel:       imageName,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Image:          "different/image",
								ServiceAccount: "different/service-account",
								Builder:        "some/builder@sha256acf123",
								CacheName:      imageName + "-cache",
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      "https://different.git/url",
										Revision: "differentrevision",
									},
								},
							},
						})
					})

					it("does create a build when the last build is a failure", func() {
						updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
							Type:   duckv1alpha1.ConditionSucceeded,
							Status: corev1.ConditionFalse,
						})

						err := reconciler.Reconcile(context.TODO(), key)
						require.NoError(t, err)

						builds, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
						require.NoError(t, err)
						assert.Equal(t, len(builds.Items), 2)
					})

					it("updates the image's volume cache", func() {
						updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
							Type:   duckv1alpha1.ConditionSucceeded,
							Status: corev1.ConditionTrue,
						})

						err := reconciler.Reconcile(context.TODO(), key)
						require.NoError(t, err)

						list, err := k8sfakeClient.CoreV1().PersistentVolumeClaims(namespace).List(v1.ListOptions{})
						require.NoError(t, err)
						require.Len(t, list.Items, 1)

						sourceResolver := list.Items[0]
						require.Equal(t, sourceResolver.Spec, corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceStorage: newQuantity,
								},
							},
						})
					})
				})

			})

			when("referenced builder has been updated", func() {
				it.Before(func() {
					_, err := fakeClient.BuildV1alpha1().Builders(namespace).Update(&v1alpha1.Builder{
						ObjectMeta: v1.ObjectMeta{
							Name: builderName,
						},
						Spec: v1alpha1.BuilderSpec{
							Image: "some/builder@sha256:newsha",
						},
						Status: v1alpha1.BuilderStatus{
							BuilderMetadata: []v1alpha1.BuildpackMetadata{
								{
									ID:      "new.buildpack",
									Version: "version",
								},
							},
						},
					})
					require.NoError(t, err)
				})

				it("does not create a build when a build is running", func() {
					updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					require.NoError(t, err)

					builds, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
					require.NoError(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})

				it("does create a build when the last build is no longer running", func() {
					updateStatusOfLastBuild(t, fakeClient, namespace, defaultBuildMetadata, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					require.NoError(t, err)

					builds, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
					require.NoError(t, err)
					assert.Equal(t, len(builds.Items), 2)

					updatedImage, err := fakeClient.BuildV1alpha1().Images(namespace).Get(imageName, v1.GetOptions{})
					require.NoError(t, err)

					newBuild, err := fakeClient.BuildV1alpha1().Builds(namespace).Get(updatedImage.Status.LastBuildRef, v1.GetOptions{})
					require.NoError(t, err)
					assert.Equal(t, newBuild, &v1alpha1.Build{
						TypeMeta: v1.TypeMeta{},
						ObjectMeta: v1.ObjectMeta{
							Name:      updatedImage.Status.LastBuildRef,
							Namespace: namespace,
							OwnerReferences: []v1.OwnerReference{
								*kmeta.NewControllerRef(image),
							},
							Labels: map[string]string{
								v1alpha1.BuildNumberLabel: "2",
								v1alpha1.ImageLabel:       imageName,
							},
						},
						Spec: v1alpha1.BuildSpec{
							Image:          "some/image",
							Builder:        "some/builder@sha256:newsha",
							ServiceAccount: "service-account",
							CacheName:      imageName + "-cache",
							Source: v1alpha1.Source{
								Git: v1alpha1.Git{
									URL:      "https://some.git/url",
									Revision: "revision",
								},
							},
						},
					})
				})
			})

			when("no new spec has been applied", func() {
				it("does not create a new build when last build is running", func() {
					updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					require.NoError(t, err)

					builds, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
					require.NoError(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})

				it("does not create a build when the last build is no longer running", func() {
					updateStatusOfLastBuild(t, fakeClient, namespace, defaultBuildMetadata, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					require.NoError(t, err)

					builds, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
					require.NoError(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})
			})

		})

		when("an image status is not up to date", func() {
			it.Before(func() {
				_, err := fakeClient.BuildV1alpha1().Images(namespace).Create(image)
				require.NoError(t, err)
			})

			it("does not create duplicate builds", func() {
				_, err := fakeClient.BuildV1alpha1().Builds(namespace).Create(&v1alpha1.Build{
					ObjectMeta: v1.ObjectMeta{
						Name: "gotprocessed-beforeimage-saved",
						Labels: map[string]string{
							v1alpha1.BuildNumberLabel: "1",
							v1alpha1.ImageLabel:       imageName,
						},
					},
					Spec:   v1alpha1.BuildSpec{},
					Status: v1alpha1.BuildStatus{},
				})
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				assert.Error(t, err, fmt.Sprintf("warning: image %s status not up to date", imageName))

				build, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
				require.NoError(t, err)

				assert.Equal(t, len(build.Items), 1)
			})
		})

		when("failed builds have exceeded the failedHistoryLimit", func() {
			var firstBuild *v1alpha1.Build

			it.Before(func() {
				_, err := fakeClient.BuildV1alpha1().Images(namespace).Create(image)
				require.Nil(t, err)

				for i := int64(0); i < failedBuildHistoryLimit+1; i++ {
					build := image.CreateBuild(resolvedSourceResolverForImage(image), builder)
					build.ObjectMeta.CreationTimestamp = v1.NewTime(time.Now().Add(time.Duration(i) * time.Minute))
					_, err = fakeClient.BuildV1alpha1().Builds(namespace).Create(build)
					require.Nil(t, err)

					updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					})
					if firstBuild == nil {
						firstBuild = build
					}
					image.Status.BuildCounter++
					image.Status.LastBuildRef = build.Name
					_, err = fakeClient.BuildV1alpha1().Images(namespace).UpdateStatus(image)
					require.NoError(t, err)
				}
			})

			it("removes failed builds over the limit", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				buildList, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
				require.NoError(t, err)

				require.Len(t, buildList.Items, int(failedBuildHistoryLimit))

				_, err = fakeClient.BuildV1alpha1().Builds(namespace).Get(firstBuild.Name, v1.GetOptions{})
				require.Error(t, err, "not found")
			})

		})

		when("success builds have exceeded the successHistoryLimit", func() {
			var firstBuild *v1alpha1.Build
			it.Before(func() {
				_, err := fakeClient.BuildV1alpha1().Images(namespace).Create(image)
				require.Nil(t, err)

				for i := int64(0); i < successBuildHistoryLimit+1; i++ {
					build := image.CreateBuild(resolvedSourceResolverForImage(image), builder)
					build.ObjectMeta.CreationTimestamp = v1.NewTime(time.Now().Add(time.Duration(i) * time.Minute))
					_, err = fakeClient.BuildV1alpha1().Builds(namespace).Create(build)
					require.Nil(t, err)

					if firstBuild == nil {
						firstBuild = build
					}

					updateStatusOfLastBuild(t, fakeClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					image.Status.BuildCounter++
					image.Status.LastBuildRef = build.Name
					_, err = fakeClient.BuildV1alpha1().Images(namespace).UpdateStatus(image)
					require.NoError(t, err)
				}
			})

			it("removes success builds over the limit", func() {

				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				buildList, err := fakeClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
				require.NoError(t, err)

				require.Len(t, buildList.Items, int(successBuildHistoryLimit))

				_, err = fakeClient.BuildV1alpha1().Builds(namespace).Get(firstBuild.Name, v1.GetOptions{})
				require.Error(t, err, "not found")
			})
		})

		it("does not return error on nonexistent image", func() {
			err := reconciler.Reconcile(context.TODO(), "not/found")
			require.NoError(t, err)
		})

	})
}

func resolveImageSource(t *testing.T, fakeClient *fake.Clientset, namespace string) {
	sourceResolvers, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).List(v1.ListOptions{})
	require.NoError(t, err)
	require.Len(t, sourceResolvers.Items, 1)
	resolver := sourceResolvers.Items[0]

	_, err = fakeClient.BuildV1alpha1().SourceResolvers(namespace).UpdateStatus(&v1alpha1.SourceResolver{
		ObjectMeta: resolver.ObjectMeta,
		Spec:       resolver.Spec, // fake client overwrites spec :(
		Status: v1alpha1.SourceResolverStatus{
			Status: alpha1.Status{
				Conditions: alpha1.Conditions{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
			ResolvedSource: v1alpha1.ResolvedSource{
				Git: v1alpha1.ResolvedGitSource{
					URL:      resolver.Spec.Source.Git.URL,
					Revision: resolver.Spec.Source.Git.Revision,
					Type:     v1alpha1.Commit,
				},
			},
		},
	})
	require.NoError(t, err)
}

func updateStatusOfLastBuild(t *testing.T, fakeImageClient *fake.Clientset, namespace string, buildMetadata v1alpha1.BuildpackMetadataList, condition duckv1alpha1.Condition) v1alpha1.Build {
	build, err := fakeImageClient.BuildV1alpha1().Builds(namespace).List(v1.ListOptions{})
	require.NoError(t, err)
	var itemList []*v1alpha1.Build
	for _, value := range build.Items {
		itemList = append(itemList, &value)
	}
	require.NotEmpty(t, itemList)
	sort.Sort(v1build.ByCreationTimestamp(itemList))

	lastBuild := itemList[len(itemList)-1]
	_, err = fakeImageClient.BuildV1alpha1().Builds(namespace).UpdateStatus(&v1alpha1.Build{
		ObjectMeta: lastBuild.ObjectMeta,
		Spec:       lastBuild.Spec, // fake client overwrites spec :(
		Status: v1alpha1.BuildStatus{
			Status: alpha1.Status{
				Conditions: alpha1.Conditions{
					condition,
				},
			},
			BuildMetadata: buildMetadata,
		},
	})
	require.NoError(t, err)
	return *lastBuild
}

func resolvedSourceResolverForImage(image *v1alpha1.Image) *v1alpha1.SourceResolver {
	return &v1alpha1.SourceResolver{
		ObjectMeta: v1.ObjectMeta{
			Name: "some--name",
		},
		Spec: v1alpha1.SourceResolverSpec{
			ServiceAccount: image.Spec.ServiceAccount,
			Source:         image.Spec.Source,
		},
		Status: v1alpha1.SourceResolverStatus{
			Status: alpha1.Status{
				Conditions: alpha1.Conditions{
					{
						Type:   duckv1alpha1.ConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
			ResolvedSource: v1alpha1.ResolvedSource{
				Git: v1alpha1.ResolvedGitSource{
					URL:      image.Spec.Source.Git.URL,
					Revision: image.Spec.Source.Git.Revision,
					Type:     v1alpha1.Commit,
				},
			},
		},
	}
}

type fakeTracker map[corev1.ObjectReference]map[string]struct{}

func (f fakeTracker) Track(ref corev1.ObjectReference, obj interface{}) error {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	_, ok := f[ref]
	if !ok {
		f[ref] = map[string]struct{}{}
	}

	f[ref][key] = struct{}{}
	return nil
}

func (fakeTracker) OnChanged(obj interface{}) {
	panic("I should not be called in tests")
}

func (f fakeTracker) IsTracking(ref corev1.ObjectReference, obj interface{}) bool {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return false
	}

	trackingObs, ok := f[ref]
	if !ok {
		return false
	}
	_, ok = trackingObs[key]

	return ok
}
