package buildchange

import (
	"encoding/json"
	"strings"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pkg/errors"
)

const (
	reasonsSeparator = ","
	errorSeparator   = "\n"
)

func NewChangeProcessor() *ChangeProcessor {
	return &ChangeProcessor{
		changes: []GenericChange{},
		errStrs: []string{},
	}
}

type ChangeProcessor struct {
	changes []GenericChange
	errStrs []string
}

func (c *ChangeProcessor) Process(change Change) *ChangeProcessor {
	if change == nil {
		return c
	}

	buildRequired, err := change.IsBuildRequired()
	if err != nil {
		err := errors.Wrapf(err, "error determining if build is required for reason '%s'", change.Reason())
		c.errStrs = append(c.errStrs, err.Error())

	} else if buildRequired {
		c.changes = append(c.changes, newGenericChange(change))
	}

	return c
}

func (c *ChangeProcessor) Summarize() (ChangeSummary, error) {
	changesStr, err := c.changesStr()
	if err != nil {
		err := errors.Wrapf(err, "error generating changes string")
		c.errStrs = append(c.errStrs, err.Error())
	}

	summary, err := NewChangeSummary(c.hasChanges(), c.reasonsStr(), changesStr, c.priority())
	if err != nil {
		err := errors.Wrapf(err, "error summarizing changes")
		c.errStrs = append(c.errStrs, err.Error())
	}

	if len(c.errStrs) > 0 {
		return summary, errors.New(strings.Join(c.errStrs, errorSeparator))
	}

	return summary, nil
}

func (c *ChangeProcessor) hasChanges() bool {
	return len(c.changes) > 0
}

func (c *ChangeProcessor) reasonsStr() string {
	if !c.hasChanges() {
		return ""
	}

	var reasons = make([]string, len(c.changes))
	for i, change := range c.changes {
		reasons[i] = change.Reason
	}

	return strings.Join(reasons, reasonsSeparator)
}

func (c *ChangeProcessor) changesStr() (string, error) {
	if !c.hasChanges() {
		return "", nil
	}

	bytes, err := json.Marshal(c.changes)
	if err != nil {
		return "", err
	}

	return string(bytes), err
}

func (c *ChangeProcessor) priority() buildapi.BuildPriority {
	priority := buildapi.BuildPriority(buildapi.BuildPriorityNone)
	if !c.hasChanges() {
		return priority
	}

	for _, change := range c.changes {
		if change.Priority > priority {
			priority = change.Priority
		}
	}
	return priority
}
