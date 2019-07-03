package git

import (
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
	"time"
)

type SourceResolverEnqueuer struct {
	Frequency            time.Duration
	PollChan             chan<- string
	SourceResolverLister v1alpha1Listers.SourceResolverLister
}

func (t *SourceResolverEnqueuer) Run(stopChan <-chan struct{}) error {
	for {
		sourceResolvers, err := t.SourceResolverLister.List(labels.Everything())
		if err != nil {
			return err
		}

		for _, sr := range sourceResolvers {
			if !sr.PollingReady() {
				continue
			}

			select {
			case t.PollChan <- key(sr):
			case <-stopChan:
				return nil
			}
		}

		select {
		case <-stopChan:
			return nil
		case <-time.After(t.Frequency):
		}
	}
}

func key(sr *v1alpha1.SourceResolver) string {
	return sr.GetNamespace() + "/" + sr.Name
}
