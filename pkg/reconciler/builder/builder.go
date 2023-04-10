package builder

import (
	"context"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/logging/logkey"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/tracker"
)

const (
	ReconcilerName = "Builders"
)

type BuilderCreator interface {
	CreateBuilder(ctx context.Context, keychain authn.Keychain, fetcher cnb.RemoteBuildpackFetcher, clusterStack *buildapi.ClusterStack, spec buildapi.BuilderSpec) (buildapi.BuilderRecord, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	builderInformer buildinformers.BuilderInformer,
	builderCreator BuilderCreator,
	keychainFactory registry.KeychainFactory,
	clusterStoreInformer buildinformers.ClusterStoreInformer,
	buildpackInformer buildinformers.BuildpackInformer,
	clusterBuildpackInformer buildinformers.ClusterBuildpackInformer,
	clusterStackInformer buildinformers.ClusterStackInformer,
) (*controller.Impl, func()) {
	c := &Reconciler{
		Client:                 opt.Client,
		BuilderLister:          builderInformer.Lister(),
		BuilderCreator:         builderCreator,
		KeychainFactory:        keychainFactory,
		ClusterStoreLister:     clusterStoreInformer.Lister(),
		BuildpackLister:        buildpackInformer.Lister(),
		ClusterBuildpackLister: clusterBuildpackInformer.Lister(),
		ClusterStackLister:     clusterStackInformer.Lister(),
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.BuilderCRName),
	)

	impl := controller.NewContext(
		ctx,
		&reconciler.NetworkErrorReconciler{
			Reconciler: c,
		},
		controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger},
	)
	builderInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())
	clusterStoreInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			c.Tracker.OnChanged,
			buildapi.SchemeGroupVersion.WithKind(buildapi.ClusterStoreKind)),
	))
	clusterStackInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			c.Tracker.OnChanged,
			buildapi.SchemeGroupVersion.WithKind(buildapi.ClusterStackKind)),
	))
	buildpackInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			c.Tracker.OnChanged,
			buildapi.SchemeGroupVersion.WithKind(buildapi.BuildpackKind)),
	))
	clusterBuildpackInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			c.Tracker.OnChanged,
			buildapi.SchemeGroupVersion.WithKind(buildapi.ClusterBuildpackKind)),
	))

	return impl, func() {
		impl.GlobalResync(builderInformer.Informer())
	}
}

type Reconciler struct {
	Client                 versioned.Interface
	BuilderLister          buildlisters.BuilderLister
	BuilderCreator         BuilderCreator
	KeychainFactory        registry.KeychainFactory
	Tracker                reconciler.Tracker
	ClusterStoreLister     buildlisters.ClusterStoreLister
	BuildpackLister        buildlisters.BuildpackLister
	ClusterBuildpackLister buildlisters.ClusterBuildpackLister
	ClusterStackLister     buildlisters.ClusterStackLister
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

func (c *Reconciler) reconcileBuilder(ctx context.Context, builder *buildapi.Builder) (buildapi.BuilderRecord, error) {
	c.Tracker.Track(reconciler.Key{
		NamespacedName: types.NamespacedName{
			Name:      builder.Spec.Stack.Name,
			Namespace: metav1.NamespaceAll,
		},
		GroupKind: schema.GroupKind{
			Group: "kpack.io",
			Kind:  buildapi.ClusterStackKind,
		},
	}, builder.NamespacedName())

	var (
		clusterStore *buildapi.ClusterStore
		err          error
	)
	if builder.Spec.Store.Name != "" {
		c.Tracker.Track(reconciler.Key{
			NamespacedName: types.NamespacedName{
				Name:      builder.Spec.Store.Name,
				Namespace: metav1.NamespaceAll,
			},
			GroupKind: schema.GroupKind{
				Group: "kpack.io",
				Kind:  buildapi.ClusterStoreKind,
			},
		}, builder.NamespacedName())

		clusterStore, err = c.ClusterStoreLister.Get(builder.Spec.Store.Name)
		if err != nil {
			return buildapi.BuilderRecord{}, err
		}
	}

	c.Tracker.TrackKind(schema.GroupKind{
		Group: "kpack.io",
		Kind:  buildapi.BuildpackKind,
	}, builder.NamespacedName())

	c.Tracker.TrackKind(schema.GroupKind{
		Group: "kpack.io",
		Kind:  buildapi.ClusterBuildpackKind,
	}, builder.NamespacedName())

	buildpacks, err := c.BuildpackLister.Buildpacks(builder.Namespace).List(labels.Everything())
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	clusterBuildpacks, err := c.ClusterBuildpackLister.List(labels.Everything())
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	clusterStack, err := c.ClusterStackLister.Get(builder.Spec.Stack.Name)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	if !clusterStack.Status.GetCondition(corev1alpha1.ConditionReady).IsTrue() {
		return buildapi.BuilderRecord{}, errors.Errorf("stack %s is not ready", clusterStack.Name)
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		ServiceAccount: builder.Spec.ServiceAccount(),
		Namespace:      builder.Namespace,
	})
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	fetcher := cnb.NewRemoteBuildpackFetcher(c.KeychainFactory, clusterStore, buildpacks, clusterBuildpacks)

	buildRecord, err := c.BuilderCreator.CreateBuilder(ctx, keychain, fetcher, clusterStack, builder.Spec.BuilderSpec)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	return buildRecord, nil
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *buildapi.Builder) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.BuilderLister.Builders(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().Builders(desired.Namespace).UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}
