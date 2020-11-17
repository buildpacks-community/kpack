package buildchange

import (
	"sort"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

// kp-cli uses time.Now().String() which looks like this
const triggerTimeLayout = "2006-01-02 15:04:05.000000 -0700 MST m=+0.000000000"

type Change interface {
	Reason() v1alpha1.BuildReason
	IsValid() bool
}

type TriggerChange struct {
	New string `json:"new" mapstructure:"new"`
}

func (t TriggerChange) Reason() v1alpha1.BuildReason {
	return v1alpha1.BuildReasonTrigger
}

func (t TriggerChange) IsValid() bool {
	_, err := time.Parse(triggerTimeLayout, t.New)
	return err == nil
}

type CommitChange struct {
	// Git revisions
	Old string `json:"old" mapstructure:"old"`
	New string `json:"new" mapstructure:"new"`
}

func (c CommitChange) Reason() v1alpha1.BuildReason {
	return v1alpha1.BuildReasonCommit
}

func (c CommitChange) IsValid() bool {
	return c.Old != c.New
}

type ConfigChange struct {
	Old Config `json:"old" mapstructure:"old"`
	New Config `json:"new" mapstructure:"new"`
}

type Config struct {
	Env       []corev1.EnvVar             `json:"env,omitempty" mapstructure:"env,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty" mapstructure:"resources,omitempty"`
	Bindings  v1alpha1.Bindings           `json:"bindings,omitempty" mapstructure:"bindings,omitempty"`
	Source    v1alpha1.SourceConfig       `json:"source,omitempty" mapstructure:"source,omitempty"`
}

func (c ConfigChange) Reason() v1alpha1.BuildReason {
	return v1alpha1.BuildReasonConfig
}

func (c ConfigChange) IsValid() bool {
	// Git revision changes are considered as CommitChange
	// Ignore them as part of ConfigChange
	var oldGitRevision, newGitRevision string

	if c.Old.Source.Git != nil {
		oldGitRevision = c.Old.Source.Git.Revision
		c.Old.Source.Git.Revision = ""
	}
	if c.New.Source.Git != nil {
		newGitRevision = c.New.Source.Git.Revision
		c.New.Source.Git.Revision = ""
	}

	valid := !equality.Semantic.DeepEqual(c.Old, c.New)

	if c.Old.Source.Git != nil {
		c.Old.Source.Git.Revision = oldGitRevision
	}
	if c.New.Source.Git != nil {
		c.New.Source.Git.Revision = newGitRevision
	}
	return valid
}

type BuildpackChange struct {
	Old []BuildpackInfo `json:"old" mapstructure:"old"`
	New []BuildpackInfo `json:"new" mapstructure:"new"`
}

type BuildpackInfo struct {
	Id      string `json:"id" mapstructure:"id"`
	Version string `json:"version" mapstructure:"version"`
}

func (b BuildpackChange) Reason() v1alpha1.BuildReason {
	return v1alpha1.BuildReasonBuildpack
}

func (b BuildpackChange) IsValid() bool {
	sort.Slice(b.Old, func(i, j int) bool {
		return b.Old[i].Id < b.Old[j].Id
	})
	sort.Slice(b.New, func(i, j int) bool {
		return b.New[i].Id < b.New[j].Id
	})
	return !cmp.Equal(b.Old, b.New)
}

type StackChange struct {
	// Run images
	Old string `json:"old" mapstructure:"old"`
	New string `json:"new" mapstructure:"new"`
}

func (s StackChange) Reason() v1alpha1.BuildReason {
	return v1alpha1.BuildReasonStack
}

func (s StackChange) IsValid() bool {
	return s.Old != s.New
}
