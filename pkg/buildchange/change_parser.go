package buildchange

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func NewChangeParser() ChangeParser {
	return ChangeParser{
		decoder: ChangeDecoder{},
	}
}

type ChangeParser struct {
	decoder ChangeDecoder
}

func (c ChangeParser) Parse(jsonStr string) (map[v1alpha1.BuildReason]Change, error) {
	reasonValueMap := map[v1alpha1.BuildReason]interface{}{}
	if err := json.Unmarshal([]byte(jsonStr), &reasonValueMap); err != nil {
		return nil, err
	}

	var reasonChangeMap = make(map[v1alpha1.BuildReason]Change, len(reasonValueMap))
	for reason, value := range reasonValueMap {
		change, err := c.parseChange(reason, value)
		if err != nil {
			return reasonChangeMap, errors.Wrapf(err, "error parsing change for reason '%s'", reason)
		}
		reasonChangeMap[reason] = change
	}
	return reasonChangeMap, nil
}

func (c ChangeParser) parseChange(reason v1alpha1.BuildReason, value interface{}) (Change, error) {
	var change Change

	if !reason.IsValid() {
		return change, errors.Errorf("unsupported reason")
	}

	change, err := c.decoder.Decode(reason, value)
	if err != nil {
		return change, errors.Wrapf(err, "error decoding change")
	}

	if !change.IsValid() {
		return nil, errors.Errorf("invalid change")
	}
	return change, err
}
