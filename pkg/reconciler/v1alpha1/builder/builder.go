package builder

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "Builders"
	Kind           = "Builder"
)

//go:generate counterfeiter . MetadataRetriever
type MetadataRetriever interface {
	GetBuilderImage(builder v1alpha1.BuilderResource) (cnb.BuilderImage, error)
}

func NewController(opt reconciler.Options, builderInformer v1alpha1informers.BuilderInformer, metadataRetriever MetadataRetriever) *controller.Impl {
	c := &Reconciler{
		Client:            opt.Client,
		MetadataRetriever: metadataRetriever,
		BuilderLister:     builderInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	c.Enqueuer = &workQueueEnqueuer{
		enqueueAfter: impl.EnqueueAfter,
		delay:        opt.BuilderPollingFrequency,
	}

	builderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

//go:generate counterfeiter . Enqueuer
type Enqueuer interface {
	Enqueue(*v1alpha1.Builder) error
}

type Reconciler struct {
	Client            versioned.Interface
	MetadataRetriever MetadataRetriever
	BuilderLister     v1alpha1Listers.BuilderLister
	Enqueuer          Enqueuer
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	builder, err := c.BuilderLister.Builders(namespace).Get(builderName)
	if k8s_errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	builder = builder.DeepCopy()

	builder, err = c.reconcileBuilderStatus(builder)

	updateErr := c.updateStatus(builder)
	if updateErr != nil {
		return updateErr
	}

	if builder.Spec.UpdatePolicy != v1alpha1.External {
		err := c.Enqueuer.Enqueue(builder)
		if err != nil {
			return err
		}
	}

	if err != nil {
		return controller.NewPermanentError(err)
	}
	return nil
}

func (c *Reconciler) updateStatus(desired *v1alpha1.Builder) error {
	original, err := c.BuilderLister.Builders(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) { //this is a bug :(
		return nil
	}

	_, err = c.Client.BuildV1alpha1().Builders(desired.Namespace).UpdateStatus(desired)
	return err
}

func (c *Reconciler) reconcileBuilderStatus(builder *v1alpha1.Builder) (*v1alpha1.Builder, error) {
	builderImage, err := c.MetadataRetriever.GetBuilderImage(builder)
	if err != nil {
		builder.Status = v1alpha1.BuilderStatus{
			Status: duckv1alpha1.Status{
				ObservedGeneration: builder.Generation,
				Conditions: duckv1alpha1.Conditions{
					{
						Type:               duckv1alpha1.ConditionReady,
						Status:             corev1.ConditionFalse,
						Message:            err.Error(),
						LastTransitionTime: apis.VolatileTime{Inner: metav1.Now()},
					},
				},
			},
		}
		return builder, err
	}

	builder.Status = v1alpha1.BuilderStatus{
		Status: duckv1alpha1.Status{
			ObservedGeneration: builder.Generation,
			Conditions: duckv1alpha1.Conditions{
				{
					Type:               duckv1alpha1.ConditionReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: apis.VolatileTime{Inner: metav1.Now()},
				},
			},
		},
		BuilderMetadata: transform(builderImage.BuilderBuildpackMetadata),
		LatestImage:     builderImage.Identifier,
		Stack: v1alpha1.BuildStack{
			RunImage: builderImage.Stack.RunImage,
			ID:       builderImage.Stack.ID,
		},
	}
	return builder, nil
}

func transform(in cnb.BuilderMetadata) v1alpha1.BuildpackMetadataList {
	out := make(v1alpha1.BuildpackMetadataList, 0, len(in))

	for _, m := range in {
		out = append(out, v1alpha1.BuildpackMetadata{
			ID:      m.ID,
			Version: m.Version,
		})
	}

	return out
}
