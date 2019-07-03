package image

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	"github.com/knative/pkg/tracker"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	v1alpha1build "github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build"
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

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

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

	if image.NeedCache() {
		buildCache, err := c.reconcileBuildCache(image)
		if err != nil {
			return err
		}

		if buildCache == nil {
			image.Status.BuildCacheName = ""
		} else {
			image.Status.BuildCacheName = buildCache.Name
		}
	} else {
		image.Status.BuildCacheName = ""
	}

	var build *v1alpha1.Build
	if image.BuildNeeded(sourceResolver, lastBuild, builder) {
		build, err = c.Client.BuildV1alpha1().Builds(image.Namespace).Create(image.CreateBuild(sourceResolver, builder))
		if err != nil {
			return fmt.Errorf("failed creating build: %s", err)
		}
		image.Status.BuildCounter = image.Status.BuildCounter + 1
	} else {
		build = lastBuild
	}

	image.Status.LastBuildRef = build.BuildRef()
	image.Status.ObservedGeneration = image.Generation

	_, err = c.Client.BuildV1alpha1().Images(namespace).UpdateStatus(image)
	if err != nil {
		return fmt.Errorf("failed updating image status: %s", err)
	}

	err = c.deleteOldBuilds(namespace, image)
	if err != nil {
		return fmt.Errorf("failed deleting build: %s", err)
	}
	return nil
}

func (c *Reconciler) reconcileSourceResolver(image *v1alpha1.Image) (*v1alpha1.SourceResolver, error) {
	name := reconciler.ChildName(image.Name, "-source")

	desiredSourceResolver := &v1alpha1.SourceResolver{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
			OwnerReferences: []v1.OwnerReference{ //untested. Test me please :)
				*kmeta.NewControllerRef(image),
			},
		},
		Spec: v1alpha1.SourceResolverSpec{
			ServiceAccount: image.Spec.ServiceAccount,
			Source:         image.Spec.Source,
		},
	}

	sourceResolver, err := c.SourceResolverLister.SourceResolvers(image.Namespace).Get(name)
	if err != nil && !errors.IsNotFound(err) {
		return sourceResolver, err
	} else if errors.IsNotFound(err) {
		sourceResolver, err = c.Client.BuildV1alpha1().SourceResolvers(image.Namespace).Create(desiredSourceResolver)
		if err != nil {
			return sourceResolver, err
		}
	}

	if equality.Semantic.DeepEqual(desiredSourceResolver.Spec, sourceResolver.Spec) {
		return sourceResolver, nil
	}

	sourceResolver = sourceResolver.DeepCopy()
	sourceResolver.Spec = desiredSourceResolver.Spec
	_, err = c.Client.BuildV1alpha1().SourceResolvers(image.Namespace).Update(sourceResolver)
	return sourceResolver, err
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
		}) //please test me :(
	}

	desiredBuildCache := image.MakeBuildCache()

	buildCache, err := c.PvcLister.PersistentVolumeClaims(image.Namespace).Get(image.Status.BuildCacheName)
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

	var successBuilds []*v1alpha1.Build
	var failedBuilds []*v1alpha1.Build

	for _, build := range builds {
		if build.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() {
			successBuilds = append(successBuilds, build)
		} else if build.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsFalse() {
			failedBuilds = append(failedBuilds, build)
		}
	}

	var limit int64 = buildHistoryDefaultLimit
	if image.Spec.FailedBuildHistoryLimit != nil {
		limit = *image.Spec.FailedBuildHistoryLimit
	}
	if len(failedBuilds) > 0 && int64(len(failedBuilds)) > limit {
		err := c.Client.BuildV1alpha1().Builds(namespace).Delete(failedBuilds[0].Name, &v1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting failed build: %s", err)
		}
	}
	limit = buildHistoryDefaultLimit
	if image.Spec.SuccessBuildHistoryLimit != nil {
		limit = *image.Spec.SuccessBuildHistoryLimit
	}
	if len(successBuilds) > 0 && int64(len(successBuilds)) > limit {
		err := c.Client.BuildV1alpha1().Builds(namespace).Delete(successBuilds[0].Name, &v1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed deleting successful build: %s", err)
		}
	}
	return nil
}

func (c *Reconciler) fetchAllBuilds(image *v1alpha1.Image) ([]*v1alpha1.Build, error) {
	imageNameReq, err := labels.NewRequirement(v1alpha1.ImageLabel, selection.DoubleEquals, []string{image.Name})
	if err != nil {
		return nil, fmt.Errorf("image name requirement: %s", err)
	}

	builds, err := c.BuildLister.Builds(image.Namespace).List(labels.NewSelector().Add(*imageNameReq))
	if err != nil {
		return nil, fmt.Errorf("list builds: %s", err)
	}
	sort.Sort(v1alpha1build.ByCreationTimestamp(builds))
	return builds, nil
}

func (c *Reconciler) fetchLastBuild(image *v1alpha1.Image) (*v1alpha1.Build, error) {
	currentBuildNumber := strconv.Itoa(currentBuildNumber(image))
	currentBuildNumberReq, err := labels.NewRequirement(v1alpha1.BuildNumberLabel, selection.GreaterThan, []string{currentBuildNumber})
	if err != nil {
		return nil, fmt.Errorf("current build number requirement: %s", err)
	}

	imageNameReq, err := labels.NewRequirement(v1alpha1.ImageLabel, selection.DoubleEquals, []string{image.Name})
	if err != nil {
		return nil, fmt.Errorf("image name requirement: %s", err)
	}

	builds, err := c.BuildLister.Builds(image.Namespace).List(labels.NewSelector().Add(*currentBuildNumberReq).Add(*imageNameReq))
	if err != nil {
		return nil, fmt.Errorf("list builds: %s", err)
	}

	if len(builds) == 0 {
		return nil, nil
	} else if len(builds) > 1 || builds[0].Name != image.Status.LastBuildRef {
		return nil, fmt.Errorf("warning: image %s status not up to date", image.Name) // what error type should we use?
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
