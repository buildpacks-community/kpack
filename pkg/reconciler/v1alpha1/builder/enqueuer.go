package builder

import (
	"time"

	"github.com/pivotal/build-service-beam/pkg/apis/build/v1alpha1"
)

type workQueueEnqueuer struct {
	enqueueAfter func(obj interface{}, after time.Duration)
	delay        time.Duration
}

func (e *workQueueEnqueuer) Enqueue(builder *v1alpha1.Builder) error {
	e.enqueueAfter(builder, 1*time.Minute)
	return nil
}
