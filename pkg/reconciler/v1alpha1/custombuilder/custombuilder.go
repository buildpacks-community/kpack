package custombuilder

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/kpack/pkg/client/informers/externalversions/experimental/v1alpha1"
	v1alpha1Listers "github.com/pivotal/kpack/pkg/client/listers/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/tracker"
)

const (
	ReconcilerName = "CustomBuilders"
	Kind           = "CustomBuilder"
)

type NewBuildpackRepository func(store *expv1alpha1.Store) cnb.BuildpackRepository

type BuilderCreator interface {
	CreateBuilder(keychain authn.Keychain, buildpackRepo cnb.BuildpackRepository, stack *expv1alpha1.Stack, spec expv1alpha1.CustomBuilderSpec) (v1alpha1.BuilderRecord, error)
}

func NewController(opt reconciler.Options,
	customBuilderInformer v1alpha1informers.CustomBuilderInformer,
	repoFactory NewBuildpackRepository,
	builderCreator BuilderCreator,
	keychainFactory registry.KeychainFactory,
	storeInformer v1alpha1informers.StoreInformer,
	stackInformer v1alpha1informers.StackInformer,
) *controller.Impl {
	c := &Reconciler{
		Client:              opt.Client,
		CustomBuilderLister: customBuilderInformer.Lister(),
		RepoFactory:         repoFactory,
		BuilderCreator:      builderCreator,
		KeychainFactory:     keychainFactory,
		StoreLister:         storeInformer.Lister(),
		StackLister:         stackInformer.Lister(),
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	customBuilderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())
	storeInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))
	stackInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))

	return impl
}

type Reconciler struct {
	Client              versioned.Interface
	CustomBuilderLister v1alpha1Listers.CustomBuilderLister
	RepoFactory         NewBuildpackRepository
	BuilderCreator      BuilderCreator
	KeychainFactory     registry.KeychainFactory
	Tracker             reconciler.Tracker
	StoreLister         v1alpha1Listers.StoreLister
	StackLister         v1alpha1Listers.StackLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	customBuilder, err := c.CustomBuilderLister.CustomBuilders(namespace).Get(builderName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	customBuilder = customBuilder.DeepCopy()

	builderRecord, creationError := c.reconcileCustomBuilder(customBuilder)
	if creationError != nil {
		customBuilder.Status.ErrorCreate(creationError)

		err := c.updateStatus(customBuilder)
		if err != nil {
			return err
		}

		return controller.NewPermanentError(creationError)
	}

	customBuilder.Status.BuilderRecord(builderRecord)
	return c.updateStatus(customBuilder)
}

func (c *Reconciler) reconcileCustomBuilder(customBuilder *expv1alpha1.CustomBuilder) (v1alpha1.BuilderRecord, error) {
	store, err := c.StoreLister.Get(customBuilder.Spec.Store)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	err = c.Tracker.Track(store, customBuilder.NamespacedName())
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	stack, err := c.StackLister.Get(customBuilder.Spec.Stack)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	err = c.Tracker.Track(stack, customBuilder.NamespacedName())
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		ServiceAccount: customBuilder.Spec.ServiceAccount,
		Namespace:      customBuilder.Namespace,
	})
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	return c.BuilderCreator.CreateBuilder(keychain, c.RepoFactory(store), stack, customBuilder.Spec.CustomBuilderSpec)
}

func (c *Reconciler) updateStatus(desired *expv1alpha1.CustomBuilder) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.CustomBuilderLister.CustomBuilders(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.ExperimentalV1alpha1().CustomBuilders(desired.Namespace).UpdateStatus(desired)
	return err
}
