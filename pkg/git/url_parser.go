package git

import (
	"fmt"
	"regexp"
)

var shortScpRegex = regexp.MustCompile(`^(ssh://)?(.*)@([[:alnum:]\.-]+):(.*)$`)

// ParseURL converts a short scp-like SSH syntax to a proper SSH URL.
// Git's ssh protocol supports a url like user@hostname:path syntax, which is
// not a valid ssh url but is inherited from scp. Because the library we
// use for git relies on the Golang SSH support, we need to convert it to a
// proper SSH URL.
// See https://git-scm.com/book/en/v2/Git-on-the-Server-The-Protocols
func parseURL(url string) string {
	parts := shortScpRegex.FindStringSubmatch(url)
	if len(parts) == 0 {
		return url
	}
	if parts[1] == "ssh://" {
		return url
	}

	return fmt.Sprintf("ssh://%v@%v/%v", parts[2], parts[3], parts[4])
}
