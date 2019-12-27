package image

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
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

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/duckbuilder"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/tracker"
)

const (
	ReconcilerName = "Images"
	Kind           = "Image"
)

func NewController(
	opt reconciler.Options,
	k8sClient k8sclient.Interface,
	imageInformer v1alpha1informers.ImageInformer,
	buildInformer v1alpha1informers.BuildInformer,
	duckbuilderInformer *duckbuilder.DuckBuilderInformer,
	sourceResolverInformer v1alpha1informers.SourceResolverInformer,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
) *controller.Impl {
	c := &Reconciler{
		Client:               opt.Client,
		K8sClient:            k8sClient,
		ImageLister:          imageInformer.Lister(),
		BuildLister:          buildInformer.Lister(),
		DuckBuilderLister:    duckbuilderInformer.Lister(),
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

	duckbuilderInformer.AddEventHandler(reconciler.Handler(c.Tracker.OnChanged))

	return impl
}

type Reconciler struct {
	Client               versioned.Interface
	DuckBuilderLister    *duckbuilder.DuckBuilderLister
	ImageLister          v1alpha1Listers.ImageLister
	BuildLister          v1alpha1Listers.BuildLister
	SourceResolverLister v1alpha1Listers.SourceResolverLister
	PvcLister            corelisters.PersistentVolumeClaimLister
	Tracker              reconciler.Tracker
	K8sClient            k8sclient.Interface
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
		return fmt.Errorf("failed attempting to fetch image with name %s: %s", imageName, err)
	}

	image = image.DeepCopy()
	image.SetDefaults(ctx)

	image, err = c.reconcileImage(image)
	if err != nil {
		return err
	}

	return c.updateStatus(image)
}

func (c *Reconciler) reconcileImage(image *v1alpha1.Image) (*v1alpha1.Image, error) {
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

	image.Status.BuildCacheName, err = c.reconcileBuildCache(image)
	if err != nil {
		return nil, err
	}

	sourceResolver, err := c.reconcileSourceResolver(image)
	if err != nil {
		return nil, err
	}

	buildApplier, err := image.ReconcileBuild(lastBuild, sourceResolver, builder)
	if err != nil {
		return nil, err
	}

	reconciledBuild, err := buildApplier.Apply(c)
	if err != nil {
		return nil, err
	}

	image.Status.LatestBuildRef = reconciledBuild.Build.BuildRef()
	image.Status.BuildCounter = reconciledBuild.BuildCounter
	image.Status.LatestImage = reconciledBuild.LatestImage
	image.Status.LatestStack = reconciledBuild.Build.Stack()
	image.Status.Conditions = reconciledBuild.Conditions
	image.Status.ObservedGeneration = image.Generation

	return image, c.deleteOldBuilds(image)
}

func (c *Reconciler) reconcileSourceResolver(image *v1alpha1.Image) (*v1alpha1.SourceResolver, error) {
	desiredSourceResolver := image.SourceResolver()

	sourceResolver, err := c.SourceResolverLister.SourceResolvers(image.Namespace).Get(image.SourceResolverName())
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, errors.Wrap(err, "cannot retrieve source resolver")
	} else if k8serrors.IsNotFound(err) {
		sourceResolver, err = c.Client.BuildV1alpha1().SourceResolvers(image.Namespace).Create(desiredSourceResolver)
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
	return c.Client.BuildV1alpha1().SourceResolvers(image.Namespace).Update(sourceResolver)
}

func (c *Reconciler) reconcileBuildCache(image *v1alpha1.Image) (string, error) {
	if !image.NeedCache() {
		buildCache, err := c.PvcLister.PersistentVolumeClaims(image.Namespace).Get(image.CacheName())
		if err != nil && !k8serrors.IsNotFound(err) {
			return "", errors.Wrap(err, "cannot retrieve persistent volume claim")
		} else if k8serrors.IsNotFound(err) {
			return "", nil
		}

		return "", c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Delete(image.CacheName(), &metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{UID: &buildCache.UID},
		})
	}

	desiredBuildCache := image.BuildCache()

	buildCache, err := c.PvcLister.PersistentVolumeClaims(image.Namespace).Get(image.CacheName())
	if err != nil && !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("failed to get image cache: %s", err)
	} else if k8serrors.IsNotFound(err) {
		buildCache, err = c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Create(desiredBuildCache)
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
	_, err = c.K8sClient.CoreV1().PersistentVolumeClaims(image.Namespace).Update(existing)
	return existing.Name, errors.Wrap(err, "cannot update persistent volume claim")
}

func (c *Reconciler) deleteOldBuilds(image *v1alpha1.Image) error {
	builds, err := c.fetchAllBuilds(image)
	if err != nil {
		return fmt.Errorf("failed fetching all builds for image: %s", err)
	}

	if builds.NumberFailedBuilds() > *image.Spec.FailedBuildHistoryLimit {
		oldestFailedBuild := builds.OldestFailure()

		err := c.Client.BuildV1alpha1().Builds(image.Namespace).Delete(oldestFailedBuild.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting failed build: %s", err)
		}
	}

	if builds.NumberSuccessfulBuilds() > *image.Spec.SuccessBuildHistoryLimit {
		oldestSuccess := builds.OldestSuccess()

		err := c.Client.BuildV1alpha1().Builds(image.Namespace).Delete(oldestSuccess.Name, &metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting successful build: %s", err)
		}
	}

	return nil
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

func sourceResolversEqual(desiredSourceResolver *v1alpha1.SourceResolver, sourceResolver *v1alpha1.SourceResolver) bool {
	return equality.Semantic.DeepEqual(desiredSourceResolver.Spec, sourceResolver.Spec) &&
		equality.Semantic.DeepEqual(desiredSourceResolver.Labels, sourceResolver.Labels)
}

func buildCacheEqual(desiredBuildCache *corev1.PersistentVolumeClaim, buildCache *corev1.PersistentVolumeClaim) bool {
	return equality.Semantic.DeepEqual(desiredBuildCache.Spec.Resources, buildCache.Spec.Resources) &&
		equality.Semantic.DeepEqual(desiredBuildCache.Labels, buildCache.Labels)
}

func (c *Reconciler) CreateBuild(build *v1alpha1.Build) (*v1alpha1.Build, error) {
	return c.Client.BuildV1alpha1().Builds(build.Namespace).Create(build)
}
