package cnb_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestFilePermissionSetup(t *testing.T) {
	spec.Run(t, "FilePermissionSetup", testFilePermissionSetup)
}

func testFilePermissionSetup(t *testing.T, when spec.G, it spec.S) {
	var (
		fakeRemoteImageFactory = &registryfakes.FakeRemoteImageFactory{}
		testVolume             string
	)

	it.Before(func() {
		var err error
		testVolume, err = ioutil.TempDir("", "permission")
		require.NoError(t, err)
	})

	it.After(func() {
		require.NoError(t, os.RemoveAll(testVolume))
	})

	when("#setup", func() {
		it("sets the owner of all requested", func() {
			fakeImage := registryfakes.NewFakeRemoteImage("some/builder", "2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895")
			require.NoError(t, fakeImage.SetEnv("CNB_USER_ID", "1234"))
			require.NoError(t, fakeImage.SetEnv("CNB_GROUP_ID", "5678"))

			fakeRemoteImageFactory.NewRemoteWithDefaultKeychainReturns(fakeImage, nil)

			chowner := &osSpy{
				chowned: make(map[string]string),
			}

			filePermissionSetup := &cnb.FilePermissionSetup{
				RemoteImageFactory: fakeRemoteImageFactory,
				Chowner:            chowner,
			}
			err := filePermissionSetup.Setup("builder/builder", testVolume)
			require.NoError(t, err)

			require.Equal(t, chowner.chowned[testVolume], "1234:5678")

			require.Equal(t, fakeRemoteImageFactory.NewRemoteWithDefaultKeychainCallCount(), 1)
			assert.Equal(t, fakeRemoteImageFactory.NewRemoteWithDefaultKeychainArgsForCall(0), "builder/builder")

		})
	})
}

type osSpy struct {
	chowned map[string]string
}

func (c *osSpy) Chown(volume string, uid, gid int) error {
	c.chowned[volume] = fmt.Sprintf("%d:%d", uid, gid)
	return nil
}
