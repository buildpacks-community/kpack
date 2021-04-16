package cnb

import (
	"strconv"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	MixinsLabel = "io.buildpacks.stack.mixins"
	StackLabel  = "io.buildpacks.stack.id"

	cnbUserId  = "CNB_USER_ID"
	cnbGroupId = "CNB_GROUP_ID"
)

type RemoteStackReader struct {
	RegistryClient RegistryClient
}

func (r *RemoteStackReader) Read(keychain authn.Keychain, clusterStackSpec buildapi.ClusterStackSpec) (buildapi.ResolvedClusterStack, error) {
	buildImage, buildIdentifier, err := r.RegistryClient.Fetch(keychain, clusterStackSpec.BuildImage.Image)
	if err != nil {
		return buildapi.ResolvedClusterStack{}, err
	}

	runImage, runIdentifier, err := r.RegistryClient.Fetch(keychain, clusterStackSpec.RunImage.Image)
	if err != nil {
		return buildapi.ResolvedClusterStack{}, err
	}

	err = validateStackId(clusterStackSpec.Id, buildImage, runImage)
	if err != nil {
		return buildapi.ResolvedClusterStack{}, err
	}

	userId, err := parseCNBID(buildImage, cnbUserId)
	if err != nil {
		return buildapi.ResolvedClusterStack{}, errors.Wrap(err, "validating build image")
	}

	groupId, err := parseCNBID(buildImage, cnbGroupId)
	if err != nil {
		return buildapi.ResolvedClusterStack{}, errors.Wrap(err, "validating build image")
	}

	buildMixins, err := readMixins(buildImage)
	if err != nil {
		return buildapi.ResolvedClusterStack{}, err
	}

	runMixins, err := readMixins(runImage)
	if err != nil {
		return buildapi.ResolvedClusterStack{}, err
	}

	mixins, err := mixins(buildMixins, runMixins)

	return buildapi.ResolvedClusterStack{
		Id: clusterStackSpec.Id,
		BuildImage: buildapi.ClusterStackStatusImage{
			LatestImage: buildIdentifier,
			Image:       clusterStackSpec.BuildImage.Image,
		},
		RunImage: buildapi.ClusterStackStatusImage{
			LatestImage: runIdentifier,
			Image:       clusterStackSpec.RunImage.Image,
		},
		Mixins:  mixins,
		UserID:  userId,
		GroupID: groupId,
	}, err
}

func validateStackId(stackId string, buildImage ggcrv1.Image, runImage ggcrv1.Image) error {
	buildStack, err := imagehelpers.GetStringLabel(buildImage, StackLabel)
	if err != nil {
		return err
	}

	runStack, err := imagehelpers.GetStringLabel(runImage, StackLabel)
	if err != nil {
		return err
	}

	if (buildStack != stackId) || (runStack != stackId) {
		return errors.Errorf("invalid stack images. expected stack: %s, build image stack: %s, run image stack: %s", stackId, buildStack, runStack)
	}

	return nil
}

func readMixins(image ggcrv1.Image) ([]string, error) {
	var mixins []string
	hasLabel, err := imagehelpers.HasLabel(image, MixinsLabel)
	if !hasLabel || err != nil {
		return mixins, err
	}

	err = imagehelpers.GetLabel(image, MixinsLabel, &mixins)
	return mixins, err
}

func mixins(build, run []string) ([]string, error) {
	buildStage, invalid, buildCommon := classifyMixins(build, "build:", "run:")
	if len(invalid) > 0 {
		return nil, errors.Errorf("build image contains run-only mixin(s): %s", strings.Join(invalid, ", "))
	}
	runStage, invalid, runCommon := classifyMixins(run, "run:", "build:")
	if len(invalid) > 0 {
		return nil, errors.Errorf("run image contains build-only mixin(s): %s", strings.Join(invalid, ", "))
	}

	if missing := missingCommonRunMixins(buildCommon, runCommon); len(missing) != 0 {
		return nil, errors.Errorf("runImage missing required mixin(s): %s", strings.Join(missing, ", "))
	}

	return append(buildCommon, append(buildStage, runStage...)...), nil
}

func classifyMixins(mixins []string, validPrefix, invalidPrefix string) (valid []string, invalid []string, common []string) {
	for _, m := range mixins {
		switch {
		case strings.HasPrefix(m, validPrefix):
			valid = append(valid, m)
		case strings.HasPrefix(m, invalidPrefix):
			invalid = append(invalid, m)
		default:
			common = append(common, m)
		}
	}
	return
}

func missingCommonRunMixins(build []string, run []string) []string {
	var missing []string
	for _, m := range build {
		if !present(run, m) {
			missing = append(missing, m)
		}
	}
	return missing
}

func parseCNBID(image ggcrv1.Image, env string) (int, error) {
	v, err := imagehelpers.GetEnv(image, env)
	if err != nil {
		return 0, err
	}
	id, err := strconv.Atoi(v)
	return id, errors.Wrapf(err, "env %s", env)
}
