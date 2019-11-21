package v1alpha1_test

import (
	"testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRebaseable(t *testing.T) {
	build := &v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack,
			},
		},
	}
	require.False(t, build.Rebasable("any.stack"))

	build = &v1alpha1.Build{
		Spec: v1alpha1.BuildSpec{
			LastBuild: &v1alpha1.LastBuild{
				Image:   "some/run",
				StackID: "matching.stack",
			},
		},
	}
	require.False(t, build.Rebasable("matching.stack"))

	build = &v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack,
			},
		},
		Spec: v1alpha1.BuildSpec{
			LastBuild: &v1alpha1.LastBuild{
				Image:   "some/run",
				StackID: "matching.stack",
			},
		},
		Status: v1alpha1.BuildStatus{},
	}
	require.True(t, build.Rebasable("matching.stack"))

}
