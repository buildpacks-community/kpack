package stack

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

//go:generate counterfeiter . StackReader
type StackReader interface {
	Read(stackSpec expv1alpha1.StackSpec) (expv1alpha1.ResolvedStack, error)
}

func NewController(opt reconciler.Options, stackInformer v1alpha1expInformers.StackInformer, stackReader StackReader) *controller.Impl {
	c := &Reconciler{
		Client:      opt.Client,
		StackLister: stackInformer.Lister(),
		StackReader: stackReader,
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	stackInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client      versioned.Interface
	StackLister v1alpha1expListers.StackLister
	StackReader StackReader
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, stackName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	stack, err := c.StackLister.Get(stackName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	stack = stack.DeepCopy()

	stack, err = c.reconcileStackStatus(stack)

	updateErr := c.updateStackStatus(stack)
	if updateErr != nil {
		return updateErr
	}

	if err != nil {
		return controller.NewPermanentError(err)
	}
	return nil
}

func (c *Reconciler) reconcileStackStatus(stack *expv1alpha1.Stack) (*expv1alpha1.Stack, error) {
	resolvedStack, err := c.StackReader.Read(stack.Spec)
	if err != nil {
		stack.Status = expv1alpha1.StackStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: stack.Generation,
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
		return stack, err
	}

	stack.Status = expv1alpha1.StackStatus{
		Status: corev1alpha1.Status{
			ObservedGeneration: stack.Generation,
			Conditions: corev1alpha1.Conditions{
				{
					LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
					Type:               corev1alpha1.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
		ResolvedStack: resolvedStack,
	}
	return stack, nil
}

func (c *Reconciler) updateStackStatus(desired *expv1alpha1.Stack) error {
	desired.Status.ObservedGeneration = desired.Generation

	original, err := c.StackLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) {
		return nil
	}

	_, err = c.Client.ExperimentalV1alpha1().Stacks().UpdateStatus(desired)
	return err
}
