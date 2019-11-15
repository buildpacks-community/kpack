package clusterbuilder

import (
	"context"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	experimentalV1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
	"k8s.io/apimachinery/pkg/api/equality"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/kpack/pkg/client/informers/externalversions/experimental/v1alpha1"
	v1alpha1Listers "github.com/pivotal/kpack/pkg/client/listers/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "CustomBuilders"
	Kind           = "CustomBuilder"
)

type BuilderCreator interface {
	CreateBuilder(keychain authn.Keychain, customBuilder *experimentalV1alpha1.CustomBuilder) (v1alpha1.BuilderRecord, error)
}

func NewController(opt reconciler.Options, customBuilderInformer v1alpha1informers.CustomBuilderInformer) *controller.Impl {
	c := &Reconciler{
		Client:              opt.Client,
		CustomBuilderLister: customBuilderInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	customBuilderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

type Reconciler struct {
	Client              versioned.Interface
	CustomBuilderLister v1alpha1Listers.CustomBuilderLister

	BuilderCreator    BuilderCreator
	KeychainFactory   registry.KeychainFactory
	RemoteImageClient registry.Client
}

//todo this is not tested.
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

	builder, creationError := c.reconcileCustomBuilder(customBuilder)
	if creationError != nil {
		customBuilder.ErrorCreate(err)

		err := c.updateStatus(customBuilder)
		if err != nil {
			return nil
		}

		return controller.NewPermanentError(creationError)
	} else {
		customBuilder.Status.BuilderStatus(builder)
	}

	return c.updateStatus(customBuilder)
}

func (c *Reconciler) updateStatus(desired *experimentalV1alpha1.CustomBuilder) error {
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

func (c *Reconciler) reconcileCustomBuilder(customBuilder *experimentalV1alpha1.CustomBuilder) (v1alpha1.BuilderRecord, error) {
	keychain, err := c.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		ServiceAccount: customBuilder.Spec.ServiceAccount,
		Namespace:      customBuilder.Namespace,
	})
	if err != nil {
		return v1alpha1.BuilderRecord{}, err
	}

	return c.BuilderCreator.CreateBuilder(keychain, customBuilder)
}
