package clusterbuilder

import (
	"time"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type workQueueEnqueuer struct {
	enqueueAfter func(obj interface{}, after time.Duration)
	delay        time.Duration
}

func (e *workQueueEnqueuer) Enqueue(builder *v1alpha1.ClusterBuilder) error {
	e.enqueueAfter(builder, 1*time.Minute)
	return nil
}
