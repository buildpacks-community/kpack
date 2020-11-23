package logs

import (
	"context"
	"fmt"
	"io"

	"github.com/pkg/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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

func (w *imageWaiter) Wait(ctx context.Context, writer io.Writer, originalImage *v1alpha1.Image) (string, error) {
	if done, err := imageUpdateHasResolved(originalImage.Generation)(watch.Event{Object: originalImage}); err != nil {
		return "", err
	} else if done {
		return w.resultOfImageWait(ctx, writer, originalImage.Generation, originalImage)
	}

	event, err := watchTools.Until(ctx,
		originalImage.ResourceVersion,
		watchOnlyOneImage{kpackClient: w.KpackClient, image: originalImage},
		filterErrors(imageUpdateHasResolved(originalImage.Generation)))
	if err != nil {
		return "", err
	}

	image, ok := event.Object.(*v1alpha1.Image)
	if !ok {
		return "", errors.New("unexpected object received")
	}

	return w.resultOfImageWait(ctx, writer, originalImage.Generation, image)
}

func (w *imageWaiter) resultOfImageWait(ctx context.Context, writer io.Writer, generation int64, image *v1alpha1.Image) (string, error) {
	if image.Status.LatestBuildImageGeneration == generation {
		return w.waitBuild(ctx, writer, image.Namespace, image.Status.LatestBuildRef)
	}

	if condition := image.Status.GetCondition(corev1alpha1.ConditionReady); condition.IsFalse() {
		if condition.Message != "" {
			return "", errors.Errorf("update to image %s failed: %s", image.Name, condition.Message)
		}

		return "", errors.Errorf("update to image %s failed", image.Name)
	}

	return image.Status.LatestImage, nil
}

func imageUpdateHasResolved(generation int64) func(event watch.Event) (bool, error) {
	return func(event watch.Event) (bool, error) {
		image, ok := event.Object.(*v1alpha1.Image)
		if !ok {
			return false, errors.New("unexpected object received")
		}

		//space shuttle style
		if image.Status.ObservedGeneration == generation {
			if !image.Status.GetCondition(corev1alpha1.ConditionReady).IsUnknown() {
				return true, nil // Ready=False or Ready=True
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

func filterErrors(condition watchTools.ConditionFunc) watchTools.ConditionFunc {
	return func(event watch.Event) (bool, error) {
		if event.Type == watch.Error {
			return false, errors.Errorf("error on watch %+v", event.Object)
		}

		return condition(event)
	}
}

type watchOnlyOneImage struct {
	kpackClient versioned.Interface
	image       *v1alpha1.Image
}

func (w watchOnlyOneImage) Watch(options v1.ListOptions) (watch.Interface, error) {
	options.FieldSelector = fmt.Sprintf("metadata.name=%s", w.image.Name)
	return w.kpackClient.KpackV1alpha1().Images(w.image.Namespace).Watch(options)
}

func (w *imageWaiter) waitBuild(ctx context.Context, writer io.Writer, namespace, buildName string) (string, error) {
	doneChan := make(chan struct{})
	defer func() { <-doneChan }()

	go func() {
		defer close(doneChan)
		err := w.logTailer.TailBuildName(ctx, writer, namespace, buildName)
		if err != nil {
			fmt.Fprintf(writer, "error tailing logs %s", err)
		}
	}()

	event, err := watchTools.ListWatchUntil(ctx,
		&listAndWatchBuild{kpackClient: w.KpackClient, namespace: namespace, buildName: buildName},
		filterErrors(func(event watch.Event) (bool, error) {
			build, ok := event.Object.(*v1alpha1.Build)
			if !ok {
				return false, errors.New("unexpected object received")
			}

			return !build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsUnknown(), nil
		}))
	if err != nil {
		return "", err
	}

	build, ok := event.Object.(*v1alpha1.Build)
	if !ok {
		return "", errors.New("unexpected object received")
	}

	if condition := build.Status.GetCondition(corev1alpha1.ConditionSucceeded); condition.IsFalse() {
		if condition.Message != "" {
			return "", errors.Errorf("update to image failed: %s", condition.Message)
		}

		return "", errors.New("update to image failed")
	}

	return build.Status.LatestImage, nil
}
