package pkg

import (
	"testing"

	"github.com/matthewmcnew/archtest"
)

func TestDependencies(t *testing.T) {
	archtest.Package(t, "github.com/pivotal/kpack/...").
		IncludeTests().
		Ignoring("github.com/pivotal/kpack/hack").
		ShouldNotDependDirectlyOn(
			"gotest.tools/v3/assert",
			"gotest.tools/v3/assert/cmp",
			"gotest.tools/assert",
			"gotest.tools/assert/cmp",

			"github.com/tj/assert",
		)
}
