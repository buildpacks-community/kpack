package git

import (
	"context"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"time"
)

type Reconciler interface {
	Reconcile(ctx context.Context, key string) error
}

type Poller struct {
	Reconciler Reconciler
	Logger     *zap.SugaredLogger
	PollChan   <-chan string
}

func (p *Poller) Run(stopChan <-chan struct{}) error {

	for {
		select {
		case <-stopChan:
			return nil
		case key := <-p.PollChan:
			startTime := time.Now()
			//create context
			logger := p.Logger.With(zap.String("build.pivotal.io/poll", key))
			ctx := logging.WithLogger(context.TODO(), logger)

			if err := p.Reconciler.Reconcile(ctx, key); err != nil {
				logger.Infof("Polling failed. Time taken: %v.", time.Since(startTime))
			} else {
				logger.Infof("Polling succeeded. Time taken: %v.", time.Since(startTime))
			}
		}
	}
}
