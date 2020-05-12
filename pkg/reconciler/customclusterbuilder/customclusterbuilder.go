package customclusterbuilder

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
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

func NewController(
	opt reconciler.Options,
	informer v1alpha1informers.CustomClusterBuilderInformer,
	repoFactory NewBuildpackRepository,
	builderCreator BuilderCreator,
	keychainFactory registry.KeychainFactory,
	storeInformer v1alpha1informers.StoreInformer,
	stackInformer v1alpha1informers.StackInformer,
) *controller.Impl {
	c := &Reconciler{
		Client:                     opt.Client,
		CustomClusterBuilderLister: informer.Lister(),
		RepoFactory:                repoFactory,
		BuilderCreator:             builderCreator,
		KeychainFactory:            keychainFactory,
		StoreLister:                storeInformer.Lister(),
		StackLister:                stackInformer.Lister(),
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	informer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())
	storeInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))
	stackInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))

	return impl
}

type Reconciler struct {
	Client                     versioned.Interface
	CustomClusterBuilderLister v1alpha1Listers.CustomClusterBuilderLister
	RepoFactory                NewBuildpackRepository
	BuilderCreator             BuilderCreator
	KeychainFactory            registry.KeychainFactory
	Tracker                    reconciler.Tracker
	StoreLister                v1alpha1Listers.StoreLister
	StackLister                v1alpha1Listers.StackLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	customBuilder, err := c.CustomClusterBuilderLister.Get(builderName)
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

func (c *Reconciler) reconcileCustomBuilder(customBuilder *expv1alpha1.CustomClusterBuilder) (v1alpha1.BuilderRecord, error) {
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

	if !stack.Status.GetCondition(corev1alpha1.ConditionReady).IsTrue() {
		return v1alpha1.BuilderRecord{}, errors.Errorf("stack %s is not ready", stack.Name)
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		ServiceAccount: customBuilder.Spec.ServiceAccountRef.Name,
		Namespace:      customBuilder.Spec.ServiceAccountRef.Namespace,
	})
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	return c.BuilderCreator.CreateBuilder(keychain, c.RepoFactory(store), stack, customBuilder.Spec.CustomBuilderSpec)
}

func (c *Reconciler) updateStatus(desired *expv1alpha1.CustomClusterBuilder) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.CustomClusterBuilderLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.ExperimentalV1alpha1().CustomClusterBuilders().UpdateStatus(desired)
	return err
}
