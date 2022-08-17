package image

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	coreinformers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging/logkey"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/duckbuilder"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/tracker"
)

const (
	ReconcilerName = "Images"
	Kind           = "Image"
)

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	k8sClient k8sclient.Interface,
	imageInformer buildinformers.ImageInformer,
	buildInformer buildinformers.BuildInformer,
	duckbuilderInformer *duckbuilder.DuckBuilderInformer,
	sourceResolverInformer buildinformers.SourceResolverInformer,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	enablePriorityClasses bool,
) *controller.Impl {
	c := &Reconciler{
		Client:                opt.Client,
		K8sClient:             k8sClient,
		ImageLister:           imageInformer.Lister(),
		BuildLister:           buildInformer.Lister(),
		DuckBuilderLister:     duckbuilderInformer.Lister(),
		SourceResolverLister:  sourceResolverInformer.Lister(),
		PvcLister:             pvcInformer.Lister(),
		EnablePriorityClasses: enablePriorityClasses,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.ImageCRName),
	)

	impl := controller.NewContext(ctx, c, controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger})

	imageInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	buildInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(buildapi.SchemeGroupVersion.WithKind(Kind).GroupKind()),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	sourceResolverInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(buildapi.SchemeGroupVersion.WithKind(Kind).GroupKind()),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	pvcInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(buildapi.SchemeGroupVersion.WithKind(Kind).GroupKind()),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())

	duckbuilderInformer.AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))

	return impl
}

type Reconciler struct {
	Client                versioned.Interface
	DuckBuilderLister     *duckbuilder.DuckBuilderLister
	ImageLister           buildlisters.ImageLister
	BuildLister           buildlisters.BuildLister
	SourceResolverLister  buildlisters.SourceResolverLister
	PvcLister             corelisters.PersistentVolumeClaimLister
	Tracker               reconciler.Tracker
	K8sClient             k8sclient.Interface
	EnablePriorityClasses bool
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, imageName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("failed splitting meta namespace key: %s", err)
	}

	image, err := c.ImageLister.Images(namespace).Get(imageName)
	if k8serrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	image = image.DeepCopy()
	image.SetDefaults(ctx)

	image, err = c.reconcileImage(ctx, image)
	if err != nil {
		return err
	}

	return c.updateStatus(ctx, image)
}

func (c *Reconciler) reconcileImage(ctx context.Context, image *buildapi.Image) (*buildapi.Image, error) {
	builder, err := c.DuckBuilderLister.Namespace(image.Namespace).Get(image.Spec.Builder)
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	} else if k8serrors.IsNotFound(err) {
		image.Status.Conditions = image.BuilderNotFound()
		image.Status.ObservedGeneration = image.Generation
		return image, nil
	}

	err = c.Tracker.Track(builder, image.NamespacedName())
	if err != nil {
		return nil, err
	}

	lastBuild, err := c.fetchLastBuild(image)
	if err != nil {
		return nil, err
	}

	if lastBuild.IsRunning() {
		return image, nil
	}

	buildCacheName, err := c.reconcileBuildCache(ctx, image)
	if err != nil {
		return nil, err
	}

	sourceResolver, err := c.reconcileSourceResolver(ctx, image)
	if err != nil {
		return nil, err
	}

	image.Status, err = c.reconcileBuild(ctx, image, lastBuild, sourceResolver, builder, buildCacheName)
	if err != nil {
		return nil, err
	}

	return image, c.deleteOldBuilds(ctx, image)
}

func (c *Reconciler) reconcileSourceResolver(ctx context.Context, image *buildapi.Image) (*buildapi.SourceResolver, error) {
	desiredSourceResolver := image.SourceResolver()

	sourceResolver, err := c.SourceResolverLister.SourceResolvers(image.Namespace).Get(image.SourceResolverName())
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "cannot retrieve source resolver")
	} else if k8serrors.IsNotFound(err) {
		sourceResolver, err = c.Client.KpackV1alpha2().SourceResolvers(image.Namespace).Create(ctx, desiredSourceResolver, metav1.CreateOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "cannot create source resolver")
		}
	}

	if sourceResolversEqual(desiredSourceResolver, sourceResolver) {
		return sourceResolver, nil
	}

	sourceResolver = sourceResolver.DeepCopy()
	sourceResolver.Spec = desiredSourceResolver.Spec
	sourceResolver.Labels = desiredSourceResolver.Labels
	return c.Client.KpackV1alpha2().SourceResolvers(image.Namespace).Update(ctx, sourceResolver, metav1.UpdateOptions{})
}

