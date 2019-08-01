package sourceresolver

import (
	"context"

	"github.com/knative/pkg/controller"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler"
)

const (
	ReconcilerName = "SourceResolvers"
	Kind           = "SourceResolver"
)

//go:generate counterfeiter . Resolver
type Resolver interface {
	Resolve(sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSource, error)
}

func NewController(opt reconciler.Options, sourceResolverInformer v1alpha1informers.SourceResolverInformer, gitResolver Resolver, blobResolver Resolver) *controller.Impl {
	c := &Reconciler{
		GitResolver:          gitResolver,
		BlobResolver:         blobResolver,
		Client:               opt.Client,
		SourceResolverLister: sourceResolverInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	c.Enqueuer = &workQueueEnqueuer{
		enqueueAfter: impl.EnqueueAfter,
		delay:        opt.SourcePollingFrequency,
	}

	sourceResolverInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

//go:generate counterfeiter . Enqueuer
type Enqueuer interface {
	Enqueue(*v1alpha1.SourceResolver) error
}

type Reconciler struct {
	GitResolver          Resolver
	BlobResolver         Resolver
	Enqueuer             Enqueuer
	Client               versioned.Interface
	SourceResolverLister v1alpha1listers.SourceResolverLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, sourceResolverName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	sourceResolver, err := c.SourceResolverLister.SourceResolvers(namespace).Get(sourceResolverName)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	sourceResolver = sourceResolver.DeepCopy()

	if sourceResolver.IsGit() {
		resolvedSource, err := c.GitResolver.Resolve(sourceResolver)
		if err != nil {
			return err
		}

		sourceResolver.ResolvedGitSource(resolvedSource.Git)
	} else if sourceResolver.IsBlob() {
		resolvedSource, err := c.BlobResolver.Resolve(sourceResolver)
		if err != nil {
			return err
		}

		sourceResolver.ResolvedBlobSource(resolvedSource.Blob)
	}

	if sourceResolver.PollingReady() {
		err := c.Enqueuer.Enqueue(sourceResolver)
		if err != nil {
			return err
		}
	}

	sourceResolver.Status.ObservedGeneration = sourceResolver.Generation
	return c.updateStatus(sourceResolver)
}

func (c *Reconciler) updateStatus(desired *v1alpha1.SourceResolver) error {
	original, err := c.SourceResolverLister.SourceResolvers(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.BuildV1alpha1().SourceResolvers(desired.Namespace).UpdateStatus(desired)
	return err
}
