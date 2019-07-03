package sourceresolver

import (
	"context"
	"github.com/knative/pkg/controller"
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/git"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

const (
	ReconcilerName = "SourceResolvers"
	Kind           = "SourceResolver"
)

//go:generate counterfeiter . GitResolver
type GitResolver interface {
	Resolve(auth git.Auth, gitSource v1alpha1.Git) (v1alpha1.ResolvedGitSource, error)
}

func NewController(opt reconciler.Options, sourceResolverInformer v1alpha1informers.SourceResolverInformer, gitResolver GitResolver, gitKeychain git.GitKeychain) *controller.Impl {
	c := &Reconciler{
		Client:               opt.Client,
		SourceResolverLister: sourceResolverInformer.Lister(),
		GitResolver:          gitResolver,
		GitKeychain:          gitKeychain,
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	sourceResolverInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

type Reconciler struct {
	GitResolver GitResolver
	GitKeychain git.GitKeychain

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

	auth, err := c.GitKeychain.Resolve(namespace, sourceResolver.Spec.ServiceAccount, sourceResolver.Spec.Source.Git)
	if err != nil {
		return err
	}

	resolvedGitSource, err := c.GitResolver.Resolve(auth, sourceResolver.Spec.Source.Git)
	if err != nil {
		return err
	}

	sourceResolver.ResolvedGitSource(resolvedGitSource)
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
