package builder

import (
	"context"

	"github.com/knative/pkg/controller"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-beam/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-beam/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/build-service-beam/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/build-service-beam/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-beam/pkg/cnb"
	"github.com/pivotal/build-service-beam/pkg/reconciler"
	"github.com/pivotal/build-service-beam/pkg/registry"
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

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	c.Enqueuer = &workQueueEnqueuer{
		enqueueAfter: impl.EnqueueAfter,
		delay:        opt.BuilderPollingFrequency,
	}

	builderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

//go:generate counterfeiter . Enqueuer
type Enqueuer interface {
	Enqueue(*v1alpha1.Builder) error
}

type Reconciler struct {
	Client            versioned.Interface
	MetadataRetriever MetadataRetriever
	BuilderLister     v1alpha1Listers.BuilderLister
	Enqueuer          Enqueuer
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
	builder = builder.DeepCopy()

	metadata, err := c.MetadataRetriever.GetBuilderBuildpacks(registry.NewNoAuthImageRef(builder.Spec.Image))
	if err != nil {
		return err
	}

	builder.Status.BuilderMetadata = transform(metadata)
	builder.Status.ObservedGeneration = builder.Generation

	err = c.updateStatus(builder)
	if err != nil {
		return err
	}

	if builder.Spec.UpdatePolicy != v1alpha1.External {
		err = c.Enqueuer.Enqueue(builder)
	}
	return err
}

func (c *Reconciler) updateStatus(desired *v1alpha1.Builder) error {
	original, err := c.BuilderLister.Builders(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status.BuilderMetadata, original.Status.BuilderMetadata) {
		return nil
	}

	_, err = c.Client.BuildV1alpha1().Builders(desired.Namespace).UpdateStatus(desired)
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
