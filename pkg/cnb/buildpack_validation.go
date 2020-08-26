package cnb

import (
	"strings"

	"github.com/pkg/errors"
)

func (bl BuildpackLayerInfo) supports(buildpackApis []string, id string, mixins []string) error {
	if len(bl.Order) != 0 {
		return nil //ignore meta-buildpacks
	}

	if !present(buildpackApis, bl.API) {
		return errors.Errorf("unsupported buildpack api: %s, expecting: %s", bl.API, strings.Join(buildpackApis, ", "))
	}

	for _, s := range bl.Stacks {
		if s.ID == id {
			return validateRequiredMixins(mixins, s.Mixins)
		}
	}
	return errors.Errorf("stack %s is not supported", id)
}

func validateRequiredMixins(providedMixins, requiredMixins []string) error {
	var missing []string
	for _, rm := range requiredMixins {
		if !present(providedMixins, rm) {
			missing = append(missing, rm)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return errors.Errorf("stack missing mixin(s): %s", strings.Join(missing, ", "))
}

func present(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
