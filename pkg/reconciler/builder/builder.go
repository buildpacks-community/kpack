package builder

import (
	"context"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/tracker"
)

const (
	ReconcilerName = "Builders"
	Kind           = "Builder"
)

type NewBuildpackRepository func(clusterStore *v1alpha1.ClusterStore) cnb.BuildpackRepository

type BuilderCreator interface {
	CreateBuilder(keychain authn.Keychain, clusterStore *v1alpha1.ClusterStore, clusterStack *v1alpha1.ClusterStack, spec v1alpha1.BuilderSpec) (v1alpha1.BuilderRecord, error)
}

func NewController(opt reconciler.Options,
	builderInformer v1alpha1informers.BuilderInformer,
	builderCreator BuilderCreator,
	keychainFactory registry.KeychainFactory,
	clusterStoreInformer v1alpha1informers.ClusterStoreInformer,
	clusterStackInformer v1alpha1informers.ClusterStackInformer,
) (*controller.Impl, func()) {
	c := &Reconciler{
		Client:             opt.Client,
		BuilderLister:      builderInformer.Lister(),
		BuilderCreator:     builderCreator,
		KeychainFactory:    keychainFactory,
		ClusterStoreLister: clusterStoreInformer.Lister(),
		ClusterStackLister: clusterStackInformer.Lister(),
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	builderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())
	clusterStoreInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))
	clusterStackInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))

	return impl, func() {
		impl.GlobalResync(builderInformer.Informer())
	}
}

type Reconciler struct {
	Client             versioned.Interface
	BuilderLister      v1alpha1Listers.BuilderLister
	BuilderCreator     BuilderCreator
	KeychainFactory    registry.KeychainFactory
	Tracker            reconciler.Tracker
	ClusterStoreLister v1alpha1Listers.ClusterStoreLister
	ClusterStackLister v1alpha1Listers.ClusterStackLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	builder, err := c.BuilderLister.Builders(namespace).Get(builderName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	builder = builder.DeepCopy()

	builderRecord, creationError := c.reconcileBuilder(builder)
	if creationError != nil {
		builder.Status.ErrorCreate(creationError)

		err := c.updateStatus(ctx, builder)
		if err != nil {
			return err
		}

		return controller.NewPermanentError(creationError)
	}

	builder.Status.BuilderRecord(builderRecord)
	return c.updateStatus(ctx, builder)
}

func (c *Reconciler) reconcileBuilder(builder *v1alpha1.Builder) (v1alpha1.BuilderRecord, error) {
	clusterStore, err := c.ClusterStoreLister.Get(builder.Spec.Store.Name)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	err = c.Tracker.Track(clusterStore, builder.NamespacedName())
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	clusterStack, err := c.ClusterStackLister.Get(builder.Spec.Stack.Name)
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	err = c.Tracker.Track(clusterStack, builder.NamespacedName())
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	if !clusterStack.Status.GetCondition(corev1alpha1.ConditionReady).IsTrue() {
		return v1alpha1.BuilderRecord{}, errors.Errorf("stack %s is not ready", clusterStack.Name)
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		ServiceAccount: builder.Spec.ServiceAccount,
		Namespace:      builder.Namespace,
	})
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	return c.BuilderCreator.CreateBuilder(keychain, clusterStore, clusterStack, builder.Spec.BuilderSpec)
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Builder) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.BuilderLister.Builders(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha1().Builders(desired.Namespace).UpdateStatus(ctx, desired, v1.UpdateOptions{})
	return err
}
