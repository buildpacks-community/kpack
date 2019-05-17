package reconciler

import (
	"time"

	"github.com/knative/pkg/controller"
	"go.uber.org/zap"

	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
)

type Options struct {
	Logger *zap.SugaredLogger

	CNBBuildClient versioned.Interface
	ResyncPeriod   time.Duration
}

func MustNewStatsReporter(reconciler string, logger *zap.SugaredLogger) controller.StatsReporter {
	stats, err := controller.NewStatsReporter(reconciler)
	if err != nil {
		logger.Fatal("Failed to initialize the stats reporter.", zap.Error(err))
	}
	return stats
}

func (o Options) TrackerResyncPeriod() time.Duration {
	return o.ResyncPeriod * 3
}
