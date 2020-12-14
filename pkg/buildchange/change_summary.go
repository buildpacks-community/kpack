package buildchange

import "github.com/pkg/errors"

type ChangeSummary struct {
	HasChanges bool
	ReasonsStr string
	ChangesStr string
}

func NewChangeSummary(hasChanges bool, reasonsStr, changesStr string) (ChangeSummary, error) {
	cs := ChangeSummary{
		HasChanges: hasChanges,
		ReasonsStr: reasonsStr,
		ChangesStr: changesStr,
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
