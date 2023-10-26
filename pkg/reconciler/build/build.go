package build

import (
	"context"
	"encoding/json"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	corev1Informers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	v1Listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging/logkey"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	ReconcilerName  = "Builds"
	Kind            = "Build"
	k8sOSLabel      = "kubernetes.io/os"
	ReasonCompleted = "Completed"
)

//go:generate counterfeiter . MetadataRetriever
type MetadataRetriever interface {
	GetBuildMetadata(string, string, authn.Keychain) (*cnb.BuildMetadata, error)
}

type PodGenerator interface {
	Generate(context.Context, buildpod.BuildPodable) (*corev1.Pod, error)
}

type PodProgressLogger interface {
	GetTerminationMessage(pod *corev1.Pod, s *corev1.ContainerStatus) (string, error)
}

func NewController(ctx context.Context, opt reconciler.Options, k8sClient k8sclient.Interface, informer buildinformers.BuildInformer, podInformer corev1Informers.PodInformer, metadataRetriever MetadataRetriever, podGenerator PodGenerator, podProgressLogger *buildchange.ProgressLogger, keychainFactory registry.KeychainFactory, injectedSidecarSupport bool) *controller.Impl {
	c := &Reconciler{
		Client:                 opt.Client,
		K8sClient:              k8sClient,
		MetadataRetriever:      metadataRetriever,
		Lister:                 informer.Lister(),
		PodLister:              podInformer.Lister(),
		PodGenerator:           podGenerator,
		PodProgressLogger:      podProgressLogger,
		KeychainFactory:        keychainFactory,
		InjectedSidecarSupport: injectedSidecarSupport,
	}

	logger := opt.Logger.With(
		zap.String(logkey.Kind, buildapi.BuildCRName),
	)

	impl := controller.NewContext(ctx, c, controller.ControllerOptions{WorkQueueName: ReconcilerName, Logger: logger})

	informer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	podInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(buildapi.SchemeGroupVersion.WithKind(Kind).GroupKind()),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}

type Reconciler struct {
	Client                 versioned.Interface
	KeychainFactory        registry.KeychainFactory
	Lister                 buildlisters.BuildLister
	MetadataRetriever      MetadataRetriever
	K8sClient              k8sclient.Interface
	PodLister              v1Listers.PodLister
	PodGenerator           PodGenerator
	PodProgressLogger      PodProgressLogger
	InjectedSidecarSupport bool
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

func (c *Reconciler) reconcile(ctx context.Context, build *buildapi.Build) error {
	if build.Finished() {
		return nil
	}

	pod, err := c.reconcileBuildPod(ctx, build)
	if err != nil && !k8s_errors.IsInvalid(err) {
		return err
	} else if k8s_errors.IsInvalid(err) {
		return controller.NewPermanentError(err)
	}

	if c.InjectedSidecarSupport {
		pod, err = c.setBuildReady(ctx, pod)
		if err != nil {
			return err
		}

		pod, err = c.cleanupIfNeeded(ctx, pod)
		if err != nil {
			return err
		}
	}

	if build.MetadataReady(pod) {
		var buildMetadata *cnb.BuildMetadata
		if pod.Spec.NodeSelector != nil && pod.Spec.NodeSelector[k8sOSLabel] == "windows" {
			cacheTag := ""
			if build.Spec.NeedRegistryCache() {
				cacheTag = build.Spec.Cache.Registry.Tag
			}

			keychain, err := c.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
				ServiceAccount: build.Spec.ServiceAccountName,
				Namespace:      build.Namespace,
			})

			if err != nil {
				return errors.Wrap(err, "unable to create app image keychain")
			}

			buildMetadata, err = c.MetadataRetriever.GetBuildMetadata(build.Tag(), cacheTag, keychain)
			if err != nil {
				return err
			}
		} else {
			buildMetadata, err = c.buildMetadataFromBuildPod(pod)
			if err != nil {
				return errors.Wrap(err, "failed to get build metadata from build pod")
			}
		}
		build.Status.BuildMetadataBuildpacks = buildMetadata.BuildpackMetadata
		build.Status.BuildMetadataExtensions = buildMetadata.ExtensionMetadata
		build.Status.LatestImage = buildMetadata.LatestImage
		build.Status.LatestCacheImage = buildMetadata.LatestCacheImage
		build.Status.Stack.RunImage = buildMetadata.StackRunImage
		build.Status.Stack.ID = buildMetadata.StackID
	}

	build.Status.PodName = pod.Name
	build.Status.StepStates = stepStates(pod)
	build.Status.StepsCompleted = stepsCompleted(pod)
	build.Status.Conditions = c.conditionForPod(pod, build.Status.StepsCompleted)
	return nil
}

func (c *Reconciler) setBuildReady(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	if _, found := pod.Annotations[buildapi.BuildReadyAnnotation]; found {
		return pod, nil
	}

	if !allContainersReady(pod) {
		return pod, nil
	}

	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				buildapi.BuildReadyAnnotation: "true",
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return c.K8sClient.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.MergePatchType, patch, metav1.PatchOptions{})
}

