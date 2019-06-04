package test

import (
	"math/rand"
	"os"
	"testing"
	"time"
)

type config struct {
	builder          string
	testRegistry     string
	imageTag         string
}

func loadConfig(t *testing.T) config {
	registry := lookup("IMAGE_REGISTRY", "registry.default.svc.cluster.local:5000")

	return config{
		builder:          lookup("BUILDER", "registry.default.svc.cluster.local:5000/builder-system:local"),
		testRegistry:     registry,
		imageTag:         registryTag(registry),
	}
}

func lookup(name, defaultVal string) string {
	val, found := os.LookupEnv(name)
	if !found {
		return defaultVal
	}
	return val
}

func registryTag(registry string) string {
	return registry + "/" + randString(5)
}

func randString(n int) string {
	randomStream := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a' + byte(randomStream.Intn(26))
	}
	return string(b)
}

