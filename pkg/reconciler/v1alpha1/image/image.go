package image

import (
	"context"
	"fmt"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	coreinformers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler"
)

const (
	ReconcilerName           = "Images"
	Kind                     = "Image"
	buildHistoryDefaultLimit = 10
)

func NewController(opt reconciler.Options,
	k8sClient k8sclient.Interface,
	imageInformer v1alpha1informers.ImageInformer,
	buildInformer v1alpha1informers.BuildInformer,
	builderInformer v1alpha1informers.BuilderInformer,
	sourceResolverInformer v1alpha1informers.SourceResolverInformer,
	pvcInformer coreinformers.PersistentVolumeClaimInformer) *controller.Impl {
	c := &Reconciler{
		Client:               opt.Client,
		K8sClient:            k8sClient,
		ImageLister:          imageInformer.Lister(),
		BuildLister:          buildInformer.Lister(),
		BuilderLister:        builderInformer.Lister(),
		SourceResolverLister: sourceResolverInformer.Lister(),
		PvcLister:            pvcInformer.Lister(),
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	imageInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	buildInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	sourceResolverInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	pvcInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
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
	Client               versioned.Interface
	ImageLister          v1alpha1Listers.ImageLister
	BuildLister          v1alpha1Listers.BuildLister
	BuilderLister        v1alpha1Listers.BuilderLister
	SourceResolverLister v1alpha1Listers.SourceResolverLister
	PvcLister            corelisters.PersistentVolumeClaimLister
	Tracker              tracker.Interface
	K8sClient            k8sclient.Interface
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, imageName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("failed splitting meta namespace key: %s", err)
	}

	image, err := c.ImageLister.Images(namespace).Get(imageName)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed attempting to fetch image with name %s: %s", imageName, err)
	}
	image = image.DeepCopy()

	lastBuild, err := c.fetchLastBuild(image)
	if err != nil {
		return fmt.Errorf("failed fetching last build: %s", err)
	}

	if lastBuild.IsRunning() {
		return nil
	}

	sourceResolver, err := c.reconcileSourceResolver(image)
	if err != nil {
		return err
	}

	builder, err := c.BuilderLister.Builders(namespace).Get(image.Spec.BuilderRef)
	if err != nil {
		return fmt.Errorf("failed fetching builder: %s", err)
	}

	err = c.Tracker.Track(builder.Ref(), image)
	if err != nil {
		return fmt.Errorf("failed setting tracker for builder: %s", err)
	}

	buildCache, err := c.reconcileBuildCache(image)
	if err != nil {
		return err
	}

	if buildCache == nil {
		image.Status.BuildCacheName = ""
	} else {
		image.Status.BuildCacheName = buildCache.Name
	}

	buildApplier, err := image.ReconcileBuild(lastBuild, sourceResolver, builder)
	if err != nil {
		return err
	}

	reconciledBuild, err := buildApplier.Apply(c)
	if err != nil {
		return err
	}

	image.Status.LastBuildRef = reconciledBuild.Build.BuildRef()
	image.Status.BuildCounter = reconciledBuild.BuildCounter

	err = c.deleteOldBuilds(namespace, image)
	if err != nil {
		return fmt.Errorf("failed deleting build: %s", err)
	}

	image.Status.ObservedGeneration = image.Generation
	return c.updateStatus(image)
}

func (c *Reconciler) reconcileSourceResolver(image *v1alpha1.Image) (*v1alpha1.SourceResolver, error) {
	desiredSourceResolver := image.SourceResolver()

	sourceResolver, err := c.SourceResolverLister.SourceResolvers(image.Namespace).Get(image.SourceResolverName())
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	} else if errors.IsNotFound(err) {
		sourceResolver, err = c.Client.BuildV1alpha1().SourceResolvers(image.Namespace).Create(desiredSourceResolver)
		if err != nil {
			return nil, err
		}
	}

	if equality.Semantic.DeepEqual(desiredSourceResolver.Spec, sourceResolver.Spec) {
		return sourceResolver, nil
	}

	sourceResolver = sourceResolver.DeepCopy()
	sourceResolver.Spec = desiredSourceResolver.Spec
	return c.Client.BuildV1alpha1().SourceResolvers(image.Namespace).Update(sourceResolver)
}

