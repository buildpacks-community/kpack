package test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/buildpack/imgutil"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fixtureBuilderLocation = "cloudfoundry/cnb"
	imageYmlFormat         = `---
apiVersion: build.pivotal.io/v1alpha1
kind: CNBImage
metadata:
  name: "%s"
spec:
  image: %s
  builderRef: "build-service-builder"
  gitUrl: "https://github.com/habitat-sh/sample-node-app"
  gitRevision: "master"`
	builderYaml = `---
apiVersion: build.pivotal.io/v1alpha1
kind: CNBBuilder
metadata:
  name: "build-service-builder"
spec:
  image: %s`
)

func TestExecuteBuild(t *testing.T) {
	spec.Run(t, "ExecuteBuild", testExecuteBuild, spec.Sequential())
}

func testExecuteBuild(t *testing.T, when spec.G, it spec.S) {
	var cfg config
	it.Before(func() {
		cfg = loadConfig(t)
		updateBuilderImageWithTag(t, cfg.builder, "bionic")
	})

	when("all is good", func() {
		it("creates new image", func() {
			imageConfig, err := ioutil.TempFile("", "image.yml")
			require.NoError(t, err)
			defer os.Remove(imageConfig.Name())

			imageName := "acceptance-test-" + randString(5)
			_, err = imageConfig.WriteString(fmt.Sprintf(imageYmlFormat, imageName, cfg.imageTag))
			require.NoError(t, err)
			builderConfig, err := ioutil.TempFile("", "builder.yml")
			defer os.Remove(builderConfig.Name())
			require.NoError(t, err)

			_, err = builderConfig.WriteString(fmt.Sprintf(builderYaml, cfg.builder))
			require.NoError(t, err)

			t.Log("Create the builder configuration")
			applyConfig(t, builderConfig.Name())
			defer deleteConfig(t, builderConfig.Name())

			t.Log("Create image that will be built")
			applyConfig(t, imageConfig.Name())
			defer deleteConfig(t, imageConfig.Name())

			t.Logf("Waiting for image '%s' to be created", cfg.imageTag)
			eventually(t, imageExists(t, cfg.imageTag), 5*time.Second, 2*time.Minute)
		})
	})
}

func applyConfig(t *testing.T, filePath string) {
	out, err := exec.Command("kubectl", "apply", "-f", filePath).CombinedOutput()
	t.Log(string(out))
	assert.NoError(t, err)
}
func deleteConfig(t *testing.T, filePath string) {
	out, err := exec.Command("kubectl", "delete", "-f", filePath).CombinedOutput()
	t.Log(string(out))
	assert.NoError(t, err)
}

func updateBuilderImageWithTag(t *testing.T, builder string, tag string) {
	remoteImage, err := imgutil.NewRemoteImage(fixtureBuilderLocation+":"+tag, authn.DefaultKeychain)
	require.NoError(t, err)

	remoteImage.Rename(builder)

	_, err = remoteImage.Save()
	require.NoError(t, err)
}

func imageExists(t *testing.T, name string) func() bool {
	return func() bool {
		_, found := imageSha(t, name)
		return found
	}
}

func imageSha(t *testing.T, name string) (string, bool) {
	remoteImage, err := imgutil.NewRemoteImage(name, authn.DefaultKeychain)
	require.NoError(t, err)

	found := remoteImage.Found()
	if !found {
		return "", found
	}

	digest, err := remoteImage.Digest()
	require.NoError(t, err)

	return digest, found
}

func eventually(t *testing.T, fun func() bool, interval time.Duration, duration time.Duration) {
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
