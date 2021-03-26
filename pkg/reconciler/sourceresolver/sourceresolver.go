package sourceresolver

import (
	"context"
	"errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1listers "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/reconciler"
	"knative.dev/pkg/controller"
)

const (
	ReconcilerName = "SourceResolvers"
	Kind           = "SourceResolver"
)

//go:generate counterfeiter . Resolver
type Resolver interface {
	Resolve(sourceResolver *v1alpha1.SourceResolver) (v1alpha1.ResolvedSourceConfig, error)
	CanResolve(*v1alpha1.SourceResolver) bool
}

func NewController(
	opt reconciler.Options,
	sourceResolverInformer v1alpha1informers.SourceResolverInformer,
	gitResolver Resolver,
	blobResolver Resolver,
	registryResolver Resolver,
) *controller.Impl {
	c := &Reconciler{
		GitResolver:          gitResolver,
		BlobResolver:         blobResolver,
		RegistryResolver:     registryResolver,
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
	RegistryResolver     Resolver
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
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	sourceResolver = sourceResolver.DeepCopy()

	sourceReconciler, err := c.sourceReconciler(sourceResolver)
	if err != nil {
		return err
	}

	resolvedSource, err := sourceReconciler.Resolve(sourceResolver)
	if err != nil {
		return err
	}

	sourceResolver.ResolvedSource(resolvedSource)

	if sourceResolver.PollingReady() {
		err := c.Enqueuer.Enqueue(sourceResolver)
		if err != nil {
			return err
		}
	}

	sourceResolver.Status.ObservedGeneration = sourceResolver.Generation
	return c.updateStatus(ctx, sourceResolver)
}

func (c *Reconciler) sourceReconciler(sourceResolver *v1alpha1.SourceResolver) (Resolver, error) {
	if c.GitResolver.CanResolve(sourceResolver) {
		return c.GitResolver, nil
	} else if c.BlobResolver.CanResolve(sourceResolver) {
		return c.BlobResolver, nil
	} else if c.RegistryResolver.CanResolve(sourceResolver) {
		return c.RegistryResolver, nil
	}
	return nil, errors.New("invalid source type")
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.SourceResolver) error {
	original, err := c.SourceResolverLister.SourceResolvers(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha1().SourceResolvers(desired.Namespace).UpdateStatus(ctx, desired, v1.UpdateOptions{})
	return err
}
