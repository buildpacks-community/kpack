package buildchange

import (
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func Log(logger *log.Logger, reasonsStr, changesStr string) error {
	return NewChangeLogger(logger, reasonsStr, changesStr).Log()
}

type ChangeLogger struct {
	logger     *log.Logger
	reasonsStr string
	changesStr string

	reasons    []v1alpha1.BuildReason
	changesMap map[v1alpha1.BuildReason]Change
}

func NewChangeLogger(logger *log.Logger, reasonsStr, changesStr string) *ChangeLogger {
	return &ChangeLogger{
		logger:     logger,
		reasonsStr: reasonsStr,
		changesStr: changesStr,
	}
}

func (c *ChangeLogger) Log() error {
	if err := c.validate(); err != nil {
		return errors.Wrapf(err, "error validating")
	}

	if err := c.parseReasons(); err != nil {
		return errors.Wrapf(err, "error parsing build reasons string '%s'", c.reasonsStr)
	}

	if err := c.parseChanges(); err != nil {
		return errors.Wrapf(err, "error parsing build changes JSON string '%s'", c.changesStr)
	}

	c.logReasons()
	return c.logChanges()
}

func (c *ChangeLogger) validate() error {
	if c.reasonsStr == "" {
		return errors.New("build reasons is empty")
	}
	if c.changesStr == "" {
		return errors.New("build changes is empty")
	}
	return nil
}

func (c *ChangeLogger) parseReasons() error {
	reasons := strings.Split(c.reasonsStr, reasonsSeparator)

	c.reasons = make([]v1alpha1.BuildReason, len(reasons))
	var invalids []string

	for i, reason := range reasons {
		buildReason := v1alpha1.BuildReason(reason)
		if buildReason.IsValid() {
			c.reasons[i] = buildReason
		} else {
			invalids = append(invalids, reason)
		}
	}

	if len(invalids) > 0 {
		return errors.Errorf("invalid reason(s): %s", strings.Join(invalids, ","))
	}
	return nil
}

func (c *ChangeLogger) parseChanges() (err error) {
	c.changesMap, err = NewChangeParser().Parse(c.changesStr)
	return err
}

func (c *ChangeLogger) logReasons() {
	c.logger.Printf("Build reason(s): %s\n", c.reasonsStr)
}

func (c *ChangeLogger) logChanges() error {
	for _, reason := range c.reasons {
		change, ok := c.changesMap[reason]
		if !ok {
			return errors.Errorf("changes not available for the reason '%s'", reason)
		}

		if err := c.logChange(change); err != nil {
			return errors.Errorf("error logging change for the reason '%s'", reason)
		}
	}
	return nil
}

func (c *ChangeLogger) logChange(change Change) error {
	differ, err := NewChangeDiffer(change)
	if err != nil {
		return errors.Wrap(err, "error generating differ")
	}

	diff, err := differ.ChangeDiff()
	if err != nil {
		return errors.Wrap(err, "error generating diff")
	}

	switch change.(type) {
	case TriggerChange:
		c.logger.Print(fmt.Sprintf("%s: %s\n", change.Reason(), diff))
	default:
		changeHeader := fmt.Sprintf("%s change:\n", change.Reason())
		c.logger.Printf(changeHeader)
		c.logger.Print(diff)
	}
	return nil
}
