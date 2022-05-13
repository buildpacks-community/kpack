package build

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1Informers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	v1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/reconciler"
)

const (
	ReconcilerName = "Builds"
	Kind           = "Build"
)

type metadataDecompressor interface {
	Decompress(string) (*BuildStatusMetadata, error)
}

type PodGenerator interface {
	Generate(context.Context, buildpod.BuildPodable) (*corev1.Pod, error)
}

func NewController(opt reconciler.Options, k8sClient k8sclient.Interface, informer buildinformers.BuildInformer, podInformer corev1Informers.PodInformer, podGenerator PodGenerator) *controller.Impl {
	c := &Reconciler{
		Client:               opt.Client,
		K8sClient:            k8sClient,
		Lister:               informer.Lister(),
		metadataDecompressor: &GzipMetadataCompressor{},
		PodLister:            podInformer.Lister(),
		PodGenerator:         podGenerator,
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName)

	informer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(buildapi.SchemeGroupVersion.WithKind(Kind).GroupKind()),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

type Reconciler struct {
	Client               versioned.Interface
	Lister               buildlisters.BuildLister
	K8sClient            k8sclient.Interface
	PodLister            v1Listers.PodLister
	PodGenerator         PodGenerator
	metadataDecompressor metadataDecompressor
}

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, buildName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	build, err := r.Lister.Builds(namespace).Get(buildName)
	if k8s_errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	build = build.DeepCopy()
	build.SetDefaults(ctx)

	err = r.reconcile(ctx, build)
	if err != nil && !controller.IsPermanentError(err) {
		return err
	} else if controller.IsPermanentError(err) {
		build.Status.Error(err)
	}

	return r.updateStatus(ctx, build)
}

func (r *Reconciler) reconcile(ctx context.Context, build *buildapi.Build) error {
	if build.Finished() {
		return nil
	}

	pod, err := r.reconcileBuildPod(ctx, build)
	if err != nil {
		return err
	}

	if build.MetadataReady(pod) {
		cm, err := r.buildMetadataFromBuildPod(pod)
		if err != nil {
			return errors.Wrap(err, "failed to get build metadata from build pod")
		}
		build.Status.BuildMetadata = cm.BuildpackMetadata
		build.Status.LatestImage = cm.LatestImage
		build.Status.Stack.RunImage = cm.StackRunImage
		build.Status.Stack.ID = cm.StackID
	}

	build.Status.PodName = pod.Name
	build.Status.StepStates = stepStates(pod)
	build.Status.StepsCompleted = stepCompleted(pod)
	build.Status.Conditions = conditionForPod(pod)
	return nil
}

func (r *Reconciler) reconcileBuildPod(ctx context.Context, build *buildapi.Build) (*corev1.Pod, error) {
	pod, err := r.PodLister.Pods(build.Namespace).Get(build.PodName())
	if err != nil && !k8s_errors.IsNotFound(err) {
		return nil, err
	} else if !k8s_errors.IsNotFound(err) {
		return pod, nil
	}

	podConfig, err := r.PodGenerator.Generate(ctx, build)
	if err != nil {
		return nil, controller.NewPermanentError(err)
	}
	return r.K8sClient.CoreV1().Pods(build.Namespace).Create(ctx, podConfig, metav1.CreateOptions{})
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

func (r *Reconciler) updateStatus(ctx context.Context, desired *buildapi.Build) error {
	desired.Status.ObservedGeneration = desired.Generation
	original, err := r.Lister.Builds(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = r.Client.KpackV1alpha2().Builds(desired.Namespace).UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) buildMetadataFromBuildPod(pod *corev1.Pod) (*BuildStatusMetadata, error) {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == "completion" {
			return r.metadataDecompressor.Decompress(status.State.Terminated.Message)
		}
	}
	return nil, errors.New("completion container not found")
}
