package builder_test

import (
	"context"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/cnb"
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
	fakeEnqueuer := &builderfakes.FakeEnqueuer{}

	reconciler := testhelpers.SyncWaitingReconciler(
		informerFactory,
		&builder.Reconciler{
			Client:            fakeClient,
			BuilderLister:     builderInformer.Lister(),
			MetadataRetriever: fakeMetadataRetriever,
			Enqueuer:          fakeEnqueuer,
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
			require.Nil(t, err)

			fakeMetadataRetriever.GetBuilderBuildpacksReturns(cnb.BuilderMetadata{
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

		it("schedule next polling when update policy is not set", func() {
			err := reconciler.Reconcile(context.TODO(), key)
			assert.Nil(t, err)

			assert.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
		})

		it("does schedule polling when update policy is set to polling", func() {
			builder.Spec.UpdatePolicy = v1alpha1.Polling
			_, err := fakeClient.BuildV1alpha1().Builders(namespace).Update(builder)
			require.Nil(t, err)

			err = reconciler.Reconcile(context.TODO(), key)
			require.Nil(t, err)

			assert.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
		})

		it("does not schedule polling when update policy is set to external", func() {
			builder.Spec.UpdatePolicy = v1alpha1.Webhook
			_, err := fakeClient.BuildV1alpha1().Builders(namespace).Update(builder)
			require.Nil(t, err)

			err = reconciler.Reconcile(context.TODO(), key)
			require.Nil(t, err)

			assert.Equal(t, 0, fakeEnqueuer.EnqueueCallCount())
		})

		it("does not return error on nonexistent builder", func() {
			err := reconciler.Reconcile(context.TODO(), "not/found")
			assert.Nil(t, err)
		})
	})
}
