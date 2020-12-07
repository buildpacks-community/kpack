package buildchange

import (
	"fmt"
	"time"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

// kp-cli uses time.Now().String() which looks like this
const expectedTimeLayout = "2006-01-02 15:04:05.000000 -0700 MST m=+0.000000000"

func NewTriggerChange(dateStr string) Change {
	parsedTime, err := time.Parse(expectedTimeLayout, dateStr)
	if err != nil {
		return triggerChange{err: err}
	}

	format := "A new build was manually triggered on %s"
	message := fmt.Sprintf(format, parsedTime.Format(time.RFC1123Z))

	return triggerChange{
		message: message,
		err:     err,
	}
}

type triggerChange struct {
	message string
	err     error
}

func (t triggerChange) Reason() v1alpha1.BuildReason { return v1alpha1.BuildReasonTrigger }

func (t triggerChange) IsBuildRequired() (bool, error) {
	return t.message != "", t.err
}

func (t triggerChange) Old() interface{} { return "" }

func (t triggerChange) New() interface{} { return t.message }
