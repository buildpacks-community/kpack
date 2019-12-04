package test

import (
	"testing"
	"time"
)

func eventually(t *testing.T, fun func() bool, interval time.Duration, duration time.Duration) {
	t.Helper()
	endTime := time.Now().Add(duration)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for currentTime := range ticker.C {
		if endTime.Before(currentTime) {
			t.Fatal("time is up")
		}
		if fun() {
			return
		}
	}
}
