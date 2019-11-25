package registry

import (
	"encoding/json"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
)

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

func SetStringLabel(image v1.Image, labels map[string]string) (v1.Image, error) {
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

func GetLabel(image v1.Image, key string, value interface{}) error {
	stringValue, err := GetStringLabel(image, key)
	if err != nil {
		return err
	}

	return json.Unmarshal([]byte(stringValue), value)
}

func GetStringLabel(image v1.Image, key string) (string, error) {
	configFile, err := image.ConfigFile()
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
