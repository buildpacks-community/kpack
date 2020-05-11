package sourceresolver

import (
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"
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
