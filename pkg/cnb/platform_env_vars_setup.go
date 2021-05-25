package cnb

import (
	"encoding/json"
)

func SetupPlatformEnvVars(dir, envVarsJSON string) error {
	var envVars []envVariable
	err := json.Unmarshal([]byte(envVarsJSON), &envVars)
	if err != nil {
		return err
	}
	return serializeEnvVars(envVars, dir)
}
