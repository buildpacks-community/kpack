package cnb

import (
	"os"
	"strings"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func SetupPlatformEnvVars(dir string) error {
	var envVars []envVariable

	for _, envVar := range os.Environ() {
		eqPos := strings.Index(envVar, "=")
		if eqPos < 0 {
			continue
		}

		key := envVar[:eqPos]
		if !strings.HasPrefix(key, v1alpha2.PlatformEnvVarPrefix) {
			continue
		}

		val := envVar[eqPos+1:]
		envVars = append(envVars, envVariable{
			Name:  key[len(v1alpha2.PlatformEnvVarPrefix):],
			Value: val,
		})
	}

	return serializeEnvVars(envVars, dir)
}
