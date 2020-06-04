package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
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

func (c *BuildLogsClient) Tail(ctx context.Context, writer io.Writer, image, build, namespace string) error {
	return c.tailPods(ctx, writer, namespace, metav1.ListOptions{
		Watch:         true,
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", v1alpha1.ImageLabel, image, v1alpha1.BuildNumberLabel, build),
	}, true)
}

func (c *BuildLogsClient) TailImage(ctx context.Context, writer io.Writer, image, namespace string) error {
	return c.tailPods(ctx, writer, namespace, metav1.ListOptions{
		Watch:         true,
		LabelSelector: fmt.Sprintf("%s=%s", v1alpha1.ImageLabel, image),
	}, false)
}

func (c *BuildLogsClient) TailBuildName(ctx context.Context, writer io.Writer, namespace string, buildName string) error {
	return c.tailPods(ctx, writer, namespace, metav1.ListOptions{
		Watch:         true,
		LabelSelector: fmt.Sprintf("%s=%s", v1alpha1.BuildLabel, buildName),
	}, true)
}

func (c *BuildLogsClient) tailPods(ctx context.Context, writer io.Writer, namespace string, listOptions metav1.ListOptions, exitPodComplete bool) error {
	readyContainers := make(chan readyContainer)

	go func() {
		defer close(readyContainers)

		err := c.watchReadyContainers(ctx, readyContainers, namespace, listOptions, exitPodComplete)
		if err != nil {
			log.Fatalf("error watching ready containers %s", err)
		}
	}()

	for container := range readyContainers {
		err := c.streamLogsForContainer(ctx, writer, container)
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

	watcher, err := c.k8sClient.CoreV1().Pods(namespace).Watch(listOptions)
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

			pod := r.Object.(*corev1.Pod)

			switch r.Type {
			case watch.Added, watch.Modified:

				for _, c := range pod.Status.InitContainerStatuses {
					if c.State.Waiting == nil {
						readyContainers <- readyContainer{
							podName:       pod.Name,
							containerName: c.Name,
							namespace:     pod.Namespace,
						}
					}
				}

				for _, c := range pod.Status.ContainerStatuses {
					if c.State.Waiting == nil {
						readyContainers <- readyContainer{
							podName:       pod.Name,
							containerName: c.Name,
							namespace:     pod.Namespace,
						}
					}
				}

				if finished(pod) && exitPodComplete {
					return nil
				}
			}
		}
	}
}

func finished(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded
}

func (c *BuildLogsClient) streamLogsForContainer(ctx context.Context, writer io.Writer, readyContainer readyContainer) error {
	if _, alreadyProcessed := c.processed[readyContainer]; alreadyProcessed {
		return nil
	}
	c.processed[readyContainer] = nil

	logReadCloser, err := c.k8sClient.CoreV1().Pods(readyContainer.namespace).GetLogs(readyContainer.podName, &corev1.PodLogOptions{
		Container: readyContainer.containerName,
		Follow:    true}).Stream()
	if err != nil {
		return err
	}

	defer logReadCloser.Close()
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

			_, err = writer.Write([]byte(fmt.Sprintf("[%s] %s", cyan(readyContainer.containerName), line)))
			if err != nil {
				return err
			}
		}
	}
}

func cyan(s string) string {
	return fmt.Sprintf("%s%s%s", "\033[0;36m", s, "\033[0m")
}
