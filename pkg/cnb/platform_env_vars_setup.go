package cnb

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
)

func SetupPlatformEnvVars(dir, envVarsJSON string) error {
	var envVars []envVariable
	err := json.Unmarshal([]byte(envVarsJSON), &envVars)
	if err != nil {
		return err
	}
	folder := path.Join(dir, "env")
	err = os.Mkdir(folder, os.ModePerm)
	if err != nil {
		return err
	}

	for _, envVar := range envVars {
		err = ioutil.WriteFile(path.Join(folder, envVar.Name), []byte(envVar.Value), os.ModePerm)
		if err != nil {
			return err
		}
	}
	return nil
}

type envVariable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
