package dockercreds_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/dockercreds"
)

func TestParseAnnotatedSecrets(t *testing.T) {
	spec.Run(t, "Test Parse Annotated Secrets", testParseAnnotatedSecrets)
}

func testParseAnnotatedSecrets(t *testing.T, when spec.G, it spec.S) {
	when("ParseMountedAnnotatedSecrets", func() {

		var testDir string
		it.Before(func() {
			var err error
			testDir, err = ioutil.TempDir("", "docker-secret-parse-test")
			require.NoError(t, err)

			require.NoError(t, os.MkdirAll(path.Join(testDir, "gcr-creds"), 0777))
			require.NoError(t, os.MkdirAll(path.Join(testDir, "dockerhub-creds"), 0777))

			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "gcr-creds", corev1.BasicAuthUsernameKey), []byte("gcr-username"), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "gcr-creds", corev1.BasicAuthPasswordKey), []byte("gcr-password"), 0600))

			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "dockerhub-creds", corev1.BasicAuthUsernameKey), []byte("dockerhub-username"), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "dockerhub-creds", corev1.BasicAuthPasswordKey), []byte("dockerhub-password"), 0600))

		})

		it.After(func() {
			require.NoError(t, os.RemoveAll(testDir))
		})

		when("ParseMountedAnnotatedSecrets", func() {
			it("parses the volume mounted creds", func() {

				logger := log.New(&bytes.Buffer{}, "", 0)

				creds, err := dockercreds.ParseMountedAnnotatedSecrets(testDir,
					[]string{
						"gcr-creds=gcr.io",
						"dockerhub-creds=index.docker.io",
					},
					logger,
				)
				require.NoError(t, err)

				assert.Equal(t, dockercreds.DockerCreds{
					"gcr.io": {
						Username: "gcr-username",
						Password: "gcr-password",
					},
					"index.docker.io": {
						Username: "dockerhub-username",
						Password: "dockerhub-password",
					},
				}, creds)
			})

		})

	})
}
