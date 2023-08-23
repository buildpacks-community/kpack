package buildchange

import (
	"bufio"
	"context"
	"io"

	"github.com/acarl005/stripansi"

	corev1 "k8s.io/api/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
)

type ProgressLogger struct {
	K8sClient k8sclient.Interface
}

// GetTerminationMessage gets the container logs for a given pod
func (p *ProgressLogger) GetTerminationMessage(pod *corev1.Pod, s *corev1.ContainerStatus) (string, error) {
	containerLog, _ := p.getContainerLogs(pod, s)

	moreInfoCmd := createMoreInfoCommand(pod.Namespace, pod.Name, s.Name)
	return " " + s.State.Terminated.Message + " " + containerLog + moreInfoCmd, nil
}

func (p *ProgressLogger) getContainerLogs(pod *corev1.Pod, s *corev1.ContainerStatus) (string, error) {
	logs := p.K8sClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: s.Name,
	})
	containerLogReader, err := logs.Stream(context.TODO())
	if err != nil {
		return "could not retrieve container logs", err
	}
	defer containerLogReader.Close()

	r := bufio.NewReader(containerLogReader)
	containerLog := ""
	for {
		line, err := r.ReadString('\n')
		if err != nil || err == io.EOF {
			break
		}
		containerLog = stripansi.Strip(line)
	}

	return containerLog, nil
}

func createMoreInfoCommand(namespace, podName, containerName string) string {
	return ": For more info use `kubectl logs -n " + namespace + " " + podName + " -c " + containerName + "`"
}
