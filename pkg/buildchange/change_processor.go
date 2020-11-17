package buildchange

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const (
	reasonsSeparator = ","
)

func NewChangeProcessor() *ChangeProcessor {
	return &ChangeProcessor{
		changes: map[v1alpha1.BuildReason]Change{},
	}
}

type ChangeProcessor struct {
	changes map[v1alpha1.BuildReason]Change
}

func (c *ChangeProcessor) Process(change Change) *ChangeProcessor {
	if change.IsValid() && change.Reason().IsValid() {
		c.changes[change.Reason()] = change
	}
	return c
}

func (c *ChangeProcessor) Summarize() (ChangeSummary, error) {
	var summary ChangeSummary
	changesStr, err := c.ChangesStr()
	if err != nil {
		return summary, errors.Wrapf(err, "error summarizing changes")
	}

	summary, err = NewChangeSummary(c.HasChanges(), c.ReasonsStr(), changesStr)
	if err != nil {
		return summary, errors.Wrapf(err, "error summarizing changes")
	}
	return summary, nil
}

func (c *ChangeProcessor) HasChanges() bool {
	return len(c.changes) > 0
}

func (c *ChangeProcessor) ReasonsStr() string {
	if c.HasChanges() {
		return strings.Join(c.reasons(), reasonsSeparator)
	} else {
		return ""
	}
}

func (c *ChangeProcessor) ChangesStr() (string, error) {
	if !c.HasChanges() {
		return "", nil
	}

	b, err := json.Marshal(c.changes)
	if err != nil {
		return "", errors.Wrap(err, "Error marshalling build changes to json")
	}
	return string(b), err
}

func (c *ChangeProcessor) reasons() []string {
	var reasons = make([]string, len(c.changes))
	var index int

	for reason := range c.changes {
		reasons[index] = string(reason)
		index++
	}

	sort.SliceStable(reasons, func(i, j int) bool {
		return strings.Index(v1alpha1.BuildReasonSortIndex, reasons[i]) <
			strings.Index(v1alpha1.BuildReasonSortIndex, reasons[j])
	})
	return reasons
}
