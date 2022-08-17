package sourceresolver

import (
	"context"
	"errors"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging/logkey"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "SourceResolvers"
)

//go:generate counterfeiter . Resolver
type Resolver interface {
	Resolve(context.Context, *buildapi.SourceResolver) (corev1alpha1.ResolvedSourceConfig, error)
	CanResolve(*buildapi.SourceResolver) bool
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	sourceResolverInformer buildinformers.SourceResolverInformer,
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

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.SourceResolverCRName),
	)

	impl := controller.NewContext(ctx, c, controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger})

	c.Enqueuer = &workQueueEnqueuer{
		enqueueAfter: impl.EnqueueAfter,
		delay:        opt.SourcePollingFrequency,
	}

	sourceResolverInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

//go:generate counterfeiter . Enqueuer
type Enqueuer interface {
	Enqueue(*buildapi.SourceResolver) error
}

type Reconciler struct {
	GitResolver          Resolver
	BlobResolver         Resolver
	RegistryResolver     Resolver
	Enqueuer             Enqueuer
	Client               versioned.Interface
	SourceResolverLister buildlisters.SourceResolverLister
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

	resolvedSource, err := sourceReconciler.Resolve(ctx, sourceResolver)
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

func (c *Reconciler) sourceReconciler(sourceResolver *buildapi.SourceResolver) (Resolver, error) {
	if c.GitResolver.CanResolve(sourceResolver) {
		return c.GitResolver, nil
	} else if c.BlobResolver.CanResolve(sourceResolver) {
		return c.BlobResolver, nil
	} else if c.RegistryResolver.CanResolve(sourceResolver) {
		return c.RegistryResolver, nil
	}
	return nil, errors.New("invalid source type")
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *buildapi.SourceResolver) error {
	original, err := c.SourceResolverLister.SourceResolvers(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().SourceResolvers(desired.Namespace).UpdateStatus(ctx, desired, v1.UpdateOptions{})
	return err
}
