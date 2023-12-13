package slsa

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsav1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	buildv1alpha2 "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/config"
)

type BuilderID string

const (
	SignedBuildID   BuilderID = "https://kpack.io/slsa/signed-build"
	UnsignedBuildID BuilderID = "https://kpack.io/slsa/unsigned-build"
)

type LifecycleProvider interface {
	Metadata() (cnb.LifecycleMetadata, error)
}

type ImageReader interface {
	Read(keychain authn.Keychain, repoName string) (string, string, map[string]string, error)
}

type Attester struct {
	Version string

	ImageReader       ImageReader
	LifecycleProvider LifecycleProvider

	Images   config.Images
	Features config.FeatureFlags
	Config   config.Config
}

func (a *Attester) GenerateStatement(build *buildv1alpha2.Build, buildMetadata *cnb.BuildMetadata, pod *corev1.Pod, builderAndAppKeychain authn.Keychain, builderId BuilderID, depFns ...BuilderDependencyFn) (intoto.Statement, error) {
	builderRepo, builderSha, builderLabels, err := a.ImageReader.Read(builderAndAppKeychain, build.Spec.Builder.Image)
	if err != nil {
		return intoto.Statement{}, fmt.Errorf("reading builder image: %v", err)
	}

	appRepo, appSha, appLabels, err := a.ImageReader.Read(builderAndAppKeychain, buildMetadata.LatestImage)
	if err != nil {
		return intoto.Statement{}, fmt.Errorf("reading app image: %v", err)
	}

	source, sourceDigest, err := extractSourceFromLabel(appLabels)
	if err != nil {
		return intoto.Statement{}, fmt.Errorf("extracting source from label: %v", err)
	}

	start, stop, err := getStartStopTime(pod)
	if err != nil {
		return intoto.Statement{}, fmt.Errorf("parsing start/stop time: %v", err)
	}

	lifecycle, err := a.LifecycleProvider.Metadata()
	if err != nil {
		return intoto.Statement{}, fmt.Errorf("reading lifecycle metadata: %v", err)
	}

	builderDeps := make([]slsav1.ResourceDescriptor, 0)
	for i, fn := range depFns {
		dep, err := fn()
		if err != nil {
			return intoto.Statement{}, fmt.Errorf("fetching builder dependency #%v: %v", i, err)
		}

		builderDeps = append(builderDeps, dep)
	}

	pred := slsav1.ProvenancePredicate{
		BuildDefinition: slsav1.ProvenanceBuildDefinition{
			BuildType:          getBuildType(a.Version),
			ExternalParameters: build.Spec,
			InternalParameters: a.internalParamsFor(build),
			ResolvedDependencies: []slsav1.ResourceDescriptor{
				{
					Name:   "source",
					URI:    source,
					Digest: sourceDigest,
				},
				{
					Name: "builder-image",
					URI:  builderRepo,
					Digest: slsacommon.DigestSet{
						"sha256": builderSha,
					},
					Annotations: convertMap(builderLabels),
				},
			},
		},
		RunDetails: slsav1.ProvenanceRunDetails{
			Builder: slsav1.Builder{
				ID: string(builderId),
				Version: map[string]string{
					"kpack":     a.Version,
					"lifecycle": lifecycle.Version,
				},
				BuilderDependencies: builderDeps,
			},
			BuildMetadata: slsav1.BuildMetadata{
				InvocationID: getInvocationId(build, pod),
				StartedOn:    start,
				FinishedOn:   stop,
			},
			Byproducts: []slsav1.ResourceDescriptor{},
		},
	}

	return intoto.Statement{
		StatementHeader: intoto.StatementHeader{
			Type:          intoto.StatementInTotoV01,
			PredicateType: slsav1.PredicateSLSAProvenance,
			Subject: []intoto.Subject{
				{
					Name: appRepo,
					Digest: slsacommon.DigestSet{
						"sha256": appSha,
					},
				},
			},
		},
		Predicate: pred,
	}, nil
}

