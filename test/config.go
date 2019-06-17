package test

import (
	"os"
	"testing"
)

type config struct {
	builder      string
	testRegistry string
	imageTag     string
}

func loadConfig(t *testing.T) config {
	registry, found := os.LookupEnv("IMAGE_REGISTRY")
	if !found {
		t.Fatal("IMAGE_REGISTRY env is needed for tests")
	}

	return config{
		testRegistry: registry,
		imageTag:     registryTag(registry),
	}
}

func registryTag(registry string) string {
	return registry + "/build-service-system-test"
}
