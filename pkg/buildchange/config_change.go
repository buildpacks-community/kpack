package buildchange

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func NewConfigChange(oldConfig, newConfig Config) Change {
	return configChange{
		old: oldConfig,
		new: newConfig,
	}
}

type configChange struct {
	old Config
	new Config
}

type Config struct {
	Env         []corev1.EnvVar             `json:"env,omitempty"`
	Resources   corev1.ResourceRequirements `json:"resources,omitempty"`
	Services    buildapi.Services           `json:"services,omitempty"`
	CNBBindings corev1alpha1.CNBBindings    `json:"cnbBindings,omitempty"`
	Source      corev1alpha1.SourceConfig   `json:"source,omitempty"`
}

func (c configChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonConfig }

func (c configChange) IsBuildRequired() (bool, error) {
	// Git revision changes are considered as COMMIT change
	// Ignore them as part of CONFIG Change
	var oldGitRevision, newGitRevision string

	if c.old.Source.Git != nil {
		oldGitRevision = c.old.Source.Git.Revision
		c.old.Source.Git.Revision = ""
	}
	if c.new.Source.Git != nil {
		newGitRevision = c.new.Source.Git.Revision
		c.new.Source.Git.Revision = ""
	}

	valid := !equality.Semantic.DeepEqual(c.old, c.new)

	if c.old.Source.Git != nil {
		c.old.Source.Git.Revision = oldGitRevision
	}
	if c.new.Source.Git != nil {
		c.new.Source.Git.Revision = newGitRevision
	}
	return valid, nil
}

func (c configChange) Old() interface{} { return c.old }

func (c configChange) New() interface{} { return c.new }

func (c configChange) Priority() buildapi.BuildPriority { return buildapi.BuildPriorityHigh }
