package cmd

import "fmt"

// This is set by CI when creating release binaries
var (
	Version   = "0.0.0"
	Commit    = "000"
	Identifer = fmt.Sprintf("v%s (git sha: %s)", Version, Commit)
)
