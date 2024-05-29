package cnb

import (
	"github.com/google/go-containerregistry/pkg/authn"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const lifecycleMetadataLabel = "io.buildpacks.lifecycle.metadata"

type RemoteLifecycleReader struct {
	RegistryClient RegistryClient
}

func (r *RemoteLifecycleReader) Read(keychain authn.Keychain, clusterLifecycleSpec buildapi.ClusterLifecycleSpec) (buildapi.ResolvedClusterLifecycle, error) {
	lifecycleImg, lifecycleIdentifier, err := r.RegistryClient.Fetch(keychain, clusterLifecycleSpec.Image)
	if err != nil {
		return buildapi.ResolvedClusterLifecycle{}, err
	}

	lifecycleMd := LifecycleMetadata{}
	err = imagehelpers.GetLabel(lifecycleImg, lifecycleMetadataLabel, &lifecycleMd)
	if err != nil {
		return buildapi.ResolvedClusterLifecycle{}, err
	}

	return buildapi.ResolvedClusterLifecycle{
		Id:      lifecycleIdentifier,
		Version: lifecycleMd.LifecycleInfo.Version,
		API: buildapi.LifecycleAPI{
			BuildpackVersion: lifecycleMd.API.BuildpackVersion,
			PlatformVersion:  lifecycleMd.API.PlatformVersion,
		},
		APIs: buildapi.LifecycleAPIs{
			Buildpack: buildapi.APIVersions{
				Deprecated: toBuildAPISet(lifecycleMd.APIs.Buildpack.Deprecated),
				Supported:  toBuildAPISet(lifecycleMd.APIs.Buildpack.Supported),
			},
			Platform: buildapi.APIVersions{
				Deprecated: toBuildAPISet(lifecycleMd.APIs.Platform.Deprecated),
				Supported:  toBuildAPISet(lifecycleMd.APIs.Platform.Supported),
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