func (c *Reconciler) reconcileBuildCache(ctx context.Context, image *buildapi.Image) (string, error) {
	if !image.Spec.NeedVolumeCache() {
		buildCache, err := c.PvcLister.PersistentVolumeClaims(image.Namespace).Get(image.CacheName())
		if err != nil && !k8serrors.IsNotFound(err) {
			return "", errors.Wrap(err, "cannot retrieve persistent volume claim")
		} else if k8serrors.IsNotFound(err) {
			return "", nil
		}

		return "", c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Delete(ctx, image.CacheName(), metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{UID: &buildCache.UID},
		})
	}

	desiredBuildCache := image.BuildCache()

	buildCache, err := c.PvcLister.PersistentVolumeClaims(image.Namespace).Get(image.CacheName())
	if err != nil && !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("failed to get image cache: %s", err)
	} else if k8serrors.IsNotFound(err) {
		buildCache, err = c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Create(ctx, desiredBuildCache, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed creating image cache for build: %s", err)
		}
	}

	if buildCacheEqual(desiredBuildCache, buildCache) {
		return buildCache.Name, nil
	}

	existing := buildCache.DeepCopy()
	existing.Spec.Resources = desiredBuildCache.Spec.Resources
	existing.ObjectMeta.Labels = desiredBuildCache.ObjectMeta.Labels
	_, err = c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return existing.Name, errors.Wrap(err, "cannot update persistent volume claim")
}

func (c *Reconciler) deleteOldBuilds(ctx context.Context, image *buildapi.Image) error {
	builds, err := c.fetchAllBuilds(image)
	if err != nil {
		return fmt.Errorf("failed fetching all builds for image: %s", err)
	}

	if builds.NumberFailedBuilds() > *image.Spec.FailedBuildHistoryLimit {
		oldestFailedBuild := builds.OldestFailure()

		err := c.Client.KpackV1alpha2().Builds(image.Namespace).Delete(ctx, oldestFailedBuild.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting failed build: %s", err)
		}
	}

	if builds.NumberSuccessfulBuilds() > *image.Spec.SuccessBuildHistoryLimit {
		oldestSuccess := builds.OldestSuccess()

		err := c.Client.KpackV1alpha2().Builds(image.Namespace).Delete(ctx, oldestSuccess.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting successful build: %s", err)
		}
	}

	return nil
}

func (c *Reconciler) fetchAllBuilds(image *buildapi.Image) (buildList, error) {
	imageNameReq, err := labels.NewRequirement(buildapi.ImageLabel, selection.DoubleEquals, []string{image.Name})
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

func (c *Reconciler) fetchLastBuild(image *buildapi.Image) (*buildapi.Build, error) {
	builds, err := c.fetchAllBuilds(image)
	if err != nil {
		return nil, err
	}
	return builds.lastBuild, nil
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *buildapi.Image) error {
	desired.Status.ObservedGeneration = desired.Generation
	original, err := c.ImageLister.Images(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().Images(desired.Namespace).UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func sourceResolversEqual(desiredSourceResolver *buildapi.SourceResolver, sourceResolver *buildapi.SourceResolver) bool {
	return equality.Semantic.DeepEqual(desiredSourceResolver.Spec, sourceResolver.Spec) &&
		equality.Semantic.DeepEqual(desiredSourceResolver.Labels, sourceResolver.Labels)
}

func buildCacheEqual(desiredBuildCache *corev1.PersistentVolumeClaim, buildCache *corev1.PersistentVolumeClaim) bool {
	return equality.Semantic.DeepEqual(desiredBuildCache.Spec.Resources, buildCache.Spec.Resources) &&
		equality.Semantic.DeepEqual(desiredBuildCache.Labels, buildCache.Labels)
}
