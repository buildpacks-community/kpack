package stack

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
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
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	ReconcilerName = "Stacks"
	Kind           = "Stack"
	StackLabel     = "io.buildpacks.stack.id"
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
	buildImageId, runImageId, err := c.getStackImages(stack.Spec)
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
		BuildImage: expv1alpha1.StackStatusImage{
			LatestImage: buildImageId,
		},
		RunImage: expv1alpha1.StackStatusImage{
			LatestImage: runImageId,
		},
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
	}
	return stack, nil
}

func (c *Reconciler) getStackImages(stackSpec expv1alpha1.StackSpec) (string, string, error) {
	keychain, err := c.KeychainFactory.KeychainForSecretRef(registry.SecretRef{})
	if err != nil {
		return "", "", err
	}

	buildImage, buildImageId, err := c.ImageFetcher.Fetch(keychain, stackSpec.BuildImage.Image)
	if err != nil {
		return "", "", err
	}

	runImage, runImageId, err := c.ImageFetcher.Fetch(keychain, stackSpec.RunImage.Image)
	if err != nil {
		return "", "", err
	}

	err = validateStackId(stackSpec.Id, buildImage, runImage)
	if err != nil {
		return "", "", err
	}

	return buildImageId, runImageId, nil
}

func validateStackId(stackId string, buildImage ggcrv1.Image, runImage ggcrv1.Image) error {
	buildStack, err := imagehelpers.GetStringLabel(buildImage, StackLabel)
	if err != nil {
		return errors.Errorf("invalid build image provided for stack: %s", err.Error())
	}

	runStack, err := imagehelpers.GetStringLabel(runImage, StackLabel)
	if err != nil {
		return errors.Errorf("invalid run image provided for stack: %s", err.Error())
	}

	if (buildStack == stackId) && (runStack == stackId) {
		return nil
	} else {
		return errors.Errorf("invalid stack images. expected stack: %s, build image stack: %s, run image stack: %s", stackId, buildStack, runStack)
	}
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
