package git

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	fixtures "gopkg.in/src-d/go-git-fixtures.v3"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
)

func TestRemoteGitResolver(t *testing.T) {
	spec.Run(t, "TestRemoteGitResolver", testRemoteGitResolver)
}

func testRemoteGitResolver(t *testing.T, when spec.G, it spec.S) {
	const (
		nonHEADCommit           = "a755256fc0a57241b92167eb748b333449a3d7e9"
		fixtureHEADMasterCommit = "6ecf0ef2c2dffb796033e5a02219af86ec6584e5"
		tag                     = "commit-tag"
		tagCommit               = "ad7897c0fb8e7d9a9ba41fa66072cf06095a6cfc"
	)

	when("#Resolve", func() {
		when("source is a commit", func() {
			it("returns type commit", func() {
				repo := fixtures.Basic().One()

				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(anonymousAuth{}, v1alpha1.Git{
					URL:      repo.URL,
					Revision: nonHEADCommit,
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, &v1alpha1.ResolvedGitSource{
					URL:      repo.URL,
					Revision: nonHEADCommit,
					Type:     v1alpha1.Commit,
				})
			})
		})

		when("source is a branch", func() {
			it("returns branch with resolved commit", func() {
				repo := fixtures.Basic().One()

				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(anonymousAuth{}, v1alpha1.Git{
					URL:      repo.URL,
					Revision: "master",
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, &v1alpha1.ResolvedGitSource{
					URL:      repo.URL,
					Revision: fixtureHEADMasterCommit,
					Type:     v1alpha1.Branch,
				})
			})
		})

		when("source is a tag", func() {
			it("returns tag with resolved commit", func() {
				repo := fixtures.ByTag("tags").One()

				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(anonymousAuth{}, v1alpha1.Git{
					URL:      repo.URL,
					Revision: tag,
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, &v1alpha1.ResolvedGitSource{
					URL:      repo.URL,
					Revision: tagCommit,
					Type:     v1alpha1.Tag,
				})
			})
		})

		when("authentication fails", func() {
			it("returns an unknown type", func() {
				repo := fixtures.ByTag("tags").One()

				gitResolver := &remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(basicAuth{
					Username: "notgonna",
					Password: "work",
				}, v1alpha1.Git{
					URL:      repo.URL,
					Revision: tag,
				})
				require.NoError(t, err)

				assert.Equal(t, resolvedGitSource, &v1alpha1.ResolvedGitSource{
					URL:      repo.URL,
					Revision: tag,
					Type:     v1alpha1.Unknown,
				})
			})
		})
	})
}
