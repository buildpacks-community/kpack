package sourceresolver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func TestEnqueueAfter(t *testing.T) {
	sourceResolver := &v1alpha1.SourceResolver{
		ObjectMeta: v1.ObjectMeta{
			Name: "name",
		},
	}

	enqueuer := &workQueueEnqueuer{
		delay: time.Minute,
		enqueueAfter: func(obj interface{}, after time.Duration) {
			require.Equal(t, sourceResolver, obj)
			require.Equal(t, after, time.Minute)
		},
	}

	err := enqueuer.Enqueue(sourceResolver)
	require.NoError(t, err)
}