func (c *Reconciler) cleanupIfNeeded(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	// check if pod is running && completion is terminated
	if pod.Status.Phase == corev1.PodRunning && completionContainerExited(pod) {
		patch, err := json.Marshal(map[string]interface{}{
			"spec": map[string]interface{}{
				"activeDeadlineSeconds": 1,
			},
		})
		if err != nil {
			return nil, err
		}

		return c.K8sClient.CoreV1().Pods(pod.Namespace).Patch(ctx, pod.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	}
	return pod, nil
}

func (c *Reconciler) reconcileBuildPod(ctx context.Context, build *buildapi.Build) (*corev1.Pod, error) {
	pod, err := c.PodLister.Pods(build.Namespace).Get(build.PodName())
	if err != nil && !k8s_errors.IsNotFound(err) {
		return nil, err
	}

	if k8s_errors.IsNotFound(err) {
		podConfig, err := c.PodGenerator.Generate(ctx, build)
		if err != nil {
			return nil, controller.NewPermanentError(err)
		}
		return c.K8sClient.CoreV1().Pods(build.Namespace).Create(ctx, podConfig, metav1.CreateOptions{})
	}

	return pod, nil
}

func (c *Reconciler) conditionForPod(pod *corev1.Pod, stepsCompleted []string) corev1alpha1.Conditions {
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionSucceeded,
				Status:             corev1.ConditionTrue,
				Reason:             ReasonCompleted,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			},
		}
	case corev1.PodFailed:
		if pod.Status.Reason == "DeadlineExceeded" && contains(stepsCompleted, "completion") {
			return corev1alpha1.Conditions{
				{
					Type:               corev1alpha1.ConditionSucceeded,
					Status:             corev1.ConditionTrue,
					Reason:             ReasonCompleted,
					LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
				},
			}
		}
		for _, s := range pod.Status.InitContainerStatuses {
			if s.State.Terminated != nil && s.State.Terminated.ExitCode != 0 && s.State.Terminated.Message != "" {
				terminationMessage, _ := c.PodProgressLogger.GetTerminationMessage(pod, &s)
				return corev1alpha1.Conditions{
					{
						Type:               corev1alpha1.ConditionSucceeded,
						Status:             corev1.ConditionFalse,
						Reason:             string(corev1.PodFailed),
						Message:            "Error: " + pod.Status.Message + terminationMessage,
						LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
					},
				}
			}
		}
		return corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionSucceeded,
				Status:             corev1.ConditionFalse,
				Reason:             string(corev1.PodFailed),
				Message:            pod.Status.Message,
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
	states := make([]corev1.ContainerState, 0, len(buildapi.BuildSteps()))
	for _, s := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if buildapi.IsBuildStep(s.Name) {
			state := createContainerStateForBuild(&s)
			states = append(states, state)
		}
	}
	return states
}

func stepsCompleted(pod *corev1.Pod) []string {
	completed := make([]string, 0, len(buildapi.BuildSteps()))
	for _, s := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if buildStepCompleted(&s) {
			completed = append(completed, s.Name)
		}
	}
	return completed
}

func buildStepCompleted(s *corev1.ContainerStatus) bool {
	return s.State.Terminated != nil && s.State.Terminated.ExitCode == 0 && buildapi.IsBuildStep(s.Name)
}

func (c *Reconciler) updateStatus(ctx context.Context, desired *buildapi.Build) error {
	desired.Status.ObservedGeneration = desired.Generation
	original, err := c.Lister.Builds(desired.Namespace).Get(desired.Name)
	if err != nil {
		return err
	}

	if equality.Semantic.DeepEqual(original.Status, desired.Status) {
		return nil
	}

	_, err = c.Client.KpackV1alpha2().Builds(desired.Namespace).UpdateStatus(ctx, desired, metav1.UpdateOptions{})
	return err
}

func (c *Reconciler) buildMetadataFromBuildPod(pod *corev1.Pod) (*cnb.BuildMetadata, error) {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == buildapi.CompletionContainerName {
			return cnb.DecompressBuildMetadata(status.State.Terminated.Message)
		}
	}
	return nil, errors.New(buildapi.CompletionContainerName + " container not found")
}

func contains(arr []string, s string) bool {
	for _, item := range arr {
		if s == item {
			return true
		}
	}

	return false
}

func completionContainerExited(pod *corev1.Pod) bool {
	for _, s := range pod.Status.ContainerStatuses {
		if s.Name == buildapi.CompletionContainerName {
			return s.State.Terminated != nil
		}
	}
	return false
}

func allContainersReady(pod *corev1.Pod) bool {
	ready := 0
	for _, container := range pod.Status.ContainerStatuses {
		if container.Ready {
			ready++
		}
	}

	return ready == len(pod.Spec.Containers)
}

func createContainerStateForBuild(s *corev1.ContainerStatus) corev1.ContainerState {
	switch {
	case s.State.Terminated != nil:
		if s.State.Terminated.Message == "" {
			successStatus := "successfully"
			if s.State.Terminated.ExitCode != 0 {
				successStatus = "with error"
			}
			s.State.Terminated.Message = "Container " + s.Name + " terminated " + successStatus
		}
	case s.State.Waiting != nil:
		if s.State.Waiting.Message == "" {
			s.State.Waiting.Message = "Container " + s.Name + " waiting"
		}
	default:
	}
	return s.State
}
