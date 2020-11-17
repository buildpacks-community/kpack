package buildchange

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/differ"
)

func NewChangeDiffer(change Change) (ChangeDiffer, error) {
	var cd ChangeDiffer
	var err error

	switch change.(type) {
	case TriggerChange:
		cd = newTriggerDiffer(change)
	case StackChange:
		cd = newStackDiffer(change)
	case BuildpackChange:
		cd = newBuildpackDiffer(change)
	case ConfigChange:
		cd = newConfigDiffer(change)
	case CommitChange:
		cd = newCommitDiffer(change)
	default:
		err = errors.Errorf("diff provider not available for change '%s'", reflect.TypeOf(change).Name())
	}
	return cd, err
}

func newTriggerDiffer(change Change) TriggerDiffer {
	return TriggerDiffer{
		change: change.(TriggerChange),
	}
}

func newStackDiffer(change Change) StackDiffer {
	return StackDiffer{
		change: change.(StackChange),
		differ: newDiffer(),
	}
}

func newBuildpackDiffer(change Change) BuildpackDiffer {
	return BuildpackDiffer{
		change: change.(BuildpackChange),
		differ: newDiffer(),
		sb:     strings.Builder{},
	}
}

func newConfigDiffer(change Change) ConfigDiffer {
	return ConfigDiffer{
		change: change.(ConfigChange),
		differ: newDiffer(),
		oldMap: map[string]interface{}{},
		newMap: map[string]interface{}{},
	}
}

func newCommitDiffer(change Change) CommitDiffer {
	return CommitDiffer{
		change: change.(CommitChange),
		differ: newDiffer(),
	}
}

func newDiffer() differ.Differ {
	options := differ.DefaultOptions()
	options.Prefix = changeDiffPrefix
	return differ.NewDiffer(options)
}
