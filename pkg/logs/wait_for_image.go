package logs

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchTools "k8s.io/client-go/tools/watch"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

type imageWaiter struct {
	KpackClient versioned.Interface
	logTailer   ImageLogTailer
}

type ImageLogTailer interface {
	TailBuildName(ctx context.Context, writer io.Writer, buildName, namespace string) error
}

func NewImageWaiter(kpackClient versioned.Interface, logTailer ImageLogTailer) *imageWaiter {
	return &imageWaiter{KpackClient: kpackClient, logTailer: logTailer}
}

func (w *imageWaiter) Wait(ctx context.Context, writer io.Writer, image *v1alpha1.Image) (string, error) {
	if done, err := imageUpdateHasResolved(image.Generation)(watch.Event{Object: image}); err != nil {
		return "", err
	} else if done {
		return w.resultOfImageWait(ctx, writer, image.Generation, image)
	}

	event, err := watchTools.Until(ctx,
		image.ResourceVersion,
		watchOneImage{kpackClient: w.KpackClient, image: image, ctx: ctx},
		filterErrors(imageUpdateHasResolved(image.Generation)))
	if err != nil {
		return "", err
	}

	image, ok := event.Object.(*v1alpha1.Image)
	if !ok {
		return "", errors.New("unexpected object received")
	}

	return w.resultOfImageWait(ctx, writer, image.Generation, image)
}

func imageUpdateHasResolved(generation int64) func(event watch.Event) (bool, error) {
	return func(event watch.Event) (bool, error) {
		image, ok := event.Object.(*v1alpha1.Image)
		if !ok {
			return false, errors.New("unexpected object received")
		}

		if image.Status.ObservedGeneration == generation { // image is reconciled
			if !image.Status.GetCondition(corev1alpha1.ConditionReady).IsUnknown() {
				return true, nil // image is resolved
			} else if image.Status.LatestBuildImageGeneration == generation {
				return true, nil // Build scheduled
			} else {
				return false, nil // still waiting on build to be scheduled
			}
		} else if image.Status.ObservedGeneration > generation {
			return false, errors.Errorf("image %s was updated before original update was processed", image.Name) // update skipped
		} else {
			return false, nil // still waiting on update
		}
	}
}

func (w *imageWaiter) resultOfImageWait(ctx context.Context, writer io.Writer, generation int64, image *v1alpha1.Image) (string, error) {
	if image.Status.LatestBuildImageGeneration == generation {
		return w.waitBuild(ctx, writer, image.Namespace, image.Status.LatestBuildRef)
	}

	if condition := image.Status.GetCondition(corev1alpha1.ConditionReady); condition.IsFalse() {
		return "", imageFailure(image.Name, condition.Message)
	}

	return image.Status.LatestImage, nil
}

func imageFailure(name, statusMessage string) error {
	errMsg := fmt.Sprintf("update to image %s failed", name)

	if statusMessage != "" {
		errMsg = fmt.Sprintf("%s: %s", errMsg, statusMessage)
	}
	return errors.New(errMsg)
}

func (w *imageWaiter) waitBuild(ctx context.Context, writer io.Writer, namespace, buildName string) (string, error) {
	doneChan := make(chan struct{})
	defer func() { <-doneChan }()

	go func() { // tail logs
		defer close(doneChan)
		err := w.logTailer.TailBuildName(ctx, writer, namespace, buildName)
		if err != nil {
			fmt.Fprintf(writer, "error tailing logs %s", err)
		}
	}()

	build, err := w.buildWatchUntil(ctx, namespace, buildName, filterErrors(buildHasResolved))
	if err != nil {
		return "", err
	}

	if condition := build.Status.GetCondition(corev1alpha1.ConditionSucceeded); condition.IsFalse() {
		return "", buildFailure(condition.Message)
	}

	return build.Status.LatestImage, nil
}

func buildHasResolved(event watch.Event) (bool, error) {
	build, ok := event.Object.(*v1alpha1.Build)
	if !ok {
		return false, errors.New("unexpected object received, expected Build")
	}

	return !build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsUnknown(), nil
}

func buildFailure(statusMessage string) error {
	errMsg := "build failed"

	if statusMessage != "" {
		errMsg = fmt.Sprintf("%s: %s", errMsg, statusMessage)
	}
	return errors.New(errMsg)
}

func (w *imageWaiter) buildWatchUntil(ctx context.Context, namespace, buildName string, condition watchTools.ConditionFunc) (*v1alpha1.Build, error) {
	build, err := w.KpackClient.KpackV1alpha1().Builds(namespace).Get(ctx, buildName, v1.GetOptions{})
	if err != nil {
		return nil, err
	}
	event, err := watchTools.UntilWithSync(ctx,
		&watchOneBuild{context: ctx, kpackClient: w.KpackClient, namespace: namespace, buildName: buildName},
		&v1alpha1.Build{},
		func(store cache.Store) (bool, error) {
			return condition(watch.Event{Object: build})
		},
		condition,
	)
	if err != nil {
		return nil, err
	}
	if event != nil { // event is nil if precondition is true
		var ok bool
		build, ok = event.Object.(*v1alpha1.Build)
		if !ok {
			return nil, errors.New("unexpected object received, expected Build")
		}
	}
	return build, nil
}

func filterErrors(condition watchTools.ConditionFunc) watchTools.ConditionFunc {
	return func(event watch.Event) (bool, error) {
		if event.Type == watch.Error {
			return false, errors.Errorf("error on watch %+v", event.Object)
		}

		return condition(event)
	}
}
