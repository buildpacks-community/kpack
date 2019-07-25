package builder

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-beam/pkg/apis/build/v1alpha1"
)

func TestEnqueueAfter(t *testing.T) {
	builder := &v1alpha1.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name: "name",
		},
	}

	enqueuer := &workQueueEnqueuer{
		delay: time.Minute,
		enqueueAfter: func(obj interface{}, after time.Duration) {
			require.Equal(t, builder, obj)
			require.Equal(t, after, time.Minute)
		},
	}

	err := enqueuer.Enqueue(builder)
	require.NoError(t, err)
}
