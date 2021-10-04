package logs

import (
	"bytes"
	"context"
	"io"
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
	spec.Run(t, "Wait for image", testWaitForImage)
}

func testWaitForImage(t *testing.T, when spec.G, it spec.S) {
	var (
		testFakeLogTailer *fakeLogTailer
		out               *bytes.Buffer

		imageWatcher *TestImageWatcher
		buildWatcher *TestBuildWatcher

		generation int64 = 1
		namespace        = "some-namespace"

		successfulBuild = &v1alpha1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-to-follow",
				Namespace: namespace,
			},
			Status: v1alpha1.BuildStatus{
				Status:      conditionSuccess(corev1.ConditionTrue, ""),
				LatestImage: "already/built@sha256:1213",
			},
		}

		failedBuild = &v1alpha1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "build-to-follow",
				Namespace: namespace,
			},
			Status: v1alpha1.BuildStatus{
				Status: conditionSuccess(corev1.ConditionFalse, "some-build-error"),
			},
		}

		image       *v1alpha1.Image
		clientset   *fake.Clientset
		imageWaiter *imageWaiter
	)

	it.Before(func() {
		testFakeLogTailer = &fakeLogTailer{}
		out = &bytes.Buffer{}
		imageWatcher = &TestImageWatcher{
			events: make(chan watch.Event, 100),
		}
		buildWatcher = &TestBuildWatcher{
			events: make(chan watch.Event, 100),
		}

		image = &v1alpha1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "some-name",
				Namespace:       namespace,
				Generation:      generation,
				ResourceVersion: "100",
			},
		}

		clientset = fake.NewSimpleClientset()
		clientset.PrependWatchReactor("images", imageWatcher.watchReactor)
		clientset.PrependWatchReactor("builds", buildWatcher.watchReactor)
		imageWaiter = NewImageWaiter(clientset, testFakeLogTailer)
	})

	when("the image is already in a successful state", func() {
		it("returns the already built image and tails the finished logs", func() {
			image.Status = v1alpha1.ImageStatus{
				LatestImage:                successfulBuild.Status.LatestImage,
				LatestBuildRef:             successfulBuild.Name,
				LatestBuildImageGeneration: generation,
				Status:                     conditionReady(corev1.ConditionTrue, generation, ""),
			}
			_, err := clientset.KpackV1alpha1().Images(namespace).Create(context.TODO(), image, metav1.CreateOptions{})
			require.NoError(t, err)

			_, err = clientset.KpackV1alpha1().Builds(namespace).Create(context.TODO(), successfulBuild, metav1.CreateOptions{})
			require.NoError(t, err)

			result, err := imageWaiter.Wait(context.TODO(), out, image)
			assert.NoError(t, err)
			assert.Equal(t, successfulBuild.Status.LatestImage, result)
			assert.Equal(t, namespace, testFakeLogTailer.args[1])
			assert.Equal(t, successfulBuild.Name, testFakeLogTailer.args[2])
		})
	})

	when("the image is already in a failed state", func() {
		it("returns an error", func() {
			image.Status = v1alpha1.ImageStatus{
				Status: conditionReady(corev1.ConditionFalse, generation, "some-image-error"),
			}

			_, err := clientset.KpackV1alpha1().Images(namespace).Create(context.TODO(), image, metav1.CreateOptions{})
			require.NoError(t, err)
			_, err = imageWaiter.Wait(context.TODO(), out, image)
			assert.EqualError(t, err, "update to image some-name failed: some-image-error")
		})
	})

	when("the image is building", func() {
		it.Before(func() {
			image.Status = v1alpha1.ImageStatus{
				LatestBuildRef:             "build-to-follow",
				LatestBuildImageGeneration: generation,
				Status:                     conditionReady(corev1.ConditionUnknown, generation, ""),
			}

			_, err := clientset.KpackV1alpha1().Images(namespace).Create(context.TODO(), image, metav1.CreateOptions{})
			require.NoError(t, err)

			_, err = clientset.KpackV1alpha1().Builds(namespace).Create(
				context.TODO(),
				&v1alpha1.Build{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "build-to-follow",
						Namespace: namespace,
					},
					Status: v1alpha1.BuildStatus{
						Status: conditionSuccess(corev1.ConditionUnknown, ""),
					},
				},
				metav1.CreateOptions{},
			)
			require.NoError(t, err)
		})

		it("tails the build logs and returns the built image on success", func() {
			buildWatcher.addEvent(watch.Event{Type: watch.Modified, Object: successfulBuild})

			result, err := imageWaiter.Wait(context.TODO(), out, image)
			assert.NoError(t, err)
			assert.Equal(t, successfulBuild.Status.LatestImage, result)
			assert.Equal(t, namespace, testFakeLogTailer.args[1])
			assert.Equal(t, successfulBuild.Name, testFakeLogTailer.args[2])
		})

		it("tails the build logs and returns an error on failure", func() {
			buildWatcher.addEvent(watch.Event{Type: watch.Modified, Object: failedBuild})

			_, err := imageWaiter.Wait(context.TODO(), out, image)
			assert.EqualError(t, err, "build failed: some-build-error")
			assert.Equal(t, namespace, testFakeLogTailer.args[1])
			assert.Equal(t, failedBuild.Name, testFakeLogTailer.args[2])
		})
	})

	when("the build has not been scheduled yet", func() {
		it("waits until resulting build is scheduled", func() {
			image.Generation = generation + 1
			image.Status = v1alpha1.ImageStatus{
				LatestImage:                "old-image",
				LatestBuildRef:             "build-to-follow",
				LatestBuildImageGeneration: generation,
				Status:                     conditionReady(corev1.ConditionUnknown, generation+1, ""),
			}

			_, err := clientset.KpackV1alpha1().Images(namespace).Create(context.TODO(), image, metav1.CreateOptions{})
			require.NoError(t, err)

			imageWatcher.addEvent(watch.Event{
				Type: watch.Modified,
				Object: &v1alpha1.Image{
					ObjectMeta: image.ObjectMeta,
					Status: v1alpha1.ImageStatus{
						LatestBuildRef:             "build-to-follow",
						LatestBuildImageGeneration: generation + 1,
						Status:                     conditionReady(corev1.ConditionUnknown, generation+1, ""),
					},
				},
			})

			_, err = clientset.KpackV1alpha1().Builds(namespace).Create(context.TODO(), successfulBuild, metav1.CreateOptions{})
			require.NoError(t, err)

			result, err := imageWaiter.Wait(context.TODO(), out, image)
			assert.NoError(t, err)
			assert.Equal(t, successfulBuild.Status.LatestImage, result)

			assert.Equal(t, namespace, testFakeLogTailer.args[1])
			assert.Equal(t, successfulBuild.Name, testFakeLogTailer.args[2])
		})
	})

	when("an image update is skipped", func() {
		it("an error is returned", func() {
			image.Status = v1alpha1.ImageStatus{
				LatestImage:                "old-image",
				LatestBuildRef:             "build-to-follow",
				LatestBuildImageGeneration: generation - 1,
				Status:                     conditionReady(corev1.ConditionUnknown, generation, ""),
			}

			_, err := clientset.KpackV1alpha1().Images(namespace).Create(context.TODO(), image, metav1.CreateOptions{})
			require.NoError(t, err)

			imageWatcher.addEvent(watch.Event{
				Type: watch.Modified,
				Object: &v1alpha1.Image{
					ObjectMeta: image.ObjectMeta,
					Status: v1alpha1.ImageStatus{
						LatestBuildRef:             "build-to-follow",
						LatestBuildImageGeneration: image.Generation + 1,
						Status:                     conditionReady(corev1.ConditionUnknown, image.Generation+1, ""),
					},
				},
			})
			_, err = imageWaiter.Wait(context.TODO(), out, image)
			assert.EqualError(t, err, "image some-name was updated before original update was processed")
		})
	})
}

