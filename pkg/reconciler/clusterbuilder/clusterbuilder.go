package clusterbuilder

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
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
	ReconcilerName = "ClusterBuilders"
)

type BuilderCreator interface {
	CreateBuilder(
		ctx context.Context,
		builderKeychain authn.Keychain,
		stackKeychain authn.Keychain,
		lifecycleKeychain authn.Keychain,
		fetcher cnb.RemoteBuildpackFetcher,
		clusterStack *buildapi.ClusterStack,
		clusterLifecycle *buildapi.ClusterLifecycle,
		spec buildapi.BuilderSpec,
		serviceAccountSecrets []*corev1.Secret,
		resolvedBuilderRef string,
	) (buildapi.BuilderRecord, error)
}

type Fetcher interface {
	SecretsForServiceAccount(context.Context, string, string) ([]*corev1.Secret, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	clusterBuilderInformer buildinformers.ClusterBuilderInformer,
	builderCreator BuilderCreator,
	keychainFactory registry.KeychainFactory,
	clusterStoreInformer buildinformers.ClusterStoreInformer,
	clusterBuildpackInformer buildinformers.ClusterBuildpackInformer,
	clusterStackInformer buildinformers.ClusterStackInformer,
	clusterLifecycleInformer buildinformers.ClusterLifecycleInformer,
	secretFetcher Fetcher,
) *controller.Impl {
	c := &Reconciler{
		Client:                 opt.Client,
		ClusterBuilderLister:   clusterBuilderInformer.Lister(),
		BuilderCreator:         builderCreator,
		KeychainFactory:        keychainFactory,
		ClusterStoreLister:     clusterStoreInformer.Lister(),
		ClusterBuildpackLister: clusterBuildpackInformer.Lister(),
		ClusterStackLister:     clusterStackInformer.Lister(),
		ClusterLifecycleLister: clusterLifecycleInformer.Lister(),
		SecretFetcher:          secretFetcher,
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
	clusterBuilderInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())
	clusterBuildpackInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			c.Tracker.OnChanged,
			buildapi.SchemeGroupVersion.WithKind(buildapi.ClusterBuildpackKind)),
	))

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
	clusterLifecycleInformer.Informer().AddEventHandler(controller.HandleAll(
		controller.EnsureTypeMeta(
			c.Tracker.OnChanged,
			buildapi.SchemeGroupVersion.WithKind(buildapi.ClusterLifecycleKind)),
	))

	return impl
}

type Reconciler struct {
	Client                 versioned.Interface
	ClusterBuilderLister   buildlisters.ClusterBuilderLister
	BuilderCreator         BuilderCreator
	KeychainFactory        registry.KeychainFactory
	Tracker                reconciler.Tracker
	ClusterStoreLister     buildlisters.ClusterStoreLister
	ClusterBuildpackLister buildlisters.ClusterBuildpackLister
	ClusterStackLister     buildlisters.ClusterStackLister
	ClusterLifecycleLister buildlisters.ClusterLifecycleLister
	SecretFetcher          Fetcher
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
	c.Tracker.Track(reconciler.Key{
		NamespacedName: types.NamespacedName{
			Name:      builder.Spec.Stack.Name,
			Namespace: corev1.NamespaceAll,
		},
		GroupKind: schema.GroupKind{
			Group: "kpack.io",
			Kind:  buildapi.ClusterStackKind,
		},
	}, builder.NamespacedName())

	c.Tracker.Track(reconciler.Key{
		NamespacedName: types.NamespacedName{
			Name:      builder.Spec.Lifecycle.Name,
			Namespace: corev1.NamespaceAll,
		},
		GroupKind: schema.GroupKind{
			Group: "kpack.io",
			Kind:  buildapi.ClusterLifecycleKind,
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
		Kind:  buildapi.ClusterBuildpackKind,
	}, builder.NamespacedName())

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

	clusterLifecycle, err := c.ClusterLifecycleLister.Get(builder.Spec.Lifecycle.Name)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	if !clusterLifecycle.Status.GetCondition(corev1alpha1.ConditionReady).IsTrue() {
		return buildapi.BuilderRecord{}, errors.Errorf("Error: clusterlifecycle '%s' is not ready", clusterLifecycle.Name)
	}

	builderKeychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		ServiceAccount: builder.Spec.ServiceAccountRef.Name,
		Namespace:      builder.Spec.ServiceAccountRef.Namespace,
	})
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	stackKeychain := builderKeychain
	if clusterStack.Spec.ServiceAccountRef != nil {
		stackKeychain, err = c.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
			ServiceAccount: clusterStack.Spec.ServiceAccountRef.Name,
			Namespace:      clusterStack.Spec.ServiceAccountRef.Namespace,
		})
		if err != nil {
			return buildapi.BuilderRecord{}, err
		}
	}

	lifecycleKeychain := builderKeychain
	if clusterLifecycle.Spec.ServiceAccountRef != nil {
		lifecycleKeychain, err = c.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
			ServiceAccount: clusterLifecycle.Spec.ServiceAccountRef.Name,
			Namespace:      clusterLifecycle.Spec.ServiceAccountRef.Namespace,
		})
		if err != nil {
			return buildapi.BuilderRecord{}, err
		}
	}

	fetcher := cnb.NewRemoteBuildpackFetcher(c.KeychainFactory, clusterStore, nil, clusterBuildpacks)

	serviceAccountSecrets, err := c.SecretFetcher.SecretsForServiceAccount(ctx, builder.Spec.ServiceAccountRef.Name, builder.Spec.ServiceAccountRef.Namespace)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	resolvedBuilderRef, err := resolveBuilderRef(builder)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	buildRecord, err := c.BuilderCreator.CreateBuilder(
		ctx,
		builderKeychain,
		stackKeychain,
		lifecycleKeychain,
		fetcher,
		clusterStack,
		clusterLifecycle,
		builder.Spec.BuilderSpec,
		serviceAccountSecrets,
		resolvedBuilderRef,
	)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	return buildRecord, nil
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

func resolveBuilderRef(builder *buildapi.ClusterBuilder) (string, error) {
	parsedRef, err := name.ParseReference(builder.Spec.Tag)
	if err != nil {
		return "", err
	}

	// this happens if there is no tag
	if parsedRef.Identifier() == "latest" {
		return parsedRef.
			Context().
			Tag(fmt.Sprintf("%s-%s", strings.ToLower(buildapi.ClusterBuilderKind), builder.Name)).Name(), nil
	}

	return parsedRef.Name(), nil
}
