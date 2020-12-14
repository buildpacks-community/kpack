package buildchange

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/differ"
)

const differPrefix = "\t"

func Log(logger *log.Logger, changesStr string) error {
	return NewChangeLogger(logger, changesStr).Log()
}

func NewChangeLogger(logger *log.Logger, changesStr string) *changeLogger {
	options := differ.DefaultOptions()
	options.Prefix = differPrefix

	return &changeLogger{
		logger:     logger,
		changesStr: changesStr,
		differ:     differ.NewDiffer(options),
	}
}

type changeLogger struct {
	logger     *log.Logger
	changesStr string

	differ  differ.Differ
	reasons []string
	changes []GenericChange
}

func (c *changeLogger) Log() error {
	if c.changesStr == "" {
		return nil
	}

	if err := c.parseChanges(); err != nil {
		return errors.Wrapf(err, "error parsing build changes JSON string '%s'", c.changesStr)
	}
	c.parseReasons()

	c.logReasons()
	return c.logChanges()
}

func (c *changeLogger) parseChanges() error {
	c.changes = []GenericChange{}
	return json.Unmarshal([]byte(c.changesStr), &c.changes)
}

func (c *changeLogger) parseReasons() {
	c.reasons = make([]string, len(c.changes))
	for i, change := range c.changes {
		c.reasons[i] = change.Reason
	}
}

func (c *changeLogger) logReasons() {
	reasons := strings.Join(c.reasons, reasonsSeparator)
	c.logger.Printf("Build reason(s): %s\n", reasons)
}

func (c *changeLogger) logChanges() error {
	for _, change := range c.changes {
		diff, err := c.differ.Diff(change.Old, change.New)
		if err != nil {
			return errors.Wrapf(err, "error logging change for reason '%s'", change.Reason)
		}

		changeHeader := fmt.Sprintf("%s:\n", change.Reason)
		c.logger.Printf(changeHeader)
		c.logger.Print(diff)
	}
	return nil
}
