package cnb

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/pkg/errors"
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

type buildEnvVariable struct {
	Env []envVariable `toml:"env"`
}

type envBuildVariable struct {
	Env []envVariable
}

func (a *envBuildVariable) UnmarshalTOML(f interface{}) error {
	var (
		env []envVariable
		err error
	)
	switch v := f.(type) {
	case map[string]interface{}:
		if envs, ok := v["build"].([]map[string]interface{}); ok {
			env, err = buildEnv(envs)
			if err != nil {
				return err
			}
		}
	case []map[string]interface{}:
		env, err = buildEnv(v)
		if err != nil {
			return err
		}
	default:
		return errors.New("environment variables in project descriptor could not be parsed")
	}
	a.Env = env
	return nil
}

func buildEnv(v []map[string]interface{}) ([]envVariable, error) {
	var e []envVariable
	for _, env := range v {
		if name, nameOk := env["name"].(string); nameOk {
			if value, valueOk := env["value"].(string); valueOk {
				e = append(e, envVariable{
					Name:  name,
					Value: value,
				})
			} else {
				return nil, errors.Errorf("environment variable '%s' is not a string value", name)
			}
		} else {
			return nil, errors.New("environment variable 'name' is not a string")
		}
	}
	return e, nil
}
