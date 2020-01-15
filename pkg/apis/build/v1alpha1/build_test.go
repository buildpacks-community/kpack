package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
