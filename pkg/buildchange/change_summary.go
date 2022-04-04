package buildchange

import (
	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pkg/errors"
)

type ChangeSummary struct {
	HasChanges bool
	ReasonsStr string
	ChangesStr string
	Priority   buildapi.BuildPriority
}

func NewChangeSummary(hasChanges bool, reasonsStr, changesStr string, priority buildapi.BuildPriority) (ChangeSummary, error) {
	cs := ChangeSummary{
		HasChanges: hasChanges,
		ReasonsStr: reasonsStr,
		ChangesStr: changesStr,
		Priority:   priority,
	}

	if !cs.IsValid() {
		return cs, errors.Errorf("invalid change summary '%+v'", cs)
	}
	return cs, nil
}

func (c ChangeSummary) IsValid() bool {
	if c.HasChanges {
		return c.ReasonsStr != "" && c.ChangesStr != ""
	} else {
		return c.ReasonsStr == "" && c.ChangesStr == ""
	}
}
