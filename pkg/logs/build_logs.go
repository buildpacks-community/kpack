package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8sclient "k8s.io/client-go/kubernetes"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type BuildLogsClient struct {
	k8sClient k8sclient.Interface
	processed map[readyContainer]interface{}
}

func NewBuildLogsClient(k8sClient k8sclient.Interface) *BuildLogsClient {
	return &BuildLogsClient{
		k8sClient: k8sClient,
		processed: make(map[readyContainer]interface{}),
	}
}

func (c *BuildLogsClient) Tail(ctx context.Context, writer io.Writer, image, build, namespace string, timestamp bool) error {
	return c.tailPods(ctx, writer, namespace, metav1.ListOptions{
		Watch:         true,
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", buildapi.ImageLabel, image, buildapi.BuildNumberLabel, build),
	}, true, true, timestamp)
}

func (c *BuildLogsClient) TailImage(ctx context.Context, writer io.Writer, image, namespace string) error {
	return c.tailPods(ctx, writer, namespace, metav1.ListOptions{
		Watch:         true,
		LabelSelector: fmt.Sprintf("%s=%s", buildapi.ImageLabel, image),
	}, false, true, false)
}

func (c *BuildLogsClient) GetImageLogs(ctx context.Context, writer io.Writer, image, namespace string) error {
	return c.getPodLogs(ctx, writer, namespace, metav1.ListOptions{
		Watch:         false,
		LabelSelector: fmt.Sprintf("%s=%s", buildapi.ImageLabel, image),
	}, false, false)
}

func (c *BuildLogsClient) TailBuildName(ctx context.Context, writer io.Writer, namespace string, buildName string, timestamp bool) error {
	return c.tailPods(ctx, writer, namespace, metav1.ListOptions{
		Watch:         true,
		LabelSelector: fmt.Sprintf("%s=%s", buildapi.BuildLabel, buildName),
	}, true, true, timestamp)
}

func (c *BuildLogsClient) tailPods(ctx context.Context, writer io.Writer, namespace string, listOptions metav1.ListOptions, exitPodComplete bool, follow, timestamp bool) error {
	readyContainers := make(chan readyContainer)

	go func() {
		defer close(readyContainers)

		err := c.watchReadyContainers(ctx, readyContainers, namespace, listOptions, exitPodComplete)
		if err != nil {
			log.Fatalf("error watching ready containers %s", err)
		}
	}()

	for container := range readyContainers {
		err := c.streamLogsForContainer(ctx, writer, container, follow, timestamp)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *BuildLogsClient) getPodLogs(ctx context.Context, writer io.Writer, namespace string, listOptions metav1.ListOptions, follow, timestamp bool) error {
	readyContainers, err := c.getContainers(ctx, namespace, listOptions)

	if err != nil {
		return err
	}

	for _, container := range readyContainers {
		err := c.streamLogsForContainer(ctx, writer, container, follow, timestamp)
		if err != nil {
			return err
		}
	}

	return nil
}

type readyContainer struct {
	podName       string
	containerName string
	namespace     string
}

func (c *BuildLogsClient) watchReadyContainers(ctx context.Context, readyContainers chan<- readyContainer, namespace string, listOptions metav1.ListOptions, exitPodComplete bool) error {

	watcher, err := c.k8sClient.CoreV1().Pods(namespace).Watch(ctx, listOptions)
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case r, ok := <-watcher.ResultChan():
			if !ok {
				return nil
			}

			switch r.Type {
			case watch.Added, watch.Modified:
				pod := r.Object.(*corev1.Pod)
				containers := map[string]readyContainer{}
				for _, c := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
					if c.State.Waiting == nil {
						containers[c.Name] = readyContainer{
							podName:       pod.Name,
							containerName: c.Name,
							namespace:     pod.Namespace,
						}
					}
				}

				for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
					if !buildapi.IsBuildStep(container.Name) {
						continue
					}
					if readyContainer, found := containers[container.Name]; found {
						readyContainers <- readyContainer
					} else {
						break
					}
				}

				if finished(pod) && exitPodComplete {
					return nil
				}
			}
		}
	}
}

func (c *BuildLogsClient) getContainers(ctx context.Context, namespace string, listOptions metav1.ListOptions) ([]readyContainer, error) {

	readyContainers := make([]readyContainer, 0)
	pods, err := c.k8sClient.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		containers := map[string]readyContainer{}
		for _, c := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
			if c.State.Waiting == nil {
				containers[c.Name] = readyContainer{
					podName:       pod.Name,
					containerName: c.Name,
					namespace:     pod.Namespace,
				}
			}
		}

		for _, container := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
			if !buildapi.IsBuildStep(container.Name) {
				continue
			}

			if readyContainer, found := containers[container.Name]; found {
				readyContainers = append(readyContainers, readyContainer)
			}
		}
	}
	return readyContainers, nil
}

func finished(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded
}

func (c *BuildLogsClient) streamLogsForContainer(ctx context.Context, writer io.Writer, readyContainer readyContainer, follow, timestamp bool) error {
	if _, alreadyProcessed := c.processed[readyContainer]; alreadyProcessed {
		return nil
	}
	c.processed[readyContainer] = nil

	logReadCloser, err := c.k8sClient.CoreV1().Pods(readyContainer.namespace).GetLogs(readyContainer.podName, &corev1.PodLogOptions{
		Container:  readyContainer.containerName,
		Follow:     follow,
		Timestamps: timestamp}).Stream(ctx)
	if err != nil {
		return err
	}
	defer logReadCloser.Close()

	_, err = writer.Write([]byte(cyan(fmt.Sprintf("===> %s\n", strings.ToUpper(readyContainer.containerName)))))
	if err != nil {
		return err
	}

	r := bufio.NewReader(logReadCloser)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := r.ReadBytes('\n')
			if err != nil && err != io.EOF {
				return nil
			}

			if err == io.EOF {
				return nil
			}

			_, err = writer.Write(line)
			if err != nil {
				return err
			}
		}
	}
}

func cyan(s string) string {
	return fmt.Sprintf("%s%s%s", "\033[0;36m", s, "\033[0m")
}
