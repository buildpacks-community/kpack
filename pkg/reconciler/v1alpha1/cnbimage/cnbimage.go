package cnbimage

import (
	"context"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	v1alpha1informers "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler"
)

const (
	ReconcilerName = "CNBImagess"
	Kind           = "CNBImage"
)

func NewController(opt reconciler.Options, cnbImageInformer v1alpha1informers.CNBImageInformer, cnbBuildInformer v1alpha1informers.CNBBuildInformer, cnbBuilderInformer v1alpha1informers.CNBBuilderInformer) *controller.Impl {
	c := &Reconciler{
		CNBClient:        opt.CNBClient,
		CNBImageLister:   cnbImageInformer.Lister(),
		CNBBuildLister:   cnbBuildInformer.Lister(),
		CNBBuilderLister: cnbBuilderInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	cnbImageInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	cnbBuildInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())

	cnbBuilderInformer.Informer().AddEventHandler(reconciler.Handler(controller.EnsureTypeMeta(
		c.Tracker.OnChanged,
		(&v1alpha1.CNBBuilder{}).GetGroupVersionKind(),
	)))

	return impl
}

type Reconciler struct {
	CNBClient        versioned.Interface
	CNBImageLister   v1alpha1Listers.CNBImageLister
	CNBBuildLister   v1alpha1Listers.CNBBuildLister
	CNBBuilderLister v1alpha1Listers.CNBBuilderLister
	Tracker          tracker.Interface
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, imageName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	image, err := c.CNBImageLister.CNBImages(namespace).Get(imageName)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	image = image.DeepCopy()

	lastBuild, err := c.fetchLastBuild(image)
	if err != nil {
		return err
	}

	if lastBuild.IsRunning() {
		return nil
	}

	builder, err := c.CNBBuilderLister.CNBBuilders(namespace).Get(image.Spec.BuilderRef)
	if err != nil {
		return err
	}

	err = c.Tracker.Track(builder.Ref(), image)
	if err != nil {
		return err
	}

	var build *v1alpha1.CNBBuild
	if image.BuildNeeded(lastBuild, builder) {
		build, err = c.CNBClient.BuildV1alpha1().CNBBuilds(image.Namespace).Create(image.CreateBuild(builder))
		if err != nil {
			return err
		}
	} else {
		build = lastBuild
	}

	image.Status.LastBuildRef = build.Name
	image.Status.ObservedGeneration = image.Generation

	_, err = c.CNBClient.BuildV1alpha1().CNBImages(namespace).UpdateStatus(image)
	if err != nil {
		return err
	}
	return err
}

func (c *Reconciler) fetchLastBuild(image *v1alpha1.CNBImage) (*v1alpha1.CNBBuild, error) {
	cnbImage, err := c.CNBBuildLister.CNBBuilds(image.Namespace).Get(image.Status.LastBuildRef)
	if errors.IsNotFound(err) {
		return nil, nil
	}
	return cnbImage, err
}
