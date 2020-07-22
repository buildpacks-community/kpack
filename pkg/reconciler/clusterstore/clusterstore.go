package clusterstore

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1expInformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/experimental/v1alpha1"
	v1alpha1expListers "github.com/pivotal/kpack/pkg/client/listers/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "ClusterStores"
	Kind           = "ClusterStore"
)

//go:generate counterfeiter . StoreReader
type StoreReader interface {
	Read(storeImages []expv1alpha1.StoreImage) ([]expv1alpha1.StoreBuildpack, error)
}

func NewController(opt reconciler.Options, clusterStoreInformer v1alpha1expInformers.ClusterStoreInformer, storeReader StoreReader) *controller.Impl {
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
	ClusterStoreLister v1alpha1expListers.ClusterStoreLister
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

	updateErr := c.updateClusterStoreStatus(clusterStore)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return controller.NewPermanentError(err)
	}
	return nil
}

func (c *Reconciler) updateClusterStoreStatus(desired *expv1alpha1.ClusterStore) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.ClusterStoreLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.ExperimentalV1alpha1().ClusterStores().UpdateStatus(desired)
	return err
}

func (c *Reconciler) reconcileClusterStoreStatus(clusterStore *expv1alpha1.ClusterStore) (*expv1alpha1.ClusterStore, error) {
	buildpacks, err := c.StoreReader.Read(clusterStore.Spec.Sources)
	if err != nil {
		clusterStore.Status = expv1alpha1.ClusterStoreStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: clusterStore.Generation,
				Conditions: corev1alpha1.Conditions{
					{
						Type:               corev1alpha1.ConditionReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
						Message:            err.Error(),
					},
				},
			},
		}
		return clusterStore, err
	}

	clusterStore.Status = expv1alpha1.ClusterStoreStatus{
		Buildpacks: buildpacks,
		Status: corev1alpha1.Status{
			ObservedGeneration: clusterStore.Generation,
			Conditions: corev1alpha1.Conditions{
				{
					LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
					Type:               corev1alpha1.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
	}
	return clusterStore, nil
}
