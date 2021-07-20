package sourceresolver

import (
	"time"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type workQueueEnqueuer struct {
	enqueueAfter func(obj interface{}, after time.Duration)
	delay        time.Duration
}

func (e *workQueueEnqueuer) Enqueue(sr *buildapi.SourceResolver) error {
	e.enqueueAfter(sr, 1*time.Minute)
	return nil
}
