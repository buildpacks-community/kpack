package logs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
)

func TestWaitForImage(t *testing.T) {
	spec.Run(t, "Wait for image", waitForImage)
}

func waitForImage(t *testing.T, when spec.G, it spec.S) {
	var (
		fakeLogTailer = &fakeLogTailer{}
		out           = &bytes.Buffer{}

		imageWatcher = &TestWatcher{
			initialResourceVersion: 1,
			events:                 make(chan watch.Event, 100),
		}

		nextBuild          = 11
		generation   int64 = 2
		imageToWatch       = &v1alpha1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "some-name",
				Namespace:       "some-namespace",
				ResourceVersion: "1",
				Generation:      generation,
			},
			Status: v1alpha1.ImageStatus{
				BuildCounter: int64(nextBuild - 1),
			},
		}
		clientset   = fake.NewSimpleClientset()
		imageWaiter = NewImageWaiter(clientset, fakeLogTailer)
	)

	it.Before(func() {
		clientset.PrependWatchReactor("images", imageWatcher.watchReactor)
	})

	when("no change is needed to an image", func() {
		it("returns the already built image", func() {
			alreadyReadyImage := &v1alpha1.Image{
				ObjectMeta: imageToWatch.ObjectMeta,
				Status: v1alpha1.ImageStatus{
					LatestImage:                "already/built@sha256:1213",
					LatestBuildImageGeneration: imageToWatch.Generation - 1,
					Status:                     conditionReady(corev1.ConditionTrue, imageToWatch.Generation),
				},
			}

			result, err := imageWaiter.Wait(context.TODO(), out, alreadyReadyImage)
			assert.NoError(t, err)
			assert.Equal(t, "already/built@sha256:1213", result)
		})

		it("returns an error if no build has been created", func() {
			alreadyReadyImage := &v1alpha1.Image{
				ObjectMeta: imageToWatch.ObjectMeta,
				Status: v1alpha1.ImageStatus{
					Status: conditionReady(corev1.ConditionFalse, imageToWatch.Generation),
				},
			}

			_, err := imageWaiter.Wait(context.TODO(), out, alreadyReadyImage)
			assert.EqualError(t, err, "update to image some-name failed")
		})
	})

	when("when a build is scheduled", func() {
		it("returns built image from the build and tails build logs", func() {
			_, err := clientset.KpackV1alpha1().Builds(imageToWatch.Namespace).Create(
				context.TODO(),
				&v1alpha1.Build{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "build-to-follow",
						Namespace:       imageToWatch.Namespace,
						ResourceVersion: "1",
					},
					Status: v1alpha1.BuildStatus{
						Status:      conditionSuccess(corev1.ConditionTrue),
						LatestImage: "image/built-bybuild@sha256:123",
					},
				},
				metav1.CreateOptions{},
			)
			require.NoError(t, err)

			scheduledBuildImage := &v1alpha1.Image{
				ObjectMeta: imageToWatch.ObjectMeta,
				Status: v1alpha1.ImageStatus{
					LatestBuildRef:             "build-to-follow",
					LatestBuildImageGeneration: imageToWatch.Generation,
					Status:                     conditionReady(corev1.ConditionUnknown, imageToWatch.Generation),
				},
			}

			result, err := imageWaiter.Wait(context.TODO(), out, scheduledBuildImage)
			assert.NoError(t, err)
			assert.Equal(t, "image/built-bybuild@sha256:123", result)

			assert.Equal(t, imageToWatch.Namespace, fakeLogTailer.args[1])
			assert.Equal(t, "build-to-follow", fakeLogTailer.args[2])
		})

		it("returns an err if resulting build fails", func() {
			build := &v1alpha1.Build{
				ObjectMeta: metav1.ObjectMeta{

					Name:            "build-to-follow",
					Namespace:       imageToWatch.Namespace,
					ResourceVersion: "1",
				},
				Status: v1alpha1.BuildStatus{
					Status: conditionSuccess(corev1.ConditionFalse),
				},
			}

			_, err := clientset.KpackV1alpha1().Builds(imageToWatch.Namespace).Create(context.TODO(), build, metav1.CreateOptions{})
			require.NoError(t, err)

			scheduledBuildImage := &v1alpha1.Image{
				ObjectMeta: imageToWatch.ObjectMeta,
				Status: v1alpha1.ImageStatus{
					LatestBuildRef:             "build-to-follow",
					LatestBuildImageGeneration: imageToWatch.Generation,
					Status:                     conditionReady(corev1.ConditionUnknown, imageToWatch.Generation),
				},
			}

			_, err = imageWaiter.Wait(context.TODO(), out, scheduledBuildImage)
			assert.Error(t, err)
			assert.EqualError(t, err, "update to image failed")

			assert.Equal(t, imageToWatch.Namespace, fakeLogTailer.args[1])
			assert.Equal(t, "build-to-follow", fakeLogTailer.args[2])
		})

		it("waits until resulting build is scheduled", func() {
			image := &v1alpha1.Image{
				ObjectMeta: imageToWatch.ObjectMeta,
			}
			imageWatcher.expectedImage = image

			imageWatcher.addEvent(watch.Event{
				Type: watch.Modified,
				Object: &v1alpha1.Image{
					ObjectMeta: image.ObjectMeta,
					Status: v1alpha1.ImageStatus{
						LatestBuildRef:             "build-to-follow",
						LatestBuildImageGeneration: image.Generation,
						Status:                     conditionReady(corev1.ConditionUnknown, image.Generation),
					},
				},
			})
			_, err := clientset.KpackV1alpha1().Builds(imageToWatch.Namespace).Create(
				context.TODO(),
				&v1alpha1.Build{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "build-to-follow",
						Namespace:       imageToWatch.Namespace,
						ResourceVersion: "1",
					},
					Status: v1alpha1.BuildStatus{
						Status:      conditionSuccess(corev1.ConditionTrue),
						LatestImage: "image/built-bybuild@sha256:123",
					},
				},
				metav1.CreateOptions{},
			)
			require.NoError(t, err)

			result, err := imageWaiter.Wait(context.TODO(), out, image)
			assert.NoError(t, err)
			assert.Equal(t, "image/built-bybuild@sha256:123", result)

			assert.Equal(t, imageToWatch.Namespace, fakeLogTailer.args[1])
			assert.Equal(t, "build-to-follow", fakeLogTailer.args[2])
		})

	})

	when("an image update is skipped", func() {
		it("an error is returned", func() {
			image := &v1alpha1.Image{
				ObjectMeta: imageToWatch.ObjectMeta,
			}
			imageWatcher.expectedImage = image

			imageWatcher.addEvent(watch.Event{
				Type: watch.Modified,
				Object: &v1alpha1.Image{
					ObjectMeta: image.ObjectMeta,
					Status: v1alpha1.ImageStatus{
						LatestBuildRef:             "build-to-follow",
						LatestBuildImageGeneration: image.Generation + 1,
						Status:                     conditionReady(corev1.ConditionUnknown, image.Generation+1),
					},
				},
			})
			_, err := imageWaiter.Wait(context.TODO(), out, image)
			assert.EqualError(t, err, "image some-name was updated before original update was processed")
		})
	})

	it("surfaces error messages", func() {
		when("there is a status message for an image error", func() {
			it("adds the status message to the returned error", func() {
				alreadyReadyImage := &v1alpha1.Image{
					ObjectMeta: imageToWatch.ObjectMeta,
					Status: v1alpha1.ImageStatus{
						Status: conditionReady(corev1.ConditionFalse, imageToWatch.Generation),
					},
				}

				alreadyReadyImage.Status.Conditions[0].Message = "some error"

				_, err := imageWaiter.Wait(context.TODO(), out, alreadyReadyImage)
				assert.EqualError(t, err, "update to image some-name failed: some error")
			})
		})
		when("there is a status message for a build error", func() {
			it("adds the status message to the returned error", func() {
				build := &v1alpha1.Build{
					ObjectMeta: metav1.ObjectMeta{
						Name:            "build-to-follow",
						Namespace:       imageToWatch.Namespace,
						ResourceVersion: "1",
					},
					Status: v1alpha1.BuildStatus{
						Status: conditionSuccess(corev1.ConditionFalse),
					},
				}
				build.Status.Conditions[0].Message = "some error"

				_, err := clientset.KpackV1alpha1().Builds(imageToWatch.Namespace).Create(context.TODO(), build, metav1.CreateOptions{})
				require.NoError(t, err)

				scheduledBuildImage := &v1alpha1.Image{
					ObjectMeta: imageToWatch.ObjectMeta,
					Status: v1alpha1.ImageStatus{
						LatestBuildRef:             "build-to-follow",
						LatestBuildImageGeneration: imageToWatch.Generation,
						Status:                     conditionReady(corev1.ConditionUnknown, imageToWatch.Generation),
					},
				}

				_, err = imageWaiter.Wait(context.TODO(), out, scheduledBuildImage)
				assert.Error(t, err)
				assert.EqualError(t, err, "update to image failed: some error")

			})
		})
	})

}

