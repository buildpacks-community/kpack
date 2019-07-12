package cnb

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
)

func SetupPlatformEnvVars(dir, envVarsJSON string) error {
	var envVars map[string]string
	err := json.Unmarshal([]byte(envVarsJSON), &envVars)
	if err != nil {
		return err
	}
	folder := path.Join(dir, "env")
	err = os.Mkdir(folder, os.ModePerm)
	if err != nil {
		return err
	}
	
	for key, value := range envVars {
		err = ioutil.WriteFile(path.Join(folder, key), []byte(value), os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}
