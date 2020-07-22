package s3_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/pivotal/kpack/pkg/s3"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCredentialsSecret(t *testing.T) {
	spec.Run(t, "Parse S3 Credentials from secret", testParseCredentialsSecret)
}

func testParseCredentialsSecret(t *testing.T, when spec.G, it spec.S) {
	when("#parseCredentialsSecret", func() {
		it("reads S3 credentials from secret and returns them", func() {
			testDir, err := ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)

			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			require.NoError(t, os.MkdirAll(path.Join(testDir, "creds"), 0777))

			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", "accesskey"), []byte("accesskey-value"), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", "secretkey"), []byte("secretkey-value"), 0600))

			credentials, err := s3.ParseMountedCredentialsSecret(testDir, "creds")
			require.NoError(t, err)

			assert.Equal(t, credentials, s3.Credentials{
				AccessKey: "accesskey-value",
				SecretKey: "secretkey-value",
			})
		})

		it("reads invalid S3 credentials from secret", func() {
			testDir, err := ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)

			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			require.NoError(t, os.MkdirAll(path.Join(testDir, "creds"), 0777))

			_, err = s3.ParseMountedCredentialsSecret(testDir, "secretCreds")
			require.EqualError(t, err, fmt.Sprintf("Error reading secret %s at %s", "secretCreds", testDir))
		})

		it("reads empty accesskey from secret", func() {
			testDir, err := ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)

			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			require.NoError(t, os.MkdirAll(path.Join(testDir, "creds"), 0777))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", "accesskey"), []byte(""), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", "secretkey"), []byte("secretkey-value"), 0600))

			_, err = s3.ParseMountedCredentialsSecret(testDir, "creds")
			require.EqualError(t, err, "accesskey is empty")
		})

		it("reads empty secretkey from secret", func() {
			testDir, err := ioutil.TempDir("", "secret-volume")
			require.NoError(t, err)

			defer func() {
				require.NoError(t, os.RemoveAll(testDir))
			}()

			require.NoError(t, os.MkdirAll(path.Join(testDir, "creds"), 0777))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", "accesskey"), []byte("accesskey-value"), 0600))
			require.NoError(t, ioutil.WriteFile(path.Join(testDir, "creds", "secretkey"), []byte(""), 0600))

			_, err = s3.ParseMountedCredentialsSecret(testDir, "creds")
			require.EqualError(t, err, "secretkey is empty")
		})
	})
}
