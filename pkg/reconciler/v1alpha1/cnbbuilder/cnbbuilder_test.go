package cnbbuilder_test

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
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbbuilder"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbbuilder/cnbbuilderfakes"
	"github.com/pivotal/build-service-system/pkg/registry"
)

//go:generate counterfeiter . MetadataRetriever

func TestCNBBuildReconciler(t *testing.T) {
	spec.Run(t, "CNBBuilder Reconciler", testCNBBuilderReconciler)
}

func testCNBBuilderReconciler(t *testing.T, when spec.G, it spec.S) {
	fakeMetadataRetriever := &cnbbuilderfakes.FakeMetadataRetriever{}
	fakeCnbClient := fake.NewSimpleClientset(&v1alpha1.CNBBuilder{})

	cnbInformerFactory := externalversions.NewSharedInformerFactory(fakeCnbClient, time.Second)
	cnbBuilderInformer := cnbInformerFactory.Build().V1alpha1().CNBBuilders()

	reconciler := testhelpers.SyncWaitingReconciler(
		&cnbbuilder.Reconciler{
			CNBClient:         fakeCnbClient,
			CNBBuilderLister:  cnbBuilderInformer.Lister(),
			MetadataRetriever: fakeMetadataRetriever,
		},
		cnbBuilderInformer.Informer().HasSynced,
	)

	stopChan := make(chan struct{})

	it.Before(func() {
		cnbInformerFactory.Start(stopChan)
	})

	it.After(func() {
		close(stopChan)
	})

	const builderName = "cnb-builder-name"
	const namespace = "some-namespace"
	const key = "some-namespace/cnb-builder-name"
	const initalGeneration int64 = 1
	const imageName = "some/builder@sha256acf123"

	cnbBuilder := &v1alpha1.CNBBuilder{
		ObjectMeta: v1.ObjectMeta{
			Name:       builderName,
			Generation: initalGeneration,
		},
		Spec: v1alpha1.CNBBuilderSpec{
			Image: imageName,
		},
	}

	when("#Reconcile", func() {
		it.Before(func() {
			_, err := fakeCnbClient.BuildV1alpha1().CNBBuilders(namespace).Create(cnbBuilder)
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

			builder, err := fakeCnbClient.BuildV1alpha1().CNBBuilders(namespace).Get(builderName, v1.GetOptions{})
			assert.Nil(t, err)

			assert.Equal(t, builder.Status.BuilderMetadata,
				v1alpha1.CNBBuildpackMetadataList{
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

			builder, err := fakeCnbClient.BuildV1alpha1().CNBBuilders(namespace).Get(builderName, v1.GetOptions{})
			assert.Nil(t, err)

			assert.Equal(t, builder.Status.ObservedGeneration, initalGeneration)
		})

		it("does not return error on nonexistent builder", func() {
			err := reconciler.Reconcile(context.TODO(), "not/found")
			assert.Nil(t, err)
		})
	})
}
