package builder_test

import (
	"context"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/builder"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/builder/builderfakes"
	"github.com/pivotal/build-service-system/pkg/registry"
)

//go:generate counterfeiter . MetadataRetriever

func TestBuildReconciler(t *testing.T) {
	spec.Run(t, "Builder Reconciler", testBuilderReconciler)
}

func testBuilderReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeMetadataRetriever := &builderfakes.FakeMetadataRetriever{}
	fakeClient := fake.NewSimpleClientset(&v1alpha1.Builder{})

	informerFactory := externalversions.NewSharedInformerFactory(fakeClient, time.Second)
	builderInformer := informerFactory.Build().V1alpha1().Builders()

	reconciler := testhelpers.SyncWaitingReconciler(
		&builder.Reconciler{
			Client:            fakeClient,
			BuilderLister:     builderInformer.Lister(),
			MetadataRetriever: fakeMetadataRetriever,
		},
		builderInformer.Informer().HasSynced,
	)

	stopChan := make(chan struct{})

	it.Before(func() {
		informerFactory.Start(stopChan)
	})

	it.After(func() {
		close(stopChan)
	})

	const (
		builderName            = "builder-name"
		namespace              = "some-namespace"
		key                    = "some-namespace/builder-name"
		imageName              = "some/builder@sha256acf123"
		initalGeneration int64 = 1
	)

	builder := &v1alpha1.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name:       builderName,
			Generation: initalGeneration,
		},
		Spec: v1alpha1.BuilderSpec{
			Image: imageName,
		},
	}

	when("#Reconcile", func() {
		it.Before(func() {
			_, err := fakeClient.BuildV1alpha1().Builders(namespace).Create(builder)
			assert.Nil(t, err)

			fakeMetadataRetriever.GetBuilderBuildpacksReturns(registry.BuilderMetadata{
				{
					ID:      "buildpack.version",
					Version: "version",
				},
			}, nil)
		})

		it("fetches the metadata for the configured builder", func() {
			err := reconciler.Reconcile(context.TODO(), key)
			assert.Nil(t, err)

			builder, err := fakeClient.BuildV1alpha1().Builders(namespace).Get(builderName, v1.GetOptions{})
			assert.Nil(t, err)

			assert.Equal(t, builder.Status.BuilderMetadata,
				v1alpha1.BuildpackMetadataList{
					{
						ID:      "buildpack.version",
						Version: "version",
					},
				},
			)
			assert.Equal(t, fakeMetadataRetriever.GetBuilderBuildpacksCallCount(), 1)
			assert.Equal(t, fakeMetadataRetriever.GetBuilderBuildpacksArgsForCall(0), registry.NewNoAuthImageRef(imageName))
		})

		it("records the observed generation", func() {
			err := reconciler.Reconcile(context.TODO(), key)
			assert.Nil(t, err)

			builder, err := fakeClient.BuildV1alpha1().Builders(namespace).Get(builderName, v1.GetOptions{})
			assert.Nil(t, err)

			assert.Equal(t, builder.Status.ObservedGeneration, initalGeneration)
		})

		it("does not return error on nonexistent builder", func() {
			err := reconciler.Reconcile(context.TODO(), "not/found")
			assert.Nil(t, err)
		})
	})
}
