package clusterstack

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
	ReconcilerName = "Stacks"
	Kind           = "Stack"
)

//go:generate counterfeiter . ClusterStackReader
type ClusterStackReader interface {
	Read(keychain authn.Keychain, clusterStackSpec buildapi.ClusterStackSpec) (buildapi.ResolvedClusterStack, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	keychainFactory registry.KeychainFactory,
	clusterStackInformer buildinformers.ClusterStackInformer,
	clusterStackReader ClusterStackReader) *controller.Impl {
	c := &Reconciler{
		Client:             opt.Client,
		ClusterStackLister: clusterStackInformer.Lister(),
		ClusterStackReader: clusterStackReader,
		KeychainFactory:    keychainFactory,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ClusterStackCRName),
	)

	impl := controller.NewContext(ctx, c, controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger})
	clusterStackInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client             versioned.Interface
	ClusterStackLister buildlisters.ClusterStackLister
	ClusterStackReader ClusterStackReader
	KeychainFactory    registry.KeychainFactory
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, clusterStackName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	clusterStack, err := c.ClusterStackLister.Get(clusterStackName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	clusterStack = clusterStack.DeepCopy()

	clusterStack, err = c.reconcileClusterStackStatus(ctx, clusterStack)

	updateErr := c.updateClusterStackStatus(ctx, clusterStack)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Reconciler) reconcileClusterStackStatus(ctx context.Context, clusterStack *buildapi.ClusterStack) (*buildapi.ClusterStack, error) {
	secretRef := registry.SecretRef{}

	if clusterStack.Spec.ServiceAccountRef != nil {
		secretRef = registry.SecretRef{
			ServiceAccount: clusterStack.Spec.ServiceAccountRef.Name,
			Namespace:      clusterStack.Spec.ServiceAccountRef.Namespace,
		}
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		clusterStack.Status = buildapi.ClusterStackStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterStack.Generation, err),
		}
		return clusterStack, err
	}

	resolvedClusterStack, err := c.ClusterStackReader.Read(keychain, clusterStack.Spec)
	if err != nil {
		clusterStack.Status = buildapi.ClusterStackStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(clusterStack.Generation, err),
		}
		return clusterStack, err
	}

	clusterStack.Status = buildapi.ClusterStackStatus{
		Status:               corev1alpha1.CreateStatusWithReadyCondition(clusterStack.Generation, nil),
		ResolvedClusterStack: resolvedClusterStack,
	}
	return clusterStack, nil
}

func (c *Reconciler) updateClusterStackStatus(ctx context.Context, desired *buildapi.ClusterStack) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.ClusterStackLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().ClusterStacks().UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}
