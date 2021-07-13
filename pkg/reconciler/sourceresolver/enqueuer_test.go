package sourceresolver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func TestEnqueueAfter(t *testing.T) {
	sourceResolver := &buildapi.SourceResolver{
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
