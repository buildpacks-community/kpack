package v1alpha1

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestRebaseable(t *testing.T) {
	build := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				BuildReasonAnnotation: BuildReasonStack,
			},
		},
	}
	require.False(t, build.rebasable("any.stack"))

	build = &Build{
		Spec: BuildSpec{
			LastBuild: &LastBuild{
				Image:   "some/run",
				StackId: "matching.stack",
			},
		},
	}
	require.False(t, build.rebasable("matching.stack"))

	build = &Build{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				BuildReasonAnnotation: BuildReasonStack,
			},
		},
		Spec: BuildSpec{
			LastBuild: &LastBuild{
				Image:   "some/run",
				StackId: "matching.stack",
			},
		},
		Status: BuildStatus{},
	}
	require.True(t, build.rebasable("matching.stack"))

}

func TestBuildReason(t *testing.T) {
	build := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				BuildReasonAnnotation: BuildReasonStack,
			},
		},
	}
	require.True(t, build.BuildReason() == BuildReasonStack)
}

func TestBuildLifecycle(t *testing.T) {
	build := &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-name",
		},
	}
	build.Status.Error(errors.New("error: display this error"))

	require.True(t, equality.Semantic.DeepEqual(build, &Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-name",
		},
		Status: BuildStatus{
			Status: corev1alpha1.Status{
				Conditions: corev1alpha1.Conditions{
					{
						Type:    corev1alpha1.ConditionSucceeded,
						Status:  corev1.ConditionFalse,
						Message: "error: display this error",
					},
				},
			},
		},
	}))
}
