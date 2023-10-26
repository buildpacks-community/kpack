package extension

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging/logkey"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	ReconcilerName = "Extensions"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	ReadExtension(keychain authn.Keychain, storeImages []corev1alpha1.ImageSource) ([]corev1alpha1.BuildpackStatus, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	keychainFactory registry.KeychainFactory,
	informer buildinformers.ExtensionInformer,
	storeReader StoreReader,
) *controller.Impl {
	c := &Reconciler{
		Client:          opt.Client,
		Lister:          informer.Lister(),
		StoreReader:     storeReader,
		KeychainFactory: keychainFactory,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ExtensionCRName),
	)

	impl := controller.NewContext(
		ctx,
		&reconciler.NetworkErrorReconciler{
			Reconciler: c,
		},
		controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger},
	)
	informer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client          versioned.Interface
	StoreReader     StoreReader
	Lister          buildlisters.ExtensionLister
	KeychainFactory registry.KeychainFactory
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, moduleName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	module, err := c.Lister.Extensions(namespace).Get(moduleName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	module = module.DeepCopy()

	module, err = c.reconcileExtensionStatus(ctx, module)

	updateErr := c.updateModuleStatus(ctx, module)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Reconciler) updateModuleStatus(ctx context.Context, desired *buildapi.Extension) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.Lister.Extensions(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().Extensions(desired.Namespace).UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (c *Reconciler) reconcileExtensionStatus(ctx context.Context, module *buildapi.Extension) (*buildapi.Extension, error) {
	secretRef := registry.SecretRef{}

	if module.Spec.ServiceAccountName != "" {
		secretRef = registry.SecretRef{
			ServiceAccount: module.Spec.ServiceAccountName,
			Namespace:      module.Namespace,
		}
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		module.Status = buildapi.ExtensionStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(module.Generation, err),
		}
		return module, err
	}

	modules, err := c.StoreReader.ReadExtension(keychain, []corev1alpha1.ImageSource{module.Spec.ImageSource})
	if err != nil {
		module.Status = buildapi.ExtensionStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(module.Generation, err),
		}
		return module, err
	}

	module.Status = buildapi.ExtensionStatus{
		Extensions: modules,
		Status:     corev1alpha1.CreateStatusWithReadyCondition(module.Generation, nil),
	}
	return module, nil
}
