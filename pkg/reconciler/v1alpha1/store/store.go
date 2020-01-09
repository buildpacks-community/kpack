package store

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	kpackcore "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1expInformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/experimental/v1alpha1"
	v1alpha1expListers "github.com/pivotal/kpack/pkg/client/listers/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	ReconcilerName = "Stores"
	Kind           = "Store"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	Read(keychain authn.Keychain, storeImages []expv1alpha1.StoreImage) ([]expv1alpha1.StoreBuildpack, error)
}

func NewController(opt reconciler.Options, storeInformer v1alpha1expInformers.StoreInformer, storeReader StoreReader, keychainFactory registry.KeychainFactory) *controller.Impl {
	c := &Reconciler{
		Client:          opt.Client,
		StoreLister:     storeInformer.Lister(),
		StoreReader:     storeReader,
		KeychainFactory: keychainFactory,
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	storeInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client          versioned.Interface
	StoreReader     StoreReader
	KeychainFactory registry.KeychainFactory
	StoreLister     v1alpha1expListers.StoreLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, storeName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	store, err := c.StoreLister.Get(storeName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	store = store.DeepCopy()

	store, err = c.reconcileStoreStatus(store)

	updateErr := c.updateStoreStatus(store)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return controller.NewPermanentError(err)
	}
	return nil
}

func (c *Reconciler) updateStoreStatus(desired *expv1alpha1.Store) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.StoreLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.ExperimentalV1alpha1().Stores().UpdateStatus(desired)
	return err
}

func (c *Reconciler) reconcileStoreStatus(store *expv1alpha1.Store) (*expv1alpha1.Store, error) {
	buildpacks, err := c.getBuildpacks(store.Spec.Sources)
	if err != nil {
		store.Status = expv1alpha1.StoreStatus{
			Status: kpackcore.Status{
				ObservedGeneration: store.Generation,
				Conditions: kpackcore.Conditions{
					{
						Type:               kpackcore.ConditionReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: kpackcore.VolatileTime{Inner: v1.Now()},
						Message:            err.Error(),
					},
				},
			},
		}
		return store, err
	}

	store.Status = expv1alpha1.StoreStatus{
		Buildpacks: buildpacks,
		Status: kpackcore.Status{
			ObservedGeneration: store.Generation,
			Conditions: kpackcore.Conditions{
				{
					LastTransitionTime: kpackcore.VolatileTime{Inner: v1.Now()},
					Type:               kpackcore.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
	}
	return store, nil
}

func (c *Reconciler) getBuildpacks(sources []expv1alpha1.StoreImage) ([]expv1alpha1.StoreBuildpack, error) {
	keychain, err := c.KeychainFactory.KeychainForSecretRef(registry.SecretRef{})
	if err != nil {
		return nil, err
	}

	return c.StoreReader.Read(keychain, sources)
}
