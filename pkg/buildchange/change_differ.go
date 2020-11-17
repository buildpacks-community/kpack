package buildchange

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/pivotal/kpack/pkg/differ"
)

const changeDiffPrefix = "\t"

type ChangeDiffer interface {
	ChangeDiff() (string, error)
}

type CommitDiffer struct {
	change CommitChange
	differ differ.Differ
}

func (c CommitDiffer) ChangeDiff() (string, error) {
	old := fmt.Sprintf("Revision: %s", c.change.Old)
	new := fmt.Sprintf("Revision: %s", c.change.New)
	return c.differ.Diff(old, new)
}

type TriggerDiffer struct {
	change TriggerChange
}

func (t TriggerDiffer) ChangeDiff() (string, error) {
	parsedTime, err := time.Parse(triggerTimeLayout, t.change.New)
	if err != nil {
		return "", err
	}

	format := "A new build was manually triggered on %s"
	return fmt.Sprintf(format, parsedTime.Format(time.RFC1123Z)), nil
}

type StackDiffer struct {
	change StackChange
	differ differ.Differ
}

func (s StackDiffer) ChangeDiff() (string, error) {
	old := fmt.Sprintf("RunImage: %s", s.change.Old)
	new := fmt.Sprintf("RunImage: %s", s.change.New)
	return s.differ.Diff(old, new)
}

type BuildpackDiffer struct {
	change BuildpackChange
	differ differ.Differ
	sb     strings.Builder
}

func (b BuildpackDiffer) ChangeDiff() (string, error) {
	old, err := b.buildpackInfo(b.change.Old)
	if err != nil {
		return "", err
	}

	new, err := b.buildpackInfo(b.change.New)
	if err != nil {
		return "", err
	}
	return b.differ.Diff(old, new)
}

func (b BuildpackDiffer) buildpackInfo(infos []BuildpackInfo) (string, error) {
	b.sb.Reset()
	for _, info := range infos {
		if _, err := b.sb.WriteString(fmt.Sprintf("%s\t%s\n", info.Id, info.Version)); err != nil {
			return "", err
		}
	}
	return b.sb.String(), nil
}

type ConfigDiffer struct {
	change ConfigChange
	differ differ.Differ

	oldMap map[string]interface{}
	newMap map[string]interface{}
}

func (c ConfigDiffer) ChangeDiff() (string, error) {
	c.filter("env", c.change.Old.Env, c.change.New.Env)
	c.filter("resources", c.change.Old.Resources, c.change.New.Resources)
	c.filter("bindings", c.change.Old.Bindings, c.change.New.Bindings)
	c.filter("source", c.change.Old.Source, c.change.New.Source)

	return c.differ.Diff(c.oldMap, c.newMap)
}

func (c *ConfigDiffer) filter(field string, o, n interface{}) {
	if cmp.Equal(o, n) {
		return
	}
	if !isTrueNil(o) {
		c.oldMap[field] = o
	}
	if !isTrueNil(n) {
		c.newMap[field] = n
	}
}

// why reflect? handle bindings
// https://stackoverflow.com/questions/21460787/nil-slice-when-passed-as-interface-is-not-nil-why-golang
func isTrueNil(i interface{}) bool {
	if i == nil {
		return true
	}

	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Slice && v.IsNil() {
		return true
	}
	return false
}
