package buildchange

import (
	"fmt"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
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

func (t triggerChange) Reason() v1alpha1.BuildReason { return v1alpha1.BuildReasonTrigger }

func (t triggerChange) IsBuildRequired() (bool, error) {
	return t.message != "", nil
}

func (t triggerChange) Old() interface{} { return "" }

func (t triggerChange) New() interface{} { return t.message }
