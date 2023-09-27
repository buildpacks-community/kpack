package clusterbuildpack

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

// TODO: add for extensions
const (
	ReconcilerName = "ClusterBuildpacks"
	Kind           = "ClusterBuildpack"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	Read(keychain authn.Keychain, storeImages []corev1alpha1.ImageSource) ([]corev1alpha1.BuildpackStatus, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	keychainFactory registry.KeychainFactory,
	clusterBuildpackInformer buildinformers.ClusterBuildpackInformer,
	storeReader StoreReader) *controller.Impl {
	c := &Reconciler{
		Client:                 opt.Client,
		ClusterBuildpackLister: clusterBuildpackInformer.Lister(),
		StoreReader:            storeReader,
		KeychainFactory:        keychainFactory,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ClusterBuildpackCRName),
	)

	impl := controller.NewContext(
		ctx,
		&reconciler.NetworkErrorReconciler{
			Reconciler: c,
		},
		controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger},
	)
	clusterBuildpackInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client                 versioned.Interface
	StoreReader            StoreReader
	ClusterBuildpackLister buildlisters.ClusterBuildpackLister
	KeychainFactory        registry.KeychainFactory
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, clusterBuildpackName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	clusterBuildpack, err := c.ClusterBuildpackLister.Get(clusterBuildpackName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	clusterBuildpack = clusterBuildpack.DeepCopy()

	clusterBuildpack, err = c.reoncileClusterBuildpackStatus(ctx, clusterBuildpack)

	updateErr := c.updateClusterBuildpackStatus(ctx, clusterBuildpack)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Reconciler) updateClusterBuildpackStatus(ctx context.Context, desired *buildapi.ClusterBuildpack) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.ClusterBuildpackLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().ClusterBuildpacks().UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (c *Reconciler) reoncileClusterBuildpackStatus(ctx context.Context, clusterBuildpack *buildapi.ClusterBuildpack) (*buildapi.ClusterBuildpack, error) {
	secretRef := registry.SecretRef{}

	if clusterBuildpack.Spec.ServiceAccountRef != nil {
		secretRef = registry.SecretRef{
			ServiceAccount: clusterBuildpack.Spec.ServiceAccountRef.Name,
			Namespace:      clusterBuildpack.Spec.ServiceAccountRef.Namespace,
		}
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		clusterBuildpack.Status = buildapi.ClusterBuildpackStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterBuildpack.Generation, err),
		}
		return clusterBuildpack, err
	}

	buildpacks, err := c.StoreReader.Read(keychain, []corev1alpha1.ImageSource{clusterBuildpack.Spec.ImageSource})
	if err != nil {
		clusterBuildpack.Status = buildapi.ClusterBuildpackStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterBuildpack.Generation, err),
		}
		return clusterBuildpack, err
	}

	clusterBuildpack.Status = buildapi.ClusterBuildpackStatus{
		Buildpacks: buildpacks,
		Status:     corev1alpha1.CreateStatusWithReadyCondition(clusterBuildpack.Generation, nil),
	}
	return clusterBuildpack, nil
}
