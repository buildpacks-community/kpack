package clusterstore

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
	ReconcilerName = "ClusterStores"
	Kind           = "ClusterStore"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	Read(keychain authn.Keychain, storeImages []corev1alpha1.StoreImage) ([]corev1alpha1.StoreBuildpack, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	keychainFactory registry.KeychainFactory,
	clusterStoreInformer buildinformers.ClusterStoreInformer,
	storeReader StoreReader) *controller.Impl {
	c := &Reconciler{
		Client:             opt.Client,
		ClusterStoreLister: clusterStoreInformer.Lister(),
		StoreReader:        storeReader,
		KeychainFactory:    keychainFactory,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ClusterStoreCRName),
	)

	impl := controller.NewContext(ctx, c, controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger})
	clusterStoreInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client             versioned.Interface
	StoreReader        StoreReader
	ClusterStoreLister buildlisters.ClusterStoreLister
	KeychainFactory    registry.KeychainFactory
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, storeName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	clusterStore, err := c.ClusterStoreLister.Get(storeName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	clusterStore = clusterStore.DeepCopy()

	clusterStore, err = c.reconcileClusterStoreStatus(ctx, clusterStore)

	updateErr := c.updateClusterStoreStatus(ctx, clusterStore)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Reconciler) updateClusterStoreStatus(ctx context.Context, desired *buildapi.ClusterStore) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.ClusterStoreLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().ClusterStores().UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (c *Reconciler) reconcileClusterStoreStatus(ctx context.Context, clusterStore *buildapi.ClusterStore) (*buildapi.ClusterStore, error) {
	secretRef := registry.SecretRef{}

	if clusterStore.Spec.ServiceAccountRef != nil {
		secretRef = registry.SecretRef{
			ServiceAccount: clusterStore.Spec.ServiceAccountRef.Name,
			Namespace:      clusterStore.Spec.ServiceAccountRef.Namespace,
		}
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		clusterStore.Status = buildapi.ClusterStoreStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterStore.Generation, err),
		}
		return clusterStore, err
	}

	buildpacks, err := c.StoreReader.Read(keychain, clusterStore.Spec.Sources)
	if err != nil {
		clusterStore.Status = buildapi.ClusterStoreStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterStore.Generation, err),
		}
		return clusterStore, err
	}

	clusterStore.Status = buildapi.ClusterStoreStatus{
		Buildpacks: buildpacks,
		Status:     corev1alpha1.CreateStatusWithReadyCondition(clusterStore.Generation, nil),
	}
	return clusterStore, nil
}
