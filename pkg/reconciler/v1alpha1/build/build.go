package build

import (
	"context"

	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1Informers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	v1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	v1alpha1informer "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1lister "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	ReconcilerName = "Builds"
	Kind           = "Build"
)

type MetadataRetriever interface {
	GetBuiltImage(repoName registry.ImageRef) (cnb.BuiltImage, error)
}

type PodGenerator interface {
	Generate(*v1alpha1.Build) (*corev1.Pod, error)
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
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
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

	if build.Finished() {
		return nil
	}

	pod, err := c.reconcileBuildPod(build)
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
	}

	build.Status.PodName = pod.Name
	build.Status.StepStates = stepStates(pod)
	build.Status.StepsCompleted = stepCompleted(pod)
	build.Status.Conditions = conditionForPod(pod)

	build.Status.ObservedGeneration = build.Generation

	return c.updateStatus(build)
}

func (c *Reconciler) reconcileBuildPod(build *v1alpha1.Build) (*corev1.Pod, error) {
	pod, err := c.PodLister.Pods(build.Namespace()).Get(build.PodName())
	if err != nil && !k8s_errors.IsNotFound(err) {
		return nil, err
	} else if k8s_errors.IsNotFound(err) {
		podConfig, err := c.PodGenerator.Generate(build)
		if err != nil {
			return nil, err
		}
		return c.K8sClient.CoreV1().Pods(build.Namespace()).Create(podConfig)
	}

	return pod, nil
}

func conditionForPod(pod *corev1.Pod) duckv1alpha1.Conditions {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return duckv1alpha1.Conditions{
			{
				Type:               duckv1alpha1.ConditionSucceeded,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: apis.VolatileTime{Inner: metav1.Now()},
			},
		}
	case corev1.PodFailed:
		return duckv1alpha1.Conditions{
			{
				Type:               duckv1alpha1.ConditionSucceeded,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: apis.VolatileTime{Inner: metav1.Now()},
			},
		}
	default:
		return duckv1alpha1.Conditions{
			{
				Type:   duckv1alpha1.ConditionSucceeded,
				Status: corev1.ConditionUnknown,
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

func (c *Reconciler) updateStatus(desired *v1alpha1.Build) error {
	original, err := c.Lister.Builds(desired.Namespace()).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.BuildV1alpha1().Builds(desired.Namespace()).UpdateStatus(desired)
	return err
}

func buildMetadataFromBuiltImage(image cnb.BuiltImage) []v1alpha1.BuildpackMetadata {
	buildpackMetadata := make([]v1alpha1.BuildpackMetadata, 0, len(image.BuildpackMetadata))
	for _, metadata := range image.BuildpackMetadata {
		buildpackMetadata = append(buildpackMetadata, v1alpha1.BuildpackMetadata{
			ID:      metadata.ID,
			Version: metadata.Version,
		})
	}
	return buildpackMetadata
}
