package buildchange_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/sclevine/spec"
	corev1 "k8s.io/api/core/v1"
)

func TestProgressLogger(t *testing.T) {
	spec.Run(t, "ProgressLogger", testProgressLogger)
}

func testProgressLogger(t *testing.T, when spec.G, it spec.S) {
	// Create a fake Kubernetes client
	k8sClient := k8sfake.NewSimpleClientset()
	pl := &buildchange.ProgressLogger{
		K8sClient: k8sClient,
	}

	when("Pod terminated successfully", func() {
		it("returns the correct status for the container that terminated with an error", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-name",
					Namespace: "test",
				},
			}
			containerStatus := &corev1.ContainerStatus{
				Name: "prepare",
				State: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						Message: "Container detect terminated with error",
					},
				},
			}

			podTeminationMessage, err := pl.GetTerminationMessage(pod, containerStatus)
			assert.NoError(t, err)
			assert.Equal(t, " Container detect terminated with error : For more info use `kubectl logs -n test some-name -c prepare`", podTeminationMessage)
		})
	})
}
