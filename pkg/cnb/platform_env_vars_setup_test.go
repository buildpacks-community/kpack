package cnb_test

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
)

func TestPlatformEnvVarsSetup(t *testing.T) {
	spec.Run(t, "PlatformEnvVarsSetup", testPlatformEnvVarsSetup)
}

func testPlatformEnvVarsSetup(t *testing.T, when spec.G, it spec.S) {
	var (
		testVolume  string
		platformEnv map[string]string
	)

	it.Before(func() {
		var err error
		testVolume, err = ioutil.TempDir("", "permission")
		require.NoError(t, err)

		platformEnv = map[string]string{
			"keyA": "valueA",
			"keyB": "valueB",
			"keyC": "foo=bar",
			"keyD": "",
		}

		for k, v := range platformEnv {
			os.Setenv(v1alpha2.PlatformEnvVarPrefix+k, v)
		}
	})

	it.After(func() {
		for k := range platformEnv {
			os.Unsetenv(v1alpha2.PlatformEnvVarPrefix + k)
		}
		os.RemoveAll(testVolume)
	})

	when("#setup", func() {
		it("writes all env var files to the platform dir", func() {
			err := cnb.SetupPlatformEnvVars(testVolume)
			require.NoError(t, err)

			for k, v := range platformEnv {
				checkEnvVar(t, testVolume, k, v)
			}
		})
	})
}

func checkEnvVar(t *testing.T, testVolume, key, value string) {
	require.FileExists(t, path.Join(testVolume, "env", key))
	buf, err := ioutil.ReadFile(path.Join(testVolume, "env", key))
	require.NoError(t, err)
	require.Equal(t, value, string(buf))
}
