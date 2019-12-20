package store

import (
	"context"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"

	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"

	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1expInformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/experimental/v1alpha1"
	v1alpha1expListers "github.com/pivotal/kpack/pkg/client/listers/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "Stores"
	Kind           = "Store"
)

type BuildPackageClient interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
}

func NewController(opt reconciler.Options, storeInformer v1alpha1expInformers.StoreInformer, client BuildPackageClient) *controller.Impl {
	c := &Reconciler{
		Client:             opt.Client,
		StoreLister:        storeInformer.Lister(),
		BuildPackageClient: client,
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	storeInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client             versioned.Interface
	BuildPackageClient BuildPackageClient
	StoreLister        v1alpha1expListers.StoreLister
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
			Status: duckv1alpha1.Status{
				ObservedGeneration: store.Generation,
				Conditions: duckv1alpha1.Conditions{
					{
						Type:               duckv1alpha1.ConditionReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: apis.VolatileTime{Inner: v1.Now()},
						Message:            err.Error(),
					},
				},
			},
		}
		return store, err
	}

	store.Status = expv1alpha1.StoreStatus{
		Buildpacks: buildpacks,
		Status: duckv1alpha1.Status{
			ObservedGeneration: store.Generation,
			Conditions: duckv1alpha1.Conditions{
				{
					LastTransitionTime: apis.VolatileTime{Inner: v1.Now()},
					Type:               duckv1alpha1.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
	}
	return store, nil
}

func (c *Reconciler) getBuildpacks(sources []expv1alpha1.BuildPackage) ([]expv1alpha1.StoreBuildpack, error) {
	var buildpacks []expv1alpha1.StoreBuildpack
	for _, buildPackage := range sources {
		image, _, err := c.BuildPackageClient.Fetch(authn.DefaultKeychain, buildPackage.Image)
		if err != nil {
			return nil, err
		}

		var packageMetadata cnb.BuildpackLayerMetadata
		err = imagehelpers.GetLabel(image, "io.buildpacks.buildpack.layers", &packageMetadata)
		if err != nil {
			return nil, err
		}

		for id := range packageMetadata {
			for version := range packageMetadata[id] {
				order, err := toStoreOrder(packageMetadata[id][version].Order)
				if err != nil {
					return nil, err
				}
				storeBP := expv1alpha1.StoreBuildpack{
					ID:           id,
					Version:      version,
					LayerDiffID:  packageMetadata[id][version].LayerDiffID,
					BuildPackage: buildPackage,
					Order:        order,
				}
				buildpacks = append(buildpacks, storeBP)
			}
		}
	}
	return buildpacks, nil
}

func toStoreOrder(order cnb.Order) (expv1alpha1.Order, error) {
	var storeOrder expv1alpha1.Order
	for _, entry := range order {
		var storeEntry expv1alpha1.OrderEntry
		for _, buildpack := range entry.Group {
			storeEntry.Group = append(storeEntry.Group, expv1alpha1.BuildpackRef{
				BuildpackInfo: expv1alpha1.BuildpackInfo{
					ID:      buildpack.ID,
					Version: buildpack.Version,
				},
				Optional: buildpack.Optional,
			})
		}
		storeOrder = append(storeOrder, storeEntry)
	}
	return storeOrder, nil
}
