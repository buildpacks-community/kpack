package cnbimage_test

import (
	"context"
	alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/kmeta"
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbimage"
	"github.com/sclevine/spec"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCNBImageReconciler(t *testing.T) {
	spec.Run(t, "CNBImage Reconciler", testCNBImageReconciler)
}

func testCNBImageReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeCnbImageClient := fake.NewSimpleClientset(&v1alpha1.CNBImage{}, &v1alpha1.CNBBuild{})

	cnbInformerFactory := externalversions.NewSharedInformerFactory(fakeCnbImageClient, time.Millisecond)
	cnbImageInformer := cnbInformerFactory.Build().V1alpha1().CNBImages()
	cnbBuildInformer := cnbInformerFactory.Build().V1alpha1().CNBBuilds()

	reconciler := testhelpers.SyncWaitingReconciler(
		&cnbimage.Reconciler{
			CNBClient:      fakeCnbImageClient,
			CNBImageLister: cnbImageInformer.Lister(),
			CNBBuildLister: cnbBuildInformer.Lister(),
		},
		cnbImageInformer.Informer().HasSynced,
		cnbBuildInformer.Informer().HasSynced,
	)
	stopChan := make(chan struct{})

	it.Before(func() {
		cnbInformerFactory.Start(stopChan)
	})

	it.After(func() {
		close(stopChan)
	})

	const imageName = "cnb-image-name"
	const namespace = "some-namespace"
	const key = "some-namespace/cnb-image-name"
	const originalGeneration int64 = 1

	cnbImage := &v1alpha1.CNBImage{
		ObjectMeta: v1.ObjectMeta{
			Name:       imageName,
			Generation: originalGeneration,
		},
		Spec: v1alpha1.CNBImageSpec{
			Image:          "some/image",
			ServiceAccount: "service-account",
			Builder:        "some/builder@sha256acf123",
			GitURL:         "https://some.git/url",
			GitRevision:    "revision",
		},
	}

	when("#Reconcile", func() {
		when("new image", func() {
			it.Before(func() {
				_, err := fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Create(cnbImage)
				assert.Nil(t, err)
			})

			it("creates an initial CNBBuild", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				assert.Nil(t, err)

				build, err := fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
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
				updatedImage, err := fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
				assert.Nil(t, err)
				assert.Equal(t, updatedImage.Status.LastBuildRef, buildName)
			})

			it("updates the observed generation with the new spec", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				assert.Nil(t, err)

				updatedImage, err := fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
				assert.Nil(t, err)
				assert.Equal(t, updatedImage.Status.ObservedGeneration, originalGeneration)
			})
		})

		when("a build has already been created", func() {
			it.Before(func() {
				_, err := fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Create(cnbImage)
				assert.Nil(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				assert.Nil(t, err)
			})

			when("a new spec is applied", func() {
				const newGeneration int64 = 2

				it.Before(func() {
					image, err := fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
					assert.Nil(t, err)

					_, err = fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Update(&v1alpha1.CNBImage{
						ObjectMeta: v1.ObjectMeta{
							Name:       imageName,
							Generation: newGeneration,
						},
						Spec: v1alpha1.CNBImageSpec{
							Image:          "different/image",
							ServiceAccount: "different/service-account",
							Builder:        "different/builder@sha256acf123",
							GitURL:         "https://different.git/url",
							GitRevision:    "differentrevision",
						},
						Status: image.Status, // fake client overwrites status :(
					})
					assert.Nil(t, err)
				})

				it("does not create a build when a build is running", func() {
					updateStatusOfLastBuild(t, fakeCnbImageClient, namespace, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 1)

					updatedImage, err := fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
					assert.Nil(t, err)
					assert.Equal(t, updatedImage.Status.ObservedGeneration, originalGeneration)
				})

				it("does create a build when the last build is no longer running", func() {
					updateStatusOfLastBuild(t, fakeCnbImageClient, namespace, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 2)

					updatedImage, err := fakeCnbImageClient.BuildV1alpha1().CNBImages(namespace).Get(imageName, v1.GetOptions{})
					assert.Nil(t, err)
					assert.Equal(t, updatedImage.Status.ObservedGeneration, newGeneration)

					newBuild, err := fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).Get(updatedImage.Status.LastBuildRef, v1.GetOptions{})
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
							Builder:        "different/builder@sha256acf123",
							GitURL:         "https://different.git/url",
							GitRevision:    "differentrevision",
						},
					})
				})
			})

			when("no new spec has been applied", func() {
				it("does not create a new build when last build is running", func() {
					updateStatusOfLastBuild(t, fakeCnbImageClient, namespace, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionFalse,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})

				it("does not create a build when the last build is no longer running", func() {
					updateStatusOfLastBuild(t, fakeCnbImageClient, namespace, duckv1alpha1.Condition{
						Type:   duckv1alpha1.ConditionSucceeded,
						Status: corev1.ConditionTrue,
					})

					err := reconciler.Reconcile(context.TODO(), key)
					assert.Nil(t, err)

					builds, err := fakeCnbImageClient.BuildV1alpha1().CNBBuilds(namespace).List(v1.ListOptions{})
					assert.Nil(t, err)
					assert.Equal(t, len(builds.Items), 1)
				})
			})

		})
	})
}

func updateStatusOfLastBuild(t *testing.T, fakeCnbImageClient *fake.Clientset, namespace string, condition duckv1alpha1.Condition) {
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
		},
	})
	assert.Nil(t, err)
}
