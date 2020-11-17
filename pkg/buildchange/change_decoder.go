package buildchange

import (
	"reflect"

	"github.com/mitchellh/mapstructure"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type ChangeDecoder struct{}

func (c ChangeDecoder) Decode(reason v1alpha1.BuildReason, value interface{}) (Change, error) {
	switch reason {
	case v1alpha1.BuildReasonConfig:
		return c.decodeConfigChange(value)

	case v1alpha1.BuildReasonCommit:
		change := CommitChange{}
		err := mapstructure.Decode(value, &change)
		return change, err

	case v1alpha1.BuildReasonBuildpack:
		change := BuildpackChange{}
		err := mapstructure.Decode(value, &change)
		return change, err

	case v1alpha1.BuildReasonStack:
		change := StackChange{}
		err := mapstructure.Decode(value, &change)
		return change, err

	case v1alpha1.BuildReasonTrigger:
		change := TriggerChange{}
		err := mapstructure.Decode(value, &change)
		return change, err
	}
	return nil, nil
}

func (c ChangeDecoder) decodeConfigChange(value interface{}) (ConfigChange, error) {
	change := ConfigChange{}
	config := &mapstructure.DecoderConfig{
		Result: &change,
		DecodeHook: func(from reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
			if from.String() == "string" &&
				to.String() == "resource.Quantity" {
				return resource.ParseQuantity(data.(string))
			}
			return data, nil
		},
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return change, err
	}
	return change, decoder.Decode(value)
}
