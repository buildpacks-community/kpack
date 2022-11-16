package git

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
)

func TestParseURL(t *testing.T) {
	spec.Focus(t, "Test Parse Git URL", testParseURL)
}

func testParseURL(t *testing.T, when spec.G, it spec.S) {
	type entry struct {
		url      string
		expected string
	}

	testURLs := func(table []entry) {
		for _, e := range table {
			assert.Equal(t, e.expected, parseURL(e.url))
		}
	}

	// https: //stackoverflow.com/questions/31801271/what-are-the-supported-git-url-formats
	when("using the local protcol", func() {
		it("parses the url correctly", func() {
			testURLs([]entry{
				{url: "/path/to/repo.git", expected: "/path/to/repo.git"},
				{url: "file:///path/to/repo.git", expected: "file:///path/to/repo.git"},
			})
		})
	})
	when("using the https protcol", func() {
		it("parses the url correctly", func() {
			testURLs([]entry{
				{url: "http://host.xz/path/to/repo.git", expected: "http://host.xz/path/to/repo.git"},
				{url: "https://host.xz/path/to/repo.git", expected: "https://host.xz/path/to/repo.git"},
			})
		})
	})
	when("using the ssh protcol", func() {
		it("parses the url correctly", func() {
			testURLs([]entry{
				{url: "ssh://user@host.xz:port/path/to/repo.git/", expected: "ssh://user@host.xz:port/path/to/repo.git/"},
				{url: "ssh://user@host.xz/path/to/repo.git/", expected: "ssh://user@host.xz/path/to/repo.git/"},
				{url: "user@host.xz:path/to/repo.git", expected: "ssh://user@host.xz/path/to/repo.git"},
			})
		})
	})
	when("using the git protcol", func() {
		it("parses the url correctly", func() {
			testURLs([]entry{
				{url: "git://host.xz/path/to/repo.git", expected: "git://host.xz/path/to/repo.git"},
			})
		})
	})
}
