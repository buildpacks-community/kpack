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

	"github.com/pivotal/build-service-beam/pkg/apis/build/v1alpha1"
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

func (c *BuildLogsClient) Tail(context context.Context, writer io.Writer, image, build, namespace string) error {
	readyContainers := make(chan readyContainer)

	go func() {
		err := c.watchReadyContainers(context, readyContainers, image, build, namespace)
		if err != nil {
			log.Fatalf("error watching ready containers %s", err)
		}
	}()

	for container := range readyContainers {
		err := c.streamLogsForContainer(context, writer, container)
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

func (c *BuildLogsClient) watchReadyContainers(ctx context.Context, readyContainers chan<- readyContainer, image, build, namespace string) error {
	watcher, err := c.k8sClient.CoreV1().Pods(namespace).Watch(metav1.ListOptions{
		Watch:         true,
		LabelSelector: labelSelector(image, build),
	})
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
			}
		}
	}
}

func labelSelector(image string, build string) string {
	if build == "" {
		return fmt.Sprintf("%s=%s", v1alpha1.ImageLabel, image)
	}

	return fmt.Sprintf("%s=%s,%s=%s", v1alpha1.ImageLabel, image, v1alpha1.BuildNumberLabel, build)
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

			_, err = writer.Write(line)
			if err != nil {
				return err
			}
		}
	}
}
