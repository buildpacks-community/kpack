package pkg

import (
	"testing"

	"github.com/matthewmcnew/archtest"
)

func TestDependencies(t *testing.T) {
	archtest.Package(t, "github.com/pivotal/kpack/...").
		IncludeTests().
		ShouldNotDependDirectlyOn("gotest.tools/...", "github.com/tj/assert/...")
}
