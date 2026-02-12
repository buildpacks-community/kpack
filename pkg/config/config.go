package config

import "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"

type Config struct {
	SystemNamespace      string `json:"systemNamespace"`
	SystemServiceAccount string `json:"systemServiceAccount"`

	EnablePriorityClasses     bool   `json:"enablePriorityClasses"`
	MaximumPlatformApiVersion string `json:"maximumPlatformApiVersion"`
	SshTrustUnknownHosts      bool   `json:"sshTrustUnknownHosts"`
	ScalingFactor             int    `json:"scalingFactor"`
}

type FeatureFlags struct {
	InjectedSidecarSupport     bool `json:"injectedSidecarSupport"`
	GenerateSlsaAttestation    bool `json:"generateSlsaAttestation"`
	GitResolverUseShallowClone bool `json:"gitResolverUseShallowClone"`
}

type Images struct {
	BuildInitImage   string `json:"buildInitImage"`
	BuildWaiterImage string `json:"buildWaiterImage"`
	CompletionImage  string `json:"completionImage"`
	RebaseImage      string `json:"rebaseImage"`
}

// TODO: evaluate if we can move the lifecycle_provider stuff out of this config package
// Ideally v1alpha2.BuildPodImages should either just use config.Images directly or be an alias to it. However this
// doesn't work right now because lifecycle_provider.go imports pkg/cnb which imports pkg/apis/build/v1alpha2 and
// thus creating an import cycle.
func (i *Images) ToBuildPodImages() v1alpha2.BuildPodImages {
	return v1alpha2.BuildPodImages{
		BuildInitImage:   i.BuildInitImage,
		BuildWaiterImage: i.BuildWaiterImage,
		CompletionImage:  i.CompletionImage,
		RebaseImage:      i.RebaseImage,
	}
}
