package test

import (
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
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
		imageTag:     registry + "/kpack-test",
	}
}

func (c *config) newImageTag() string {
	genTag := c.imageTag + "-" + strconv.Itoa(rand.Int())
	return genTag
}

type dockerCredentials map[string]authn.AuthConfig

type dockerConfigJson struct {
	Auths dockerCredentials `json:"auths"`
}
