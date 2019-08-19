package reconciler

import (
	"time"

	"go.uber.org/zap"

	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

type Options struct {
	Logger *zap.SugaredLogger

	Client                  versioned.Interface
	ResyncPeriod            time.Duration
	SourcePollingFrequency  time.Duration
	BuilderPollingFrequency time.Duration
}

func (o Options) TrackerResyncPeriod() time.Duration {
	return o.ResyncPeriod * 3
}
