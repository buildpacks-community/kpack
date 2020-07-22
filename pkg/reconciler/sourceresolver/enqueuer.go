package sourceresolver

import (
	"time"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type workQueueEnqueuer struct {
	enqueueAfter func(obj interface{}, after time.Duration)
	delay        time.Duration
}

func (e *workQueueEnqueuer) Enqueue(sr *v1alpha1.SourceResolver) error {
	e.enqueueAfter(sr, 1*time.Minute)
	return nil
}
