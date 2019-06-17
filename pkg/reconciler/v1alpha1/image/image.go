package image

import (
	"context"
	"fmt"
	"strconv"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	v1alpha1informers "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler"
)

const (
	ReconcilerName = "Images"
	Kind           = "Image"
)

func NewController(opt reconciler.Options, imageInformer v1alpha1informers.ImageInformer, buildInformer v1alpha1informers.BuildInformer, builderInformer v1alpha1informers.BuilderInformer) *controller.Impl {
	c := &Reconciler{
		Client:        opt.Client,
		ImageLister:   imageInformer.Lister(),
		BuildLister:   buildInformer.Lister(),
		BuilderLister: builderInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	imageInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	buildInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())

	builderInformer.Informer().AddEventHandler(reconciler.Handler(controller.EnsureTypeMeta(
		c.Tracker.OnChanged,
		(&v1alpha1.Builder{}).GetGroupVersionKind(),
	)))

	return impl
}

type Reconciler struct {
	Client        versioned.Interface
	ImageLister   v1alpha1Listers.ImageLister
	BuildLister   v1alpha1Listers.BuildLister
	BuilderLister v1alpha1Listers.BuilderLister
	Tracker       tracker.Interface
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, imageName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	image, err := c.ImageLister.Images(namespace).Get(imageName)
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

	builder, err := c.BuilderLister.Builders(namespace).Get(image.Spec.BuilderRef)
	if err != nil {
		return err
	}

	err = c.Tracker.Track(builder.Ref(), image)
	if err != nil {
		return err
	}

	var build *v1alpha1.Build
	if image.BuildNeeded(lastBuild, builder) {
		build, err = c.Client.BuildV1alpha1().Builds(image.Namespace).Create(image.CreateBuild(builder))
		if err != nil {
			return err
		}
		image.Status.BuildCounter = image.Status.BuildCounter + 1
	} else {
		build = lastBuild
	}

	image.Status.LastBuildRef = build.Name
	image.Status.ObservedGeneration = image.Generation

	_, err = c.Client.BuildV1alpha1().Images(namespace).UpdateStatus(image)
	if err != nil {
		return err
	}
	return err
}

func (c *Reconciler) fetchLastBuild(image *v1alpha1.Image) (*v1alpha1.Build, error) {
	currentBuildNumber := strconv.Itoa(currentBuildNumber(image))
	currentBuildNumberReq, err := labels.NewRequirement(v1alpha1.BuildNumberLabel, selection.GreaterThan, []string{currentBuildNumber})
	if err != nil {
		return nil, err
	}

	imageNameReq, err := labels.NewRequirement(v1alpha1.ImageLabel, selection.DoubleEquals, []string{image.Name})
	if err != nil {
		return nil, err
	}

	builds, err := c.BuildLister.Builds(image.Namespace).List(labels.NewSelector().Add(*currentBuildNumberReq).Add(*imageNameReq))
	if err != nil {
		return nil, err
	}

	if len(builds) == 0 {
		return nil, nil
	} else if len(builds) > 1 || builds[0].Name != image.Status.LastBuildRef {
		return nil, fmt.Errorf("warning: image %s status not up to date", image.Name) //what error type should we use?
	}

	return builds[0], err
}

func currentBuildNumber(image *v1alpha1.Image) int {
	buildNumber := int(image.Status.BuildCounter - 1)
	if buildNumber < 0 {
		return 0
	}
	return buildNumber
}
