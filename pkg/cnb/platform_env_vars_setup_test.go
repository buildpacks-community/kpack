package cnb_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/cnb"
)

func TestPlatformEnvVarsSetup(t *testing.T) {
	spec.Run(t, "PlatformEnvVarsSetup", testPlatformEnvVarsSetup)
}

func testPlatformEnvVarsSetup(t *testing.T, when spec.G, it spec.S) {
	var (
		testVolume string
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
		it("writes all env var files to the platform dir", func() {
			err := cnb.SetupPlatformEnvVars(testVolume, `[{"name": "keyA", "value": "valueA"}, {"name": "keyB", "value": "valueB"}, {"name": "keyC", "value": "valueC"}]`)
			require.NoError(t, err)

			checkEnvVar(t, testVolume, "keyA", "valueA")
			checkEnvVar(t, testVolume, "keyB", "valueB")
			checkEnvVar(t, testVolume, "keyC", "valueC")
		})
	})
}

func checkEnvVar(t *testing.T, testVolume, key, value string) {
	require.FileExists(t, path.Join(testVolume, "env", key))
	buf, err := ioutil.ReadFile(path.Join(testVolume, "env", key))
	require.NoError(t, err)
	require.Equal(t, value, string(buf))
}
