package cnbimage

import (
	"context"

	"github.com/knative/pkg/controller"
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

func NewController(opt reconciler.Options, cnbClient versioned.Interface, cnbImageInformer v1alpha1informers.CNBImageInformer, cnbBuildInformer v1alpha1informers.CNBBuildInformer) *controller.Impl {
	c := &Reconciler{
		CNBClient:      cnbClient,
		CNBImageLister: cnbImageInformer.Lister(),
		CNBBuildLister: cnbBuildInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	cnbImageInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	cnbBuildInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

type Reconciler struct {
	CNBClient      versioned.Interface
	CNBImageLister v1alpha1Listers.CNBImageLister
	CNBBuildLister v1alpha1Listers.CNBBuildLister
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

	var build *v1alpha1.CNBBuild
	if image.BuildNeeded(lastBuild) {
		build, err = c.CNBClient.BuildV1alpha1().CNBBuilds(image.Namespace).Create(image.CreateBuild())
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
