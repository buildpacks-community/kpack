package git

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
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
				gitResolver := remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(anonymousAuth, corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      url,
						Revision: nonHEADCommit,
					},
					SubPath: "/foo/bar",
				})
				require.NoError(t, err)

				assert.Equal(t, corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      url,
						Revision: nonHEADCommit,
						SubPath:  "/foo/bar",
						Type:     corev1alpha1.Commit,
					},
				}, resolvedGitSource)
			})
		})

		when("source is a branch", func() {
			it("returns branch with resolved commit", func() {
				gitResolver := remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(anonymousAuth, corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      url,
						Revision: "master",
					},
					SubPath: "/foo/bar",
				})
				require.NoError(t, err)

				assert.Equal(t, corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      url,
						Revision: fixtureHEADMasterCommit,
						Type:     corev1alpha1.Branch,
						SubPath:  "/foo/bar",
					},
				}, resolvedGitSource)
			})
		})

		when("source is a tag", func() {
			it("returns tag with resolved commit", func() {
				tagsUrl := "https://github.com/git-fixtures/tags.git"

				gitResolver := remoteGitResolver{}

				resolvedGitSource, err := gitResolver.Resolve(anonymousAuth, corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      tagsUrl,
						Revision: tag,
					},
					SubPath: "/foo/bar",
				})
				require.NoError(t, err)

				assert.Equal(t, corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      tagsUrl,
						Revision: tagCommit,
						Type:     corev1alpha1.Tag,
						SubPath:  "/foo/bar",
					},
				}, resolvedGitSource)
			})
		})

		when("authentication fails", func() {
			it("returns an unknown type", func() {
				gitResolver := remoteGitResolver{}

				resolvedGitSource, _ := gitResolver.Resolve(&http.BasicAuth{
					Username: "bad-username",
					Password: "bad-password",
				}, corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      "git@localhost:org/repo",
						Revision: tag,
					},
					SubPath: "/foo/bar",
				})

				assert.Equal(t, corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      "git@localhost:org/repo",
						Revision: tag,
						Type:     corev1alpha1.Unknown,
						SubPath:  "/foo/bar",
					},
				}, resolvedGitSource)
			})
		})
	})
}
