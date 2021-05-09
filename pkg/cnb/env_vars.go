package cnb

import (
	"io/ioutil"
	"os"
	"path"
)

func serializeEnvVars(envVars []envVariable, platformDir string) error {
	var err error
	folder := path.Join(platformDir, "env")
	err = os.MkdirAll(folder, os.ModePerm)
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
	Name  string `json:"name" toml:"name"`
	Value string `json:"value" toml:"value"`
}
