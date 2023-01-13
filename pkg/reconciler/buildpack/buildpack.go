package buildpack

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
	ReconcilerName = "Buildpacks"
	Kind           = "Buildpack"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	Read(keychain authn.Keychain, storeImages []corev1alpha1.StoreImage) ([]corev1alpha1.StoreBuildpack, error)
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	keychainFactory registry.KeychainFactory,
	buildpackInformer buildinformers.BuildpackInformer,
	storeReader StoreReader) *controller.Impl {
	c := &Reconciler{
		Client:          opt.Client,
		BuildpackLister: buildpackInformer.Lister(),
		StoreReader:     storeReader,
		KeychainFactory: keychainFactory,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.BuildpackCRName),
	)

	impl := controller.NewContext(
		ctx,
		&reconciler.NetworkErrorReconciler{
			Reconciler: c,
		},
		controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger},
	)
	buildpackInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client          versioned.Interface
	StoreReader     StoreReader
	BuildpackLister buildlisters.BuildpackLister
	KeychainFactory registry.KeychainFactory
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, buildpackName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	buildpack, err := c.BuildpackLister.Buildpacks(namespace).Get(buildpackName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	buildpack = buildpack.DeepCopy()

	buildpack, err = c.reconcileBuildpackStatus(ctx, buildpack)

	updateErr := c.updateBuildpackStatus(ctx, buildpack)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return err
	}
	return nil
}

func (c *Reconciler) updateBuildpackStatus(ctx context.Context, desired *buildapi.Buildpack) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.BuildpackLister.Buildpacks(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().Buildpacks(desired.Namespace).UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (c *Reconciler) reconcileBuildpackStatus(ctx context.Context, buildpack *buildapi.Buildpack) (*buildapi.Buildpack, error) {
	secretRef := registry.SecretRef{}

	if buildpack.Spec.ServiceAccountName != "" {
		secretRef = registry.SecretRef{
			ServiceAccount: buildpack.Spec.ServiceAccountName,
			Namespace:      buildpack.Namespace,
		}
	}

	keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, secretRef)
	if err != nil {
		buildpack.Status = buildapi.BuildpackStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(buildpack.Generation, err),
		}
		return buildpack, err
	}

	buildpacks, err := c.StoreReader.Read(keychain, []corev1alpha1.StoreImage{buildpack.Spec.Source})
	if err != nil {
		buildpack.Status = buildapi.BuildpackStatus{
			Status: corev1alpha1.CreateStatusWithReadyCondition(buildpack.Generation, err),
		}
		return buildpack, err
	}

	buildpack.Status = buildapi.BuildpackStatus{
		Buildpacks: buildpacks,
		Status:     corev1alpha1.CreateStatusWithReadyCondition(buildpack.Generation, nil),
	}
	return buildpack, nil
}
