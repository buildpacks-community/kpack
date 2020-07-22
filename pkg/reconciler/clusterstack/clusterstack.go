package clusterstack

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ReconcilerName = "Stacks"
	Kind           = "Stack"
)

//go:generate counterfeiter . ClusterStackReader
type ClusterStackReader interface {
	Read(clusterStackSpec expv1alpha1.ClusterStackSpec) (expv1alpha1.ResolvedClusterStack, error)
}

func NewController(opt reconciler.Options, clusterStackInformer v1alpha1expInformers.ClusterStackInformer, clusterStackReader ClusterStackReader) *controller.Impl {
	c := &Reconciler{
		Client:             opt.Client,
		ClusterStackLister: clusterStackInformer.Lister(),
		ClusterStackReader: clusterStackReader,
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	clusterStackInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client             versioned.Interface
	ClusterStackLister v1alpha1expListers.ClusterStackLister
	ClusterStackReader ClusterStackReader
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

	clusterStack, err = c.reconcileClusterStackStatus(clusterStack)

	updateErr := c.updateClusterStackStatus(clusterStack)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return controller.NewPermanentError(err)
	}
	return nil
}

func (c *Reconciler) reconcileClusterStackStatus(clusterStack *expv1alpha1.ClusterStack) (*expv1alpha1.ClusterStack, error) {
	resolvedClusterStack, err := c.ClusterStackReader.Read(clusterStack.Spec)
	if err != nil {
		clusterStack.Status = expv1alpha1.ClusterStackStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: clusterStack.Generation,
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
		return clusterStack, err
	}

	clusterStack.Status = expv1alpha1.ClusterStackStatus{
		Status: corev1alpha1.Status{
			ObservedGeneration: clusterStack.Generation,
			Conditions: corev1alpha1.Conditions{
				{
					LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
					Type:               corev1alpha1.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
		ResolvedClusterStack: resolvedClusterStack,
	}
	return clusterStack, nil
}

func (c *Reconciler) updateClusterStackStatus(desired *expv1alpha1.ClusterStack) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.ClusterStackLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.ExperimentalV1alpha1().ClusterStacks().UpdateStatus(desired)
	return err
}
