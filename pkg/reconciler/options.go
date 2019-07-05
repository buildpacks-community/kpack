package reconciler

import (
	"time"

	"go.uber.org/zap"

	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
)

type Options struct {
	Logger *zap.SugaredLogger

	Client           versioned.Interface
	ResyncPeriod     time.Duration
	PollingFrequency time.Duration
}

func (o Options) TrackerResyncPeriod() time.Duration {
	return o.ResyncPeriod * 3
}
