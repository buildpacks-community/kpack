package blob_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pivotal/kpack/pkg/blob"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
)

func TestFileKeychain(t *testing.T) {
	spec.Run(t, "testFileKeychain", testFileKeychain)
}

func testFileKeychain(t *testing.T, when spec.G, it spec.S) {
	var (
		testVolume string
		testDir    string
		hostName   = "some-blobstore.com"
		secretName = "some-secret"
	)

	it.Before(func() {
		var err error
		testVolume, err = os.MkdirTemp("", "")
		require.NoError(t, err)

		testDir = filepath.Join(testVolume, secretName)
		require.NoError(t, os.Mkdir(testDir, 0777))
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testDir))
	})

	when("files are valid", func() {
		it("reads username/password", func() {
			os.WriteFile(filepath.Join(testDir, "username"), []byte("some-username"), 0777)
			os.WriteFile(filepath.Join(testDir, "password"), []byte("some-password"), 0777)

			keychain, err := blob.NewMountedSecretBlobKeychain(testVolume, []string{fmt.Sprintf("%v=%v", secretName, hostName)})
			require.NoError(t, err)

			auth, header, err := keychain.Resolve("https://some-blobstore.com")
			require.NoError(t, err)

			require.Equal(t, "Basic c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk", auth)
			require.Nil(t, header)
		})

		it("reads bearer token", func() {
			os.WriteFile(filepath.Join(testDir, "bearer"), []byte("some-token"), 0777)

			keychain, err := blob.NewMountedSecretBlobKeychain(testVolume, []string{fmt.Sprintf("%v=%v", secretName, hostName)})
			require.NoError(t, err)

			auth, header, err := keychain.Resolve("https://some-blobstore.com")
			require.NoError(t, err)

			require.Equal(t, "Bearer some-token", auth)
			require.Nil(t, header)
		})

		it("reads authorization", func() {
			os.WriteFile(filepath.Join(testDir, "authorization"), []byte("PRIVATE-METHOD some-auth"), 0777)

			keychain, err := blob.NewMountedSecretBlobKeychain(testVolume, []string{fmt.Sprintf("%v=%v", secretName, hostName)})
			require.NoError(t, err)

			auth, header, err := keychain.Resolve("https://some-blobstore.com")
			require.NoError(t, err)

			require.Equal(t, "PRIVATE-METHOD some-auth", auth)
			require.Nil(t, header)
		})
	})

	when("files are invalid", func() {
		it("errors if no method is found", func() {
			keychain, err := blob.NewMountedSecretBlobKeychain(testVolume, []string{fmt.Sprintf("%v=%v", secretName, hostName)})
			require.NoError(t, err)

			_, _, err = keychain.Resolve("https://some-blobstore.com")
			require.EqualError(t, err, "no auths found for 'some-secret'")
		})

		it("errors if more than one method is found", func() {
			os.WriteFile(filepath.Join(testDir, "username"), []byte("some-username"), 0777)
			os.WriteFile(filepath.Join(testDir, "password"), []byte("some-password"), 0777)
			os.WriteFile(filepath.Join(testDir, "bearer"), []byte("some-token"), 0777)

			keychain, err := blob.NewMountedSecretBlobKeychain(testVolume, []string{fmt.Sprintf("%v=%v", secretName, hostName)})
			require.NoError(t, err)

			_, _, err = keychain.Resolve("https://some-blobstore.com")
			require.EqualError(t, err, "multiple auths found for 'some-secret', only one of username/password, bearer, authorization is allowed")
		})

		it("errors if the domain doesn't match", func() {
			os.WriteFile(filepath.Join(testDir, "username"), []byte("some-username"), 0777)
			os.WriteFile(filepath.Join(testDir, "password"), []byte("some-password"), 0777)

			keychain, err := blob.NewMountedSecretBlobKeychain(testVolume, []string{fmt.Sprintf("%v=%v", secretName, hostName)})
			require.NoError(t, err)

			_, _, err = keychain.Resolve("https://some-other-domain.com")
			require.EqualError(t, err, "no secrets matched for 'some-other-domain.com'")
		})
	})
}
