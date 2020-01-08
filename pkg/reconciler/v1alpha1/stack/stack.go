package stack

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"

	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1expInformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/experimental/v1alpha1"
	v1alpha1expListers "github.com/pivotal/kpack/pkg/client/listers/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	ReconcilerName = "Stacks"
	Kind           = "Stack"
)

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
}

func NewController(opt reconciler.Options, stackInformer v1alpha1expInformers.StackInformer, keychainFactory registry.KeychainFactory, fetcher ImageFetcher) *controller.Impl {
	c := &Reconciler{
		Client:          opt.Client,
		KeychainFactory: keychainFactory,
		ImageFetcher:    fetcher,
		StackLister:     stackInformer.Lister(),
	}
	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)
	stackInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))
	return impl
}

type Reconciler struct {
	Client          versioned.Interface
	KeychainFactory registry.KeychainFactory
	ImageFetcher    ImageFetcher
	StackLister     v1alpha1expListers.StackLister
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
	baseImageId, runImageId, err := c.getStackImages(stack.Spec)
	if err != nil {
		stack.Status = expv1alpha1.StackStatus{
			Status: duckv1alpha1.Status{
				ObservedGeneration: stack.Generation,
				Conditions: duckv1alpha1.Conditions{
					{
						Type:               duckv1alpha1.ConditionReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: apis.VolatileTime{Inner: metav1.Now()},
						Message:            err.Error(),
					},
				},
			},
		}
		return stack, err
	}

	stack.Status = expv1alpha1.StackStatus{
		BuildImage: expv1alpha1.StackStatusImage{
			LatestImage: baseImageId,
		},
		RunImage: expv1alpha1.StackStatusImage{
			LatestImage: runImageId,
		},
		Status: duckv1alpha1.Status{
			ObservedGeneration: stack.Generation,
			Conditions: duckv1alpha1.Conditions{
				{
					LastTransitionTime: apis.VolatileTime{Inner: metav1.Now()},
					Type:               duckv1alpha1.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
	}
	return stack, nil
}

func (c *Reconciler) getStackImages(stackSpec expv1alpha1.StackSpec) (string, string, error) {
	keychain, err := c.KeychainFactory.KeychainForSecretRef(registry.SecretRef{})
	if err != nil {
		return "", "", err
	}

	_, baseImageId, err := c.ImageFetcher.Fetch(keychain, stackSpec.BuildImage.Image)
	if err != nil {
		return "", "", err
	}

	_, runImageId, err := c.ImageFetcher.Fetch(keychain, stackSpec.RunImage.Image)
	if err != nil {
		return "", "", err
	}

	return baseImageId, runImageId, nil
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