func conditionReady(status corev1.ConditionStatus, generation int64, message string) corev1alpha1.Status {
	return corev1alpha1.Status{
		ObservedGeneration: generation,
		Conditions: []corev1alpha1.Condition{
			{
				Type:    corev1alpha1.ConditionReady,
				Status:  status,
				Message: message,
			},
		},
	}
}

func conditionSuccess(status corev1.ConditionStatus, message string) corev1alpha1.Status {
	return corev1alpha1.Status{
		Conditions: []corev1alpha1.Condition{
			{
				Type:    corev1alpha1.ConditionSucceeded,
				Status:  status,
				Message: message,
			},
		},
	}
}

type TestImageWatcher struct {
	events chan watch.Event
}

func (t *TestImageWatcher) addEvent(event watch.Event) {
	t.events <- event
}

func (t *TestImageWatcher) Stop() {
}

func (t *TestImageWatcher) ResultChan() <-chan watch.Event {
	return t.events
}

func (t *TestImageWatcher) watchReactor(action clientgotesting.Action) (handled bool, ret watch.Interface, err error) {
	return true, t, nil
}

type TestBuildWatcher struct {
	events chan watch.Event
}

func (t *TestBuildWatcher) addEvent(event watch.Event) {
	t.events <- event
}

func (t *TestBuildWatcher) Stop() {
}

func (t *TestBuildWatcher) ResultChan() <-chan watch.Event {
	return t.events
}

func (t *TestBuildWatcher) watchReactor(action clientgotesting.Action) (handled bool, ret watch.Interface, err error) {
	return true, t, nil
}

type fakeLogTailer struct {
	args []interface{}
}

func (f *fakeLogTailer) TailBuildName(ctx context.Context, writer io.Writer, buildName, namespace string) error {
	f.args = []interface{}{writer, buildName, namespace}
	return nil
}
