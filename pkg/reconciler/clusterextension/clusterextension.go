package clusterextension

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
	ReconcilerName = "ClusterExtensions"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	ReadExtension(keychain authn.Keychain, storeImages []corev1alpha1.ImageSource) ([]corev1alpha1.BuildpackStatus, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	keychainFactory registry.KeychainFactory,
	informer buildinformers.ClusterExtensionInformer,
	storeReader StoreReader) *controller.Impl {
	c := &Reconciler{
		Client:          opt.Client,
		Lister:          informer.Lister(),
		StoreReader:     storeReader,
		KeychainFactory: keychainFactory,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ClusterExtensionCRName),
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
	Lister          buildlisters.ClusterExtensionLister
	KeychainFactory registry.KeychainFactory
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, moduleName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	module, err := c.Lister.Get(moduleName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	module = module.DeepCopy()

	module, err = c.reconcileStatus(ctx, module)

	updateErr := c.updateStatus(ctx, module)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *buildapi.ClusterExtension) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.Lister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().ClusterExtensions().UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (c *Reconciler) reconcileStatus(ctx context.Context, module *buildapi.ClusterExtension) (*buildapi.ClusterExtension, error) {
	secretRef := registry.SecretRef{}

	if module.Spec.ServiceAccountRef != nil {
		secretRef = registry.SecretRef{
			ServiceAccount: module.Spec.ServiceAccountRef.Name,
			Namespace:      module.Spec.ServiceAccountRef.Namespace,
		}
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		module.Status = buildapi.ClusterExtensionStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(module.Generation, err),
		}
		return module, err
	}

	modules, err := c.StoreReader.ReadExtension(keychain, []corev1alpha1.ImageSource{module.Spec.ImageSource})
	if err != nil {
		module.Status = buildapi.ClusterExtensionStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(module.Generation, err),
		}
		return module, err
	}

	module.Status = buildapi.ClusterExtensionStatus{
		Extensions: modules,
		Status:     corev1alpha1.CreateStatusWithReadyCondition(module.Generation, nil),
	}
	return module, nil
}
