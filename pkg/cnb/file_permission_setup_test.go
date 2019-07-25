package cnb_test

import (
	"fmt"
	"github.com/buildpack/imgutil/fakes"
	"github.com/pivotal/build-service-beam/pkg/cnb"
	"github.com/pivotal/build-service-beam/pkg/registry"
	"github.com/pivotal/build-service-beam/pkg/registry/registryfakes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"

	"github.com/sclevine/spec"
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
		os.RemoveAll(testVolume)
	})

	when("#setup", func() {
		it("sets the owner of all requested", func() {
			fakeImage := fakes.NewImage("some/builder", "topLayerSha", "digest")
			require.NoError(t, fakeImage.SetEnv("CNB_USER_ID", "1234"))
			require.NoError(t, fakeImage.SetEnv("CNB_GROUP_ID", "5678"))

			fakeRemoteImageFactory.NewRemoteReturns(fakeImage, nil)

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

			require.Equal(t, fakeRemoteImageFactory.NewRemoteCallCount(), 1)
			assert.Equal(t, fakeRemoteImageFactory.NewRemoteArgsForCall(0), registry.NewNoAuthImageRef("builder/builder"))

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
