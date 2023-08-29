package buildchange

import (
	"bufio"
	"context"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
)

const (
	ansiRegex         = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	MaxLogMessageSize = 800
)

type ProgressLogger struct {
	K8sClient k8sclient.Interface
}

// GetTerminationMessage creates a termination message for a pod.
// Message consists out of container name that has terminated unsuccessfully with the
// last line of the container log truncated to 800 characters and the command to get more info from that container.
func (p *ProgressLogger) GetTerminationMessage(pod *corev1.Pod, s *corev1.ContainerStatus) (string, error) {
	containerLog, _ := p.getContainerLogs(pod, s)

	moreInfoCmd := createMoreInfoCommand(pod.Namespace, pod.Name, s.Name)
	return " " + s.State.Terminated.Message + " " + truncate(containerLog) + moreInfoCmd, nil
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

	scanner := bufio.NewScanner(containerLogReader)
	containerLog := ""

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			containerLog = stripAnsi(line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "error reading container log", err
	}

	return containerLog, nil
}

func createMoreInfoCommand(namespace, podName, containerName string) string {
	return ": For more info use `kubectl logs -n " + namespace + " " + podName + " -c " + containerName + "`"
}

func stripAnsi(str string) string {
	re := regexp.MustCompile(ansiRegex)
	return re.ReplaceAllString(str, "")
}

func truncate(text string) string {
	if len(text) < MaxLogMessageSize {
		return text
	} else {
		return text[:MaxLogMessageSize]
	}
}
