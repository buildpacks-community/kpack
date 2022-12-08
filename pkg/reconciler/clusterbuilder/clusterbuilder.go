package clusterBuilder

import (
	"context"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging/logkey"

	"github.com/google/go-containerregistry/pkg/authn"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/tracker"
)

const (
	ReconcilerName = "ClusterBuilders"
)

type BuilderCreator interface {
	CreateBuilder(keychain authn.Keychain, clusterStore *buildapi.ClusterStore, clusterStack *buildapi.ClusterStack, spec buildapi.BuilderSpec) (buildapi.BuilderRecord, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	clusterBuilderInformer buildinformers.ClusterBuilderInformer,
	builderCreator BuilderCreator,
	keychainFactory registry.KeychainFactory,
	clusterStoreInformer buildinformers.ClusterStoreInformer,
	clusterStackInformer buildinformers.ClusterStackInformer,
) (*controller.Impl, func()) {
	c := &Reconciler{
		Client:               opt.Client,
		ClusterBuilderLister: clusterBuilderInformer.Lister(),
		BuilderCreator:       builderCreator,
		KeychainFactory:      keychainFactory,
		ClusterStoreLister:   clusterStoreInformer.Lister(),
		ClusterStackLister:   clusterStackInformer.Lister(),
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ClusterBuilderCRName),
	)

	impl := controller.NewContext(
		ctx,
		&reconciler.NetworkErrorReconciler{
			Reconciler: c,
		},
		controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger},
	)
	clusterBuilderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())
	clusterStoreInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))
	clusterStackInformer.Informer().AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))

	return impl, func() {
		impl.GlobalResync(clusterBuilderInformer.Informer())
	}
}

type Reconciler struct {
	Client               versioned.Interface
	ClusterBuilderLister buildlisters.ClusterBuilderLister
	BuilderCreator       BuilderCreator
	KeychainFactory      registry.KeychainFactory
	Tracker              reconciler.Tracker
	ClusterStoreLister   buildlisters.ClusterStoreLister
	ClusterStackLister   buildlisters.ClusterStackLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	builder, err := c.ClusterBuilderLister.Get(builderName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	builder = builder.DeepCopy()

	builderRecord, creationError := c.reconcileBuilder(ctx, builder)
	if creationError != nil {
		builder.Status.ErrorCreate(creationError)

		err := c.updateStatus(ctx, builder)
		if err != nil {
			return err
		}

		return creationError
	}

	builder.Status.BuilderRecord(builderRecord)
	return c.updateStatus(ctx, builder)
}

func (c *Reconciler) reconcileBuilder(ctx context.Context, builder *buildapi.ClusterBuilder) (buildapi.BuilderRecord, error) {
	err := c.Tracker.Track(reconciler.Key{
		NamespacedName: types.NamespacedName{
			Name:      builder.Spec.Store.Name,
			Namespace: v1.NamespaceAll,
		},
		GroupKind: schema.GroupKind{
			Group: "kpack.io",
			Kind:  buildapi.ClusterStoreKind,
		},
	}, builder.NamespacedName())
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	err = c.Tracker.Track(reconciler.Key{
		NamespacedName: types.NamespacedName{
			Name:      builder.Spec.Stack.Name,
			Namespace: v1.NamespaceAll,
		},
		GroupKind: schema.GroupKind{
			Group: "kpack.io",
			Kind:  buildapi.ClusterStackKind,
		},
	}, builder.NamespacedName())
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	clusterStore, err := c.ClusterStoreLister.Get(builder.Spec.Store.Name)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	clusterStack, err := c.ClusterStackLister.Get(builder.Spec.Stack.Name)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	if !clusterStack.Status.GetCondition(corev1alpha1.ConditionReady).IsTrue() {
		return buildapi.BuilderRecord{}, reconciler.NewNotReadyError("stack %s is not ready", clusterStack.Name)
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		ServiceAccount: builder.Spec.ServiceAccountRef.Name,
		Namespace:      builder.Spec.ServiceAccountRef.Namespace,
	})
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	return c.BuilderCreator.CreateBuilder(keychain, clusterStore, clusterStack, builder.Spec.BuilderSpec)
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *buildapi.ClusterBuilder) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.ClusterBuilderLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().ClusterBuilders().UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}
