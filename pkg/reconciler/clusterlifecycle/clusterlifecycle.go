package clusterlifecycle

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
	ReconcilerName = "Lifecycles"
	Kind           = "Lifecycle"
)

//go:generate counterfeiter . ClusterLifecycleReader
type ClusterLifecycleReader interface {
	// TODO: add implementation
	Read(keychain authn.Keychain, clusterLifecycleSpec buildapi.ClusterLifecycleSpec) (buildapi.ResolvedClusterLifecycle, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	keychainFactory registry.KeychainFactory,
	clusterLifecycleInformer buildinformers.ClusterLifecycleInformer,
	clusterLifecycleReader ClusterLifecycleReader,
) *controller.Impl {
	c := &Reconciler{
		Client:                 opt.Client,
		ClusterLifecycleLister: clusterLifecycleInformer.Lister(),
		ClusterLifecycleReader: clusterLifecycleReader,
		KeychainFactory:        keychainFactory,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ClusterLifecycleCRName),
	)

	impl := controller.NewContext(
		ctx,
		&reconciler.NetworkErrorReconciler{
			Reconciler: c,
		},
		controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger},
	)
	clusterLifecycleInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client                 versioned.Interface
	ClusterLifecycleLister buildlisters.ClusterLifecycleLister
	ClusterLifecycleReader ClusterLifecycleReader
	KeychainFactory        registry.KeychainFactory
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, clusterLifecycleName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	clusterLifecycle, err := c.ClusterLifecycleLister.Get(clusterLifecycleName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	clusterLifecycle = clusterLifecycle.DeepCopy()

	clusterLifecycle, err = c.reconcileClusterLifecycleStatus(ctx, clusterLifecycle)

	updateErr := c.updateClusterLifecycleStatus(ctx, clusterLifecycle)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Reconciler) reconcileClusterLifecycleStatus(ctx context.Context, clusterLifecycle *buildapi.ClusterLifecycle) (*buildapi.ClusterLifecycle, error) {
	secretRef := registry.SecretRef{}

	if clusterLifecycle.Spec.ServiceAccountRef != nil {
		secretRef = registry.SecretRef{
			ServiceAccount: clusterLifecycle.Spec.ServiceAccountRef.Name,
			Namespace:      clusterLifecycle.Spec.ServiceAccountRef.Namespace,
		}
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		clusterLifecycle.Status = buildapi.ClusterLifecycleStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterLifecycle.Generation, err),
		}
		return clusterLifecycle, err
	}

	resolvedClusterLifecycle, err := c.ClusterLifecycleReader.Read(keychain, clusterLifecycle.Spec)
	if err != nil {
		clusterLifecycle.Status = buildapi.ClusterLifecycleStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterLifecycle.Generation, err),
		}
		return clusterLifecycle, err
	}

	clusterLifecycle.Status = buildapi.ClusterLifecycleStatus{
		Status:                   corev1alpha1.CreateStatusWithReadyCondition(clusterLifecycle.Generation, nil),
		ResolvedClusterLifecycle: resolvedClusterLifecycle,
	}
	return clusterLifecycle, nil
}

func (c *Reconciler) updateClusterLifecycleStatus(ctx context.Context, desired *buildapi.ClusterLifecycle) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.ClusterLifecycleLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().ClusterLifecycles().UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}