func conditionReady(status corev1.ConditionStatus, generation int64) corev1alpha1.Status {
	return corev1alpha1.Status{
		ObservedGeneration: generation,
		Conditions: []corev1alpha1.Condition{
			{
				Type:   corev1alpha1.ConditionReady,
				Status: status,
			},
		},
	}
}

func conditionSuccess(status corev1.ConditionStatus) corev1alpha1.Status {
	return corev1alpha1.Status{
		Conditions: []corev1alpha1.Condition{
			{
				Type:   corev1alpha1.ConditionSucceeded,
				Status: status,
			},
		},
	}
}

type TestWatcher struct {
	events                 chan watch.Event
	initialResourceVersion int
	expectedImage          *v1alpha1.Image
}

func (t *TestWatcher) addEvent(event watch.Event) {
	t.initialResourceVersion++

	image := event.Object.(*v1alpha1.Image)
	image.ResourceVersion = strconv.Itoa(t.initialResourceVersion)
	t.events <- event
}

func (t *TestWatcher) Stop() {
}

func (t *TestWatcher) ResultChan() <-chan watch.Event {
	return t.events
}

func (t *TestWatcher) watchReactor(action clientgotesting.Action) (handled bool, ret watch.Interface, err error) {
	namespace := action.GetNamespace()
	if t.expectedImage == nil {
		return false, nil, errors.New("test watcher must be configured with an expected image to be used")

	}

	if namespace != t.expectedImage.Namespace {
		return false, nil, errors.New("unexpected namespace watch")
	}

	watchAction := action.(clientgotesting.WatchAction)
	if watchAction.GetWatchRestrictions().ResourceVersion != t.expectedImage.ResourceVersion {
		return false, nil, errors.New("expected watch on resource version")
	}

	match, found := watchAction.GetWatchRestrictions().Fields.RequiresExactMatch("metadata.name")
	if !found {
		return false, nil, errors.New("expected watch on name")
	}
	if match != t.expectedImage.Name {
		return false, nil, errors.New("expected watch on name")
	}

	return true, t, nil
}

type fakeLogTailer struct {
	args []interface{}
}

func (f *fakeLogTailer) TailBuildName(ctx context.Context, writer io.Writer, buildName, namespace string) error {
	f.args = []interface{}{writer, buildName, namespace}
	return nil
}
