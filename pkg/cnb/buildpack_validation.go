package cnb

import (
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
)

var anyStackMinimumVersion = semver.MustParse("0.5")

func (bl BuildpackLayerInfo) supports(buildpackApis []string, id string, mixins []string, relaxedMixinContract bool) error {
	if len(bl.Order) != 0 {
		return nil //ignore meta-buildpacks
	}

	if !present(buildpackApis, bl.API) {
		return errors.Errorf("unsupported buildpack api: %s, expecting: %s", bl.API, strings.Join(buildpackApis, ", "))
	}

	for _, s := range bl.Stacks {
		buildpackVersion, err := semver.NewVersion(bl.API)
		if err != nil {
			return err
		}

		if s.ID == id || isAnystack(s.ID, buildpackVersion) {
			return validateRequiredMixins(mixins, s.Mixins, relaxedMixinContract)
		}
	}
	return errors.Errorf("stack %s is not supported", id)
}

func validateRequiredMixins(providedMixins, requiredMixins []string, relaxedMixinContract bool) error {
	var missing []string
	for _, rm := range requiredMixins {
		if !mixinPresent(providedMixins, rm, relaxedMixinContract) {
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

func mixinPresent(mixins []string, mixin string, relaxedMixinContract bool) bool {
	if !relaxedMixinContract {
		return present(mixins, mixin)
	}

	// A buildpack's mixin requirements must be satisfied by the stack in one of the following scenarios.
	// 1) the stack provides the mixin `run:<mixin>` and the buildpack requires `run:<mixin>`
	// 2) the stack provides the mixin `build:<mixin>` and the buildpack requires `build:<mixin>`
	// 3) the stack provides the mixin `<mixin>` and the buildpack requires `<mixin>`
	// 4) the stack provides the mixin `<mixin>` and the buildpack requires `build:<mixin>`
	// 5) the stack provides the mixin `<mixin>` and the buildpack requires `run:<mixin>`
	// 6) the stack provides the mixin `<mixin>` and the buildpack requires both `run:<mixin>` and `build:<mixin>`
	// 7) the stack provides the mixins `build:<mixin>` and `run:<mixin>` the buildpack requires `<mixin>`
	if strings.HasPrefix(mixin, "build:") || strings.HasPrefix(mixin, "run:") {
		return present(mixins, mixin) || present(mixins, stageRemoved(mixin))
	}

	return present(mixins, mixin) ||
		(present(mixins, "build:"+mixin) && present(mixins, "run:"+mixin))
}

func stageRemoved(needle string) string {
	return strings.SplitN(needle, ":", 2)[1]
}
func isAnystack(stackId string, buildpackVersion *semver.Version) bool {
	return stackId == "*" && buildpackVersion.Compare(anyStackMinimumVersion) >= 0
}