type internalParams struct {
	BuilderImage string `json:"builderImage"`

	config.Config
	config.Images
	config.FeatureFlags
}

func (a *Attester) internalParamsFor(build *buildv1alpha2.Build) internalParams {
	return internalParams{
		BuilderImage: build.Spec.Builder.Image,

		Config:       a.Config,
		FeatureFlags: a.Features,
		Images:       a.Images,
	}
}

func getInvocationId(build *buildv1alpha2.Build, pod *corev1.Pod) string {
	return fmt.Sprintf("https://kpack.io/%v/%v/%v@%v", build.Namespace, build.Name, pod.Name, pod.Spec.NodeName)
}

func getBuildType(version string) string {
	return fmt.Sprintf("https://github.com/buildpacks-community/kpack/blob/v%v/docs/slsa.md", version)
}

func getStartStopTime(pod *corev1.Pod) (*time.Time, *time.Time, error) {
	var (
		start *time.Time
		stop  *time.Time
	)

	for _, c := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if c.Name == buildv1alpha2.PrepareContainerName {
			if c.State.Terminated != nil && c.State.Terminated.ExitCode == 0 {
				start = &c.State.Terminated.StartedAt.Time
			} else {
				return nil, nil, fmt.Errorf("prepare not finished yet")
			}
		}

		if c.Name == buildv1alpha2.CompletionContainerName {
			if c.State.Terminated != nil && c.State.Terminated.ExitCode == 0 {
				stop = &c.State.Terminated.FinishedAt.Time
			} else {
				return nil, nil, fmt.Errorf("completion not finished yet")
			}
		}
	}

	if start == nil || stop == nil {
		return nil, nil, fmt.Errorf("failed to extract time")
	}

	return start, stop, nil
}

func convertMap(orig map[string]string) map[string]interface{} {
	res := make(map[string]interface{})
	for k, v := range orig {
		res[k] = v
	}
	return res
}

type BuilderDependencyFn func() (slsav1.ResourceDescriptor, error)

type versionedObject struct {
	Name            string `json:"name"`
	ResourceVersion string `json:"resourceVersion"`
}

type K8sObject interface {
	GetObjectKind() schema.ObjectKind
	GetName() string
	GetResourceVersion() string
}

// WithVersionedObject converts a kubernetes object to a SLSA ResourceDescriptor, where the name is
// the Kind, and the content is the json serialzed Name and ResourceVersion of the object.
func WithVersionedObject(obj K8sObject) BuilderDependencyFn {
	return func() (slsav1.ResourceDescriptor, error) {
		versioned := versionedObject{
			Name:            obj.GetName(),
			ResourceVersion: obj.GetResourceVersion(),
		}
		bytes, err := json.Marshal(versioned)
		if err != nil {
			return slsav1.ResourceDescriptor{}, fmt.Errorf("marshalling json: %v", err)
		}

		return slsav1.ResourceDescriptor{
			Name:    obj.GetObjectKind().GroupVersionKind().Kind,
			Content: bytes,
		}, nil
	}
}

// WithVersionedObjects is the same as WithVersionedObject but handles a slice of objects. These
// objects must have the same GVK
func WithVersionedObjects(objs []K8sObject) BuilderDependencyFn {
	return func() (slsav1.ResourceDescriptor, error) {
		kind := ""
		versioned := make([]versionedObject, len(objs))
		for i, obj := range objs {
			if kind == "" {
				kind = obj.GetObjectKind().GroupVersionKind().Kind
			} else if kind != obj.GetObjectKind().GroupVersionKind().Kind {
				return slsav1.ResourceDescriptor{}, fmt.Errorf("objects have different kinds")
			}

			versioned[i] = versionedObject{
				Name:            obj.GetName(),
				ResourceVersion: obj.GetResourceVersion(),
			}
		}
		bytes, err := json.Marshal(versioned)
		if err != nil {
			return slsav1.ResourceDescriptor{}, fmt.Errorf("marshalling json: %v", err)
		}

		return slsav1.ResourceDescriptor{
			Name:    kind,
			Content: bytes,
		}, nil
	}
}
