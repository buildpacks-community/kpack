package builder

import (
	"context"

	"github.com/knative/pkg/controller"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/cnb"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	ReconcilerName = "Builders"
	Kind           = "Builder"
)

type MetadataRetriever interface {
	GetBuilderBuildpacks(repo registry.ImageRef) (cnb.BuilderMetadata, error)
}

func NewController(opt reconciler.Options, builderInformer v1alpha1informers.BuilderInformer, metadataRetriever MetadataRetriever) *controller.Impl {
	c := &Reconciler{
		Client:            opt.Client,
		MetadataRetriever: metadataRetriever,
		BuilderLister:     builderInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	builderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

type Reconciler struct {
	Client            versioned.Interface
	MetadataRetriever MetadataRetriever
	BuilderLister     v1alpha1Listers.BuilderLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	builder, err := c.BuilderLister.Builders(namespace).Get(builderName)
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

	_, err = c.Client.BuildV1alpha1().Builders(namespace).UpdateStatus(builder)

	return err
}

func transform(in cnb.BuilderMetadata) v1alpha1.BuildpackMetadataList {
	out := make(v1alpha1.BuildpackMetadataList, 0, len(in))

	for _, m := range in {
		out = append(out, v1alpha1.BuildpackMetadata{
			ID:      m.ID,
			Version: m.Version,
		})
	}

	return out
}
