package clusterbuilder

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	kpackcore "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "ClusterBuilders"
	Kind           = "ClusterBuilder"
)

//go:generate counterfeiter . MetadataRetriever
type MetadataRetriever interface {
	GetBuilderImage(builder cnb.FetchableBuilder) (v1alpha1.BuilderRecord, error)
}

func NewController(opt reconciler.Options, clusterBuilderInformer v1alpha1informers.ClusterBuilderInformer, metadataRetriever MetadataRetriever) *controller.Impl {
	c := &Reconciler{
		Client:               opt.Client,
		MetadataRetriever:    metadataRetriever,
		ClusterBuilderLister: clusterBuilderInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	c.Enqueuer = &workQueueEnqueuer{
		enqueueAfter: impl.EnqueueAfter,
		delay:        opt.BuilderPollingFrequency,
	}

	clusterBuilderInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	return impl
}

//go:generate counterfeiter . Enqueuer
type Enqueuer interface {
	Enqueue(builder *v1alpha1.ClusterBuilder) error
}

type Reconciler struct {
	Client               versioned.Interface
	MetadataRetriever    MetadataRetriever
	Enqueuer             Enqueuer
	ClusterBuilderLister v1alpha1Listers.ClusterBuilderLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	_, builderName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	builder, err := c.ClusterBuilderLister.Get(builderName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	builder = builder.DeepCopy()
	builder.SetDefaults(ctx)

	builder, err = c.reconcileClusterBuilderStatus(builder)

	updateErr := c.updateClusterBuilderStatus(builder)
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

func (c *Reconciler) updateClusterBuilderStatus(desired *v1alpha1.ClusterBuilder) error {
	original, err := c.ClusterBuilderLister.Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(desired.Status, original.Status) { //this is a bug :(
		return nil
	}

	_, err = c.Client.BuildV1alpha1().ClusterBuilders().UpdateStatus(desired)
	return err
}

func (c *Reconciler) reconcileClusterBuilderStatus(builder *v1alpha1.ClusterBuilder) (*v1alpha1.ClusterBuilder, error) {
	builderRecord, err := c.MetadataRetriever.GetBuilderImage(builder)
	if err != nil {
		builder.Status = v1alpha1.BuilderStatus{
			Status: kpackcore.Status{
				ObservedGeneration: builder.Generation,
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
		return builder, err
	}

	builder.Status = v1alpha1.BuilderStatus{
		Status: kpackcore.Status{
			ObservedGeneration: builder.Generation,
			Conditions: kpackcore.Conditions{
				{
					LastTransitionTime: kpackcore.VolatileTime{Inner: v1.Now()},
					Type:               kpackcore.ConditionReady,
					Status:             corev1.ConditionTrue,
				},
			},
		},
		BuilderMetadata: builderRecord.Buildpacks,
		LatestImage:     builderRecord.Image,
		Stack:           builderRecord.Stack,
	}
	return builder, nil
}
