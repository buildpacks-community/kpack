package git

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func TestRemoteGitResolver(t *testing.T) {
	spec.Run(t, "TestRemoteGitResolver", testRemoteGitResolver)
}

func testRemoteGitResolver(t *testing.T, when spec.G, it spec.S) {
	const (
		url                     = "https://github.com/git-fixtures/basic.git"
		nonHEADCommit           = "a755256fc0a57241b92167eb748b333449a3d7e9"
		fixtureHEADMasterCommit = "6ecf0ef2c2dffb796033e5a02219af86ec6584e5"
		tag                     = "commit-tag"
		tagCommit               = "ad7897c0fb8e7d9a9ba41fa66072cf06095a6cfc"
	)

	when("#Resolve", func() {
		when("source is a commit", func() {
			it("returns type commit", func() {
				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(&fakeGitKeychain{}, buildapi.SourceConfig{
					Git: &buildapi.Git{
						URL:      url,
						Revision: nonHEADCommit,
					},
					SubPath: "/foo/bar",
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, buildapi.ResolvedSourceConfig{
					Git: &buildapi.ResolvedGitSource{
						URL:      url,
						Revision: nonHEADCommit,
						SubPath:  "/foo/bar",
						Type:     buildapi.Commit,
					},
				})
			})
		})

		when("source is a branch", func() {
			it("returns branch with resolved commit", func() {
				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(&fakeGitKeychain{}, buildapi.SourceConfig{
					Git: &buildapi.Git{
						URL:      url,
						Revision: "master",
					},
					SubPath: "/foo/bar",
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, buildapi.ResolvedSourceConfig{
					Git: &buildapi.ResolvedGitSource{
						URL:      url,
						Revision: fixtureHEADMasterCommit,
						Type:     buildapi.Branch,
						SubPath:  "/foo/bar",
					},
				})
			})
		})

		when("source is a tag", func() {
			it("returns tag with resolved commit", func() {
				tagsUrl := "https://github.com/git-fixtures/tags.git"

				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(&fakeGitKeychain{}, buildapi.SourceConfig{
					Git: &buildapi.Git{
						URL:      tagsUrl,
						Revision: tag,
					},
					SubPath: "/foo/bar",
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, buildapi.ResolvedSourceConfig{
					Git: &buildapi.ResolvedGitSource{
						URL:      tagsUrl,
						Revision: tagCommit,
						Type:     buildapi.Tag,
						SubPath:  "/foo/bar",
					},
				})
			})
		})

		when("authentication fails", func() {
			it("returns an unknown type", func() {
				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(&fakeGitKeychain{}, buildapi.SourceConfig{
					Git: &buildapi.Git{
						URL:      "git@localhost:org/repo",
						Revision: tag,
					},
					SubPath: "/foo/bar",
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, buildapi.ResolvedSourceConfig{
					Git: &buildapi.ResolvedGitSource{
						URL:      "git@localhost:org/repo",
						Revision: tag,
						Type:     buildapi.Unknown,
						SubPath:  "/foo/bar",
					},
				})
			})
		})
	})
}
