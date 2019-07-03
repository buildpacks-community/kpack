package reconciler

import (
	"crypto/md5"
	"fmt"
)

// The longest name supported by the K8s is 63.
// These constants
const (
	longest = 63
	md5Len  = 32
	head    = longest - md5Len
)

// ChildName generates a name for the resource based upong the parent resource and suffix.
// If the concatenated name is longer than K8s permits the name is hashed and truncated to permit
// construction of the resource, but still keeps it unique.
func ChildName(parent, suffix string) string {
	n := parent
	if len(parent) > (longest - len(suffix)) {
		n = fmt.Sprintf("%s%x", parent[:head-len(suffix)], md5.Sum([]byte(parent)))
	}
	return n + suffix
}
