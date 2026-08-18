package compat

import "os"

func init() {
	// Disable WatchListClient by default to prevent issues with older clusters and test fakes
	// missing bookmark events (which causes informers to hang).
	if _, ok := os.LookupEnv("KUBE_FEATURE_WatchListClient"); !ok {
		os.Setenv("KUBE_FEATURE_WatchListClient", "false")
	}
}
