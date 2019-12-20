package test

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
)

type config struct {
	builder      string
	testRegistry string
	imageTag     string

	generatedImageNames []string
}

func loadConfig(t *testing.T) config {
	registry, found := os.LookupEnv("IMAGE_REGISTRY")
	if !found {
		t.Fatal("IMAGE_REGISTRY env is needed for tests")
	}

	return config{
		testRegistry: registry,
		imageTag:     registry + "/kpack-test",
	}
}

func (c *config) newImageTag() string {
	genTag := c.imageTag + "-" + strconv.Itoa(rand.Int())
	c.generatedImageNames = append(c.generatedImageNames, genTag)
	return genTag
}

func (c *config) stackTag() string {
	return c.testRegistry + "/stack"
}

func (c *config) buildpackageTag(name string) string {
	return fmt.Sprintf("%s/%s-buildpackage", c.testRegistry, name)
}
