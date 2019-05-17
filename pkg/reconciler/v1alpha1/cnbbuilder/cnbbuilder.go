package cnbbuilder

import (
	"context"

	"github.com/knative/pkg/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	ReconcilerName = "CNBBuilders"
	Kind           = "CNBBuilder"
)

type MetadataRetriever interface {
	GetBuilderBuildpacks(repo registry.ImageRef) (registry.BuilderMetadata, error)
}

func NewController(opt reconciler.Options, cnbBuilderInformer v1alpha1informers.CNBBuilderInformer, metadataRetriever MetadataRetriever) *controller.Impl {
	c := &Reconciler{
		CNBClient:         opt.CNBClient,
		MetadataRetriever: metadataRetriever,
		CNBBuilderLister:  cnbBuilderInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	cnbBuilderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

type Reconciler struct {
	CNBClient         versioned.Interface
	MetadataRetriever MetadataRetriever
	CNBBuilderLister  v1alpha1Listers.CNBBuilderLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	builder, err := c.CNBBuilderLister.CNBBuilders(namespace).Get(builderName)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	builder.DeepCopy()

	metadata, err := c.MetadataRetriever.GetBuilderBuildpacks(registry.NewNoAuthImageRef(builder.Spec.Image))
	if err != nil {
		return err
	}

	builder.Status.BuilderMetadata = transform(metadata)
	builder.Status.ObservedGeneration = builder.Generation

	_, err = c.CNBClient.BuildV1alpha1().CNBBuilders(namespace).UpdateStatus(builder)

	return err
}

func transform(in registry.BuilderMetadata) v1alpha1.CNBBuildpackMetadataList {
	out := make(v1alpha1.CNBBuildpackMetadataList, 0, len(in))

	for _, m := range in {
		out = append(out, v1alpha1.CNBBuildpackMetadata{
			ID:      m.ID,
			Version: m.Version,
		})
	}

	return out
}
