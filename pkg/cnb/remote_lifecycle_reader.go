package cnb

import (
	"github.com/google/go-containerregistry/pkg/authn"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const lifecycleBuilderMetadataLabel = "io.buildpacks.builder.metadata"

type RemoteLifecycleReader struct {
	RegistryClient RegistryClient
}

func (r *RemoteLifecycleReader) Read(keychain authn.Keychain, clusterLifecycleSpec buildapi.ClusterLifecycleSpec) (buildapi.ResolvedClusterLifecycle, error) {
	lifecycleImg, imageIdentifier, err := r.RegistryClient.Fetch(keychain, clusterLifecycleSpec.Image)
	if err != nil {
		return buildapi.ResolvedClusterLifecycle{}, err
	}

	deprecatedLifecycleMD := LifecycleDescriptor{}
	err = imagehelpers.GetLabel(lifecycleImg, lifecycleBuilderMetadataLabel, &deprecatedLifecycleMD)
	if err != nil {
		return buildapi.ResolvedClusterLifecycle{}, err
	}

	lifecycleMD := LifecycleAPIs{}
	err = imagehelpers.GetLabel(lifecycleImg, lifecycleApisLabel, &lifecycleMD)
	if err != nil {
		return buildapi.ResolvedClusterLifecycle{}, err
	}

	return buildapi.ResolvedClusterLifecycle{
		Image: buildapi.ClusterLifecycleStatusImage{
			LatestImage: imageIdentifier,
			Image:       clusterLifecycleSpec.Image,
		},
		Version: deprecatedLifecycleMD.Info.Version,
		API: buildapi.LifecycleAPI{
			BuildpackVersion: deprecatedLifecycleMD.API.BuildpackVersion,
			PlatformVersion:  deprecatedLifecycleMD.API.PlatformVersion,
		},
		APIs: buildapi.LifecycleAPIs{
			Buildpack: buildapi.APIVersions{
				Deprecated: toBuildAPISet(lifecycleMD.Buildpack.Deprecated),
				Supported:  toBuildAPISet(lifecycleMD.Buildpack.Supported),
			},
			Platform: buildapi.APIVersions{
				Deprecated: toBuildAPISet(lifecycleMD.Platform.Deprecated),
				Supported:  toBuildAPISet(lifecycleMD.Platform.Supported),
			},
		},
	}, nil
}

func toBuildAPISet(from APISet) buildapi.APISet {
	var to buildapi.APISet
	for _, f := range from {
		to = append(to, f)
	}
	return to
}