func (c *Reconciler) reconcileBuildCache(image *v1alpha1.Image) (*corev1.PersistentVolumeClaim, error) {
	if !image.NeedCache() {
		buildCache, err := c.PvcLister.PersistentVolumeClaims(image.Namespace).Get(image.CacheName())
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		} else if errors.IsNotFound(err) {
			return nil, nil
		}

		return nil, c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Delete(image.CacheName(), &v1.DeleteOptions{
			Preconditions: &v1.Preconditions{UID: &buildCache.UID},
		})
	}

	desiredBuildCache := image.BuildCache()

	buildCache, err := c.PvcLister.PersistentVolumeClaims(image.Namespace).Get(image.CacheName())
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get image cache: %s", err)
	} else if errors.IsNotFound(err) {
		buildCache, err = c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Create(desiredBuildCache)
		if err != nil {
			return nil, fmt.Errorf("failed creating image cache for build: %s", err)
		}
	}

	if equality.Semantic.DeepEqual(desiredBuildCache.Spec.Resources, buildCache.Spec.Resources) {
		return buildCache, nil
	}

	existing := buildCache.DeepCopy()
	existing.Spec.Resources = desiredBuildCache.Spec.Resources
	existing.ObjectMeta.Labels = desiredBuildCache.ObjectMeta.Labels
	return c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Update(existing)
}

func (c *Reconciler) deleteOldBuilds(namespace string, image *v1alpha1.Image) error {
	builds, err := c.fetchAllBuilds(image)
	if err != nil {
		return fmt.Errorf("failed fetching all builds for image: %s", err)
	}

	if builds.NumberFailedBuilds() > limitOrDefault(image.Spec.FailedBuildHistoryLimit, buildHistoryDefaultLimit) {
		oldestFailedBuild := builds.OldestFailure()

		err := c.Client.BuildV1alpha1().Builds(namespace).Delete(oldestFailedBuild.Name, &v1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting failed build: %s", err)
		}
	}

	if builds.NumberSuccessfulBuilds() > limitOrDefault(image.Spec.SuccessBuildHistoryLimit, buildHistoryDefaultLimit) {
		oldestSuccess := builds.OldestSuccess()

		err := c.Client.BuildV1alpha1().Builds(namespace).Delete(oldestSuccess.Name, &v1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting successful build: %s", err)
		}
	}

	return nil
}

func limitOrDefault(limit *int64, defaultLimit int64) int64 {
	if limit != nil {
		return *limit
	}
	return defaultLimit
}

func (c *Reconciler) fetchAllBuilds(image *v1alpha1.Image) (buildList, error) {
	imageNameReq, err := labels.NewRequirement(v1alpha1.ImageLabel, selection.DoubleEquals, []string{image.Name})
	if err != nil {
		return buildList{}, fmt.Errorf("image name requirement: %s", err)
	}

	add := labels.NewSelector().Add(*imageNameReq)
	builds, err := c.BuildLister.Builds(image.Namespace).List(add)
	if err != nil {
		return buildList{}, fmt.Errorf("list builds: %s", err)
	}

	return newBuildList(builds)
}

func (c *Reconciler) fetchLastBuild(image *v1alpha1.Image) (*v1alpha1.Build, error) {
	builds, err := c.fetchAllBuilds(image)
	if err != nil {
		return nil, err
	}

	return builds.lastBuild, nil
}

func (c *Reconciler) updateStatus(desired *v1alpha1.Image) error {
	original, err := c.ImageLister.Images(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.BuildV1alpha1().Images(desired.Namespace).UpdateStatus(desired)
	return err
}

func (c *Reconciler) CreateBuild(build *v1alpha1.Build) (*v1alpha1.Build, error) {
	return c.Client.BuildV1alpha1().Builds(build.Namespace()).Create(build)
}
