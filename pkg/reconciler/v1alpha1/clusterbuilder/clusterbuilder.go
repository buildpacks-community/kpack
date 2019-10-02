package clusterbuilder

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
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

	builder = c.reconcileClusterBuilderStatus(builder)

	err = c.updateClusterBuilderStatus(builder)
	if err != nil {
		return err
	}

	if builder.Spec.UpdatePolicy != v1alpha1.External {
		err = c.Enqueuer.Enqueue(builder)
		if err != nil {
			return err
		}
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

func (c *Reconciler) reconcileClusterBuilderStatus(builder *v1alpha1.ClusterBuilder) *v1alpha1.ClusterBuilder {
	builderImage, err := c.MetadataRetriever.GetBuilderImage(builder)
	if err != nil {
		builder.Status = v1alpha1.BuilderStatus{
			Status: duckv1alpha1.Status{
				ObservedGeneration: builder.Generation,
				Conditions: duckv1alpha1.Conditions{
					{
						Type:    duckv1alpha1.ConditionReady,
						Status:  corev1.ConditionFalse,
						Message: err.Error(),
					},
				},
			},
		}
		return builder
	}

	builder.Status = v1alpha1.BuilderStatus{
		Status: duckv1alpha1.Status{
			ObservedGeneration: builder.Generation,
			Conditions: duckv1alpha1.Conditions{
				{
					Type:   duckv1alpha1.ConditionReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
		BuilderMetadata: transform(builderImage.BuilderBuildpackMetadata),
		LatestImage:     builderImage.Identifier,
		RunImage:        builderImage.RunImage,
	}
	return builder
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
