package clusterstore

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "ClusterStores"
	Kind           = "ClusterStore"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	Read(storeImages []buildapi.StoreImage) ([]buildapi.StoreBuildpack, error)
}

func NewController(opt reconciler.Options, clusterStoreInformer buildinformers.ClusterStoreInformer, storeReader StoreReader) *controller.Impl {
	c := &Reconciler{
		Client:             opt.Client,
		ClusterStoreLister: clusterStoreInformer.Lister(),
		StoreReader:        storeReader,
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	clusterStoreInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client             versioned.Interface
	StoreReader        StoreReader
	ClusterStoreLister buildlisters.ClusterStoreLister
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

	clusterStore, err = c.reconcileClusterStoreStatus(clusterStore)

	updateErr := c.updateClusterStoreStatus(ctx, clusterStore)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return controller.NewPermanentError(err)
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

func (c *Reconciler) reconcileClusterStoreStatus(clusterStore *buildapi.ClusterStore) (*buildapi.ClusterStore, error) {
	buildpacks, err := c.StoreReader.Read(clusterStore.Spec.Sources)
	if err != nil {
		clusterStore.Status = buildapi.ClusterStoreStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: clusterStore.Generation,
				Conditions: corev1alpha1.Conditions{
					{
						Type:               corev1alpha1.ConditionReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
						Message:            err.Error(),
					},
				},
			},
		}
		return clusterStore, err
	}

	clusterStore.Status = buildapi.ClusterStoreStatus{
		Buildpacks: buildpacks,
		Status: corev1alpha1.Status{
			ObservedGeneration: clusterStore.Generation,
			Conditions: corev1alpha1.Conditions{
				{
					LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
					Type:               corev1alpha1.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
	}
	return clusterStore, nil
}
