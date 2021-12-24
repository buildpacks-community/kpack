package buildchange

import (
	"fmt"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func NewTriggerChange(dateStr string) Change {
	format := "A new build was manually triggered on %s"
	message := fmt.Sprintf(format, dateStr)

	return triggerChange{
		message: message,
	}
}

type triggerChange struct {
	message string
}

func (t triggerChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonTrigger }

func (t triggerChange) IsBuildRequired() (bool, error) {
	return t.message != "", nil
}

func (t triggerChange) Old() interface{} { return "" }

func (t triggerChange) New() interface{} { return t.message }

func (t triggerChange) Priority() buildapi.BuildPriority { return buildapi.BuildPriorityHigh }
