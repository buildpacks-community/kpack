package build

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1Informers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	v1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informer "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1lister "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "Builds"
	Kind           = "Build"
)

//go:generate counterfeiter . MetadataRetriever
type MetadataRetriever interface {
	GetBuiltImage(repoName *v1alpha1.Build) (cnb.BuiltImage, error)
}

type PodGenerator interface {
	Generate(build buildpod.BuildPodable) (*corev1.Pod, error)
}

func NewController(opt reconciler.Options, k8sClient k8sclient.Interface, informer v1alpha1informer.BuildInformer, podInformer corev1Informers.PodInformer, metadataRetriever MetadataRetriever, podGenerator PodGenerator) *controller.Impl {
	c := &Reconciler{
		Client:            opt.Client,
		K8sClient:         k8sClient,
		MetadataRetriever: metadataRetriever,
		Lister:            informer.Lister(),
		PodLister:         podInformer.Lister(),
		PodGenerator:      podGenerator,
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	informer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGVK(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

type Reconciler struct {
	Client            versioned.Interface
	Lister            v1alpha1lister.BuildLister
	MetadataRetriever MetadataRetriever
	K8sClient         k8sclient.Interface
	PodLister         v1Listers.PodLister
	PodGenerator      PodGenerator
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, buildName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	build, err := c.Lister.Builds(namespace).Get(buildName)
	if k8s_errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	build = build.DeepCopy()
	build.SetDefaults(ctx)

	err = c.reconcile(ctx, build)
	if err != nil && !controller.IsPermanentError(err) {
		return err
	} else if controller.IsPermanentError(err) {
		build.Status.Error(err)
	}

	return c.updateStatus(ctx, build)
}

func (c *Reconciler) reconcile(ctx context.Context, build *v1alpha1.Build) error {
	if build.Finished() {
		return nil
	}

	pod, err := c.reconcileBuildPod(ctx, build)
	if err != nil {
		return err
	}

	if build.MetadataReady(pod) {
		image, err := c.MetadataRetriever.GetBuiltImage(build)
		if err != nil {
			return err
		}

		build.Status.BuildMetadata = buildMetadataFromBuiltImage(image)
		build.Status.LatestImage = image.Identifier
		build.Status.Stack.RunImage = image.Stack.RunImage
		build.Status.Stack.ID = image.Stack.ID
	}

	build.Status.PodName = pod.Name
	build.Status.StepStates = stepStates(pod)
	build.Status.StepsCompleted = stepCompleted(pod)
	build.Status.Conditions = conditionForPod(pod)
	return nil
}

func (c *Reconciler) reconcileBuildPod(ctx context.Context, build *v1alpha1.Build) (*corev1.Pod, error) {
	pod, err := c.PodLister.Pods(build.Namespace).Get(build.PodName())
	if err != nil && !k8s_errors.IsNotFound(err) {
		return nil, err
	} else if !k8s_errors.IsNotFound(err) {
		return pod, nil
	}

	podConfig, err := c.PodGenerator.Generate(build)
	if err != nil {
		return nil, controller.NewPermanentError(err)
	}
	return c.K8sClient.CoreV1().Pods(build.Namespace).Create(ctx, podConfig, metav1.CreateOptions{})
}

func conditionForPod(pod *corev1.Pod) corev1alpha1.Conditions {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionSucceeded,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
		}
	case corev1.PodFailed:
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionSucceeded,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
		}
	case corev1.PodPending:
		for _, c := range pod.Status.InitContainerStatuses {
			if c.State.Waiting != nil {
				return corev1alpha1.Conditions{
					{
						Type:               corev1alpha1.ConditionSucceeded,
						Status:             corev1.ConditionUnknown,
						Reason:             c.State.Waiting.Reason,
						Message:            c.State.Waiting.Message,
						LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
					},
				}
			}
		}
		fallthrough
	default:
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionSucceeded,
				Status:             corev1.ConditionUnknown,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
		}
	}
}

func stepStates(pod *corev1.Pod) []corev1.ContainerState {
	states := make([]corev1.ContainerState, 0, len(pod.Status.InitContainerStatuses))
	for _, s := range pod.Status.InitContainerStatuses {
		states = append(states, s.State)
	}
	return states
}

func stepCompleted(pod *corev1.Pod) []string {
	completed := make([]string, 0, len(pod.Status.InitContainerStatuses))
	for _, s := range pod.Status.InitContainerStatuses {
		if s.State.Terminated != nil {
			completed = append(completed, s.Name)
		}
	}
	return completed
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Build) error {
	desired.Status.ObservedGeneration = desired.Generation
	original, err := c.Lister.Builds(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha1().Builds(desired.Namespace).UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func buildMetadataFromBuiltImage(image cnb.BuiltImage) []v1alpha1.BuildpackMetadata {
	buildpackMetadata := make([]v1alpha1.BuildpackMetadata, 0, len(image.BuildpackMetadata))
	for _, metadata := range image.BuildpackMetadata {
		buildpackMetadata = append(buildpackMetadata, v1alpha1.BuildpackMetadata{
			Id:       metadata.ID,
			Version:  metadata.Version,
			Homepage: metadata.Homepage,
		})
	}
	return buildpackMetadata
}
