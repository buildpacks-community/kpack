package imagehelpers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
)

func GetCreatedAt(image v1.Image) (time.Time, error) {
	cfg, err := configFile(image)
	if err != nil {
		return time.Time{}, err
	}
	return cfg.Created.UTC(), nil
}

func GetEnv(image v1.Image, key string) (string, error) {
	cfg, err := configFile(image)
	if err != nil {
		return "", err
	}
	for _, envVar := range cfg.Config.Env {
		parts := strings.Split(envVar, "=")
		if parts[0] == key {
			return parts[1], nil
		}
	}
	return "", errors.Errorf("ENV %s not found", key)
}

func SetEnv(image v1.Image, key, value string) (v1.Image, error) {
	cfg, err := configFile(image)
	if err != nil {
		return nil, err
	}

	config := *cfg.Config.DeepCopy()

	config.Env = append(config.Env, fmt.Sprintf("%s=%s", key, value))

	return mutate.Config(image, config)
}

func HasLabel(image v1.Image, key string) (bool, error) {
	configFile, err := configFile(image)
	if err != nil {
		return false, err
	}

	_, ok := configFile.Config.Labels[key]
	return ok, err
}

func GetStringLabel(image v1.Image, key string) (string, error) {
	configFile, err := configFile(image)
	if err != nil {
		return "", err
	}

	config := configFile.Config.DeepCopy()

	stringValue, ok := config.Labels[key]
	if !ok {
		return "", errors.Errorf("could not find label %s", key)
	}

	return stringValue, nil
}

func GetLabel(image v1.Image, key string, value interface{}) error {
	stringValue, err := GetStringLabel(image, key)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(stringValue), value)
}

func SetStringLabel(image v1.Image, key, value string) (v1.Image, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	config := *configFile.Config.DeepCopy()
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}

	config.Labels[key] = value

	return mutate.Config(image, config)
}

func SetStringLabels(image v1.Image, labels map[string]string) (v1.Image, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	config := *configFile.Config.DeepCopy()
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}

	for k, v := range labels {
		config.Labels[k] = v
	}

	return mutate.Config(image, config)
}

func SetLabels(image v1.Image, labels map[string]interface{}) (v1.Image, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	config := *configFile.Config.DeepCopy()
	if config.Labels == nil {
		config.Labels = map[string]string{}
	}

	for k, v := range labels {
		dataBytes, err := json.Marshal(v)
		if err != nil {
			return nil, errors.Wrapf(err, "marshalling data to JSON for label %s", k)
		}

		config.Labels[k] = string(dataBytes)
	}

	return mutate.Config(image, config)
}

func SetWorkingDir(image v1.Image, dir string) (v1.Image, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return nil, err
	}

	config := *configFile.Config.DeepCopy()
	config.WorkingDir = dir
	return mutate.Config(image, config)
}

func GetWorkingDir(image v1.Image) (string, error) {
	configFile, err := image.ConfigFile()
	if err != nil {
		return "", err
	}

	config := *configFile.Config.DeepCopy()
	return config.WorkingDir, nil
}

func configFile(image v1.Image) (*v1.ConfigFile, error) {
	cfg, err := image.ConfigFile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get image config")
	} else if cfg == nil {
		return nil, errors.Errorf("got nil image config")
	}
	return cfg, nil
}
