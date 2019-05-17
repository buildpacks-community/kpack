package cnbimage_test

import (
	"context"
	"testing"
	"time"

	alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/kmeta"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbimage"
)

func TestCNBImageReconciler(t *testing.T) {
	spec.Run(t, "CNBImage Reconciler", testCNBImageReconciler)
}

func testCNBImageReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeCnbClient := fake.NewSimpleClientset(&v1alpha1.CNBImage{}, &v1alpha1.CNBBuild{}, &v1alpha1.CNBBuilder{})

	cnbInformerFactory := externalversions.NewSharedInformerFactory(fakeCnbClient, time.Second)
	cnbImageInformer := cnbInformerFactory.Build().V1alpha1().CNBImages()
	cnbBuildInformer := cnbInformerFactory.Build().V1alpha1().CNBBuilds()
	cnbBuilderInformer := cnbInformerFactory.Build().V1alpha1().CNBBuilders()

	fakeTracker := fakeTracker{}

	reconciler := testhelpers.SyncWaitingReconciler(
		&cnbimage.Reconciler{
			CNBClient:        fakeCnbClient,
			CNBImageLister:   cnbImageInformer.Lister(),
			CNBBuildLister:   cnbBuildInformer.Lister(),
			CNBBuilderLister: cnbBuilderInformer.Lister(),
			Tracker:          fakeTracker,
		},
		cnbImageInformer.Informer().HasSynced,
		cnbBuildInformer.Informer().HasSynced,
		cnbBuilderInformer.Informer().HasSynced,
	)
	stopChan := make(chan struct{})

	const imageName = "cnb-image-name"
	const builderName = "cnb-builder-name"
	const namespace = "some-namespace"
	const key = "some-namespace/cnb-image-name"
	const originalGeneration int64 = 1

	cnbImage := &v1alpha1.CNBImage{
		ObjectMeta: v1.ObjectMeta{
			Name:       imageName,
			Namespace:  namespace,
			Generation: originalGeneration,
		},
		Spec: v1alpha1.CNBImageSpec{
			Image:          "some/image",
			ServiceAccount: "service-account",
			BuilderRef:     builderName,
			GitURL:         "https://some.git/url",
			GitRevision:    "revision",
		},
	}

	defaultBuildMetadata := v1alpha1.CNBBuildpackMetadataList{
		{
			ID:      "buildpack.version",
			Version: "version",
		},
	}

	cnbBuilder := &v1alpha1.CNBBuilder{
		ObjectMeta: v1.ObjectMeta{
			Name: builderName,
		},
		Spec: v1alpha1.CNBBuilderSpec{
			Image: "some/builder@sha256acf123",
		},
		Status: v1alpha1.CNBBuilderStatus{
			BuilderMetadata: defaultBuildMetadata,
		},
	}

	it.Before(func() {
		cnbInformerFactory.Start(stopChan)

		_, err := fakeCnbClient.BuildV1alpha1().CNBBuilders(namespace).Create(cnbBuilder)
		assert.Nil(t, err)
	})

	it.After(func() {
		close(stopChan)
	})

	when("#Reconcile", func() {
		when("new image", func() {
			it.Before(func() {
				_, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Create(cnbImage)
				assert.Nil(t, err)
			})

			it("creates an initial CNBBuild", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				assert.Nil(t, err)

				build, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
				assert.Nil(t, err)
				assert.Equal(t, len(build.Items), 1)

				buildName := build.Items[0].ObjectMeta.Name
				assert.Equal(t, build.Items[0], v1alpha1.CNBBuild{
					TypeMeta: v1.TypeMeta{},
					ObjectMeta: v1.ObjectMeta{
						Name:      buildName,
						Namespace: namespace,
						OwnerReferences: []v1.OwnerReference{
							*kmeta.NewControllerRef(cnbImage),
						},
					},
					Spec: v1alpha1.CNBBuildSpec{
						Image:          "some/image",
						Builder:        "some/builder@sha256acf123",
						ServiceAccount: "service-account",
						GitURL:         "https://some.git/url",
						GitRevision:    "revision",
					},
				})
				updatedImage, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
				assert.Nil(t, err)
				assert.Equal(t, updatedImage.Status.LastBuildRef, buildName)
			})

			it("updates the observed generation with the new spec", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				assert.Nil(t, err)

				updatedImage, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
				assert.Nil(t, err)
				assert.Equal(t, updatedImage.Status.ObservedGeneration, originalGeneration)
			})

			it("tracks the builder", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				assert.Nil(t, err)

				updatedImage, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
				assert.Nil(t, err)

				gvk := cnbBuilder.GetGroupVersionKind()
				isTracking := fakeTracker.IsTracking(corev1.ObjectReference{
					APIVersion: gvk.GroupVersion().String(),
					Kind:       gvk.Kind,
					Namespace:  namespace,
					Name:       builderName,
				}, updatedImage)

				assert.True(t, isTracking)
			})
		})

		when("a build has already been created", func() {
			it.Before(func() {
				_, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Create(cnbImage)
				assert.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				assert.Nil(t, err)
			})

			when("a new spec is applied", func() {
				const newGeneration int64 = 2

				it.Before(func() {
					image, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
					assert.Nil(t, err)

					_, err = fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Update(&v1alpha1.CNBImage{
						ObjectMeta: v1.ObjectMeta{
							Name:       imageName,
							Generation: newGeneration,
						},
						Spec: v1alpha1.CNBImageSpec{
							Image:          "different/image",
							ServiceAccount: "different/service-account",
							BuilderRef:     builderName,
							GitURL:         "https://different.git/url",
							GitRevision:    "differentrevision",
						},
						Status: image.Status, // fake client overwrites status :(
					})
					assert.Nil(t, err)
				})

				it("does not create a build when a build is running", func() {
					updateStatusOfLastBuild(t, fakeCnbClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 1)

					updatedImage, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
					assert.Nil(t, err)
					assert.Equal(t, updatedImage.Status.ObservedGeneration, originalGeneration)
				})

				it("does create a build when the last build is successful", func() {
					updateStatusOfLastBuild(t, fakeCnbClient, namespace, defaultBuildMetadata, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 2)

					updatedImage, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
					assert.Nil(t, err)
					assert.Equal(t, updatedImage.Status.ObservedGeneration, newGeneration)

					newBuild, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).Get(updatedImage.Status.LastBuildRef, v1.GetOptions{})
					assert.Nil(t, err)
					assert.Equal(t, newBuild, &v1alpha1.CNBBuild{
						TypeMeta: v1.TypeMeta{},
						ObjectMeta: v1.ObjectMeta{
							Name:      updatedImage.Status.LastBuildRef,
							Namespace: namespace,
							OwnerReferences: []v1.OwnerReference{
								*kmeta.NewControllerRef(cnbImage),
							},
						},
						Spec: v1alpha1.CNBBuildSpec{
							Image:          "different/image",
							ServiceAccount: "different/service-account",
							Builder:        "some/builder@sha256acf123",
							GitURL:         "https://different.git/url",
							GitRevision:    "differentrevision",
						},
					})
				})

				it("does create a build when the last build is a failure", func() {
					updateStatusOfLastBuild(t, fakeCnbClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 2)
				})
			})

			when("referenced builder has been updated", func() {
				it.Before(func() {
					_, err := fakeCnbClient.BuildV1alpha1().CNBBuilders(namespace).Update(&v1alpha1.CNBBuilder{
						ObjectMeta: v1.ObjectMeta{
							Name: builderName,
						},
						Spec: v1alpha1.CNBBuilderSpec{
							Image: "some/builder@sha256:newsha",
						},
						Status: v1alpha1.CNBBuilderStatus{
							BuilderMetadata: []v1alpha1.CNBBuildpackMetadata{
								{
									ID:      "new.buildpack",
									Version: "version",
								},
							},
						},
					})
					assert.Nil(t, err)
				})

				it("does not create a build when a build is running", func() {
					updateStatusOfLastBuild(t, fakeCnbClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})

				it("does create a build when the last build is no longer running", func() {
					updateStatusOfLastBuild(t, fakeCnbClient, namespace, defaultBuildMetadata, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 2)

					updatedImage, err := fakeCnbClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
					assert.Nil(t, err)

					newBuild, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).Get(updatedImage.Status.LastBuildRef, v1.GetOptions{})
					assert.Nil(t, err)
					assert.Equal(t, newBuild, &v1alpha1.CNBBuild{
						TypeMeta: v1.TypeMeta{},
						ObjectMeta: v1.ObjectMeta{
							Name:      updatedImage.Status.LastBuildRef,
							Namespace: namespace,
							OwnerReferences: []v1.OwnerReference{
								*kmeta.NewControllerRef(cnbImage),
							},
						},
						Spec: v1alpha1.CNBBuildSpec{
							Image:          "some/image",
							Builder:        "some/builder@sha256:newsha",
							ServiceAccount: "service-account",
							GitURL:         "https://some.git/url",
							GitRevision:    "revision",
						},
					})
				})
			})

			when("no new spec has been applied", func() {
				it("does not create a new build when last build is running", func() {
					updateStatusOfLastBuild(t, fakeCnbClient, namespace, nil, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionUnknown,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})

				it("does not create a build when the last build is no longer running", func() {
					updateStatusOfLastBuild(t, fakeCnbClient, namespace, defaultBuildMetadata, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})
			})

		})

		it("does not return error on nonexistent image", func() {
			err := reconciler.Reconcile(context.TODO(), "not/found")
			assert.Nil(t, err)
		})
	})
}

func updateStatusOfLastBuild(t *testing.T, fakeCnbImageClient *fake.Clientset, namespace string, buildMetadata v1alpha1.CNBBuildpackMetadataList, condition duckv1alpha1.Condition) {
	build, err := fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
	assert.Nil(t, err)
	assert.Equal(t, len(build.Items), 1)

	lastBuild := build.Items[0]
	_, err = fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).UpdateStatus(&v1alpha1.CNBBuild{
		ObjectMeta: v1.ObjectMeta{Name: lastBuild.Name},
		Spec:       lastBuild.Spec, //fake client overwrites spec :(
		Status: v1alpha1.CNBBuildStatus{
			Status: alpha1.Status{
				Conditions: alpha1.Conditions{
					condition,
				},
			},
			BuildMetadata: buildMetadata,
		},
	})
	assert.Nil(t, err)
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
