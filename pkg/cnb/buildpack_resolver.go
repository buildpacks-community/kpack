package cnb

import (
	"fmt"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/duckbuildpack"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
)

// BuildpackResolver will attempt to resolve a Buildpack reference to a
// Buildpack from either the ClusterStore, Buildpacks, or ClusterBuildpacks
type BuildpackResolver interface {
	resolve(ref v1alpha2.BuilderBuildpackRef) (K8sRemoteBuildpack, error)
	ClusterStoreObservedGeneration() int64
	UsedObjects() []v1.ObjectReference
}

type buildpackResolver struct {
	clusterstore *v1alpha2.ClusterStore
	buildpacks   []*duckbuildpack.DuckBuildpack
	usedObjects  []v1.ObjectReference
}

func NewBuildpackResolver(clusterStore *v1alpha2.ClusterStore, buildpacks []*duckbuildpack.DuckBuildpack) BuildpackResolver {
	return &buildpackResolver{
		clusterstore: clusterStore,
		buildpacks:   buildpacks,
		usedObjects:  make([]v1.ObjectReference, 0),
	}
}

func (r *buildpackResolver) ClusterStoreObservedGeneration() int64 {
	if r.clusterstore != nil {
		return r.clusterstore.Status.ObservedGeneration
	}
	return 0
}

func (r *buildpackResolver) UsedObjects() []v1.ObjectReference {
	return r.usedObjects
}

func (r *buildpackResolver) resolve(ref v1alpha2.BuilderBuildpackRef) (K8sRemoteBuildpack, error) {
	var matchingBuildpacks []K8sRemoteBuildpack
	var err error
	switch {
	case ref.Kind != "" && ref.Id != "":
		if ref.Kind == v1alpha2.ClusterStoreKind {
			matchingBuildpacks, err = r.resolveFromClusterStore(ref.Id, r.clusterstore)
			if err != nil {
				return K8sRemoteBuildpack{}, err
			}
		} else {
			bp := findBuildpack(ref.ObjectReference, r.buildpacks)
			if bp == nil {
				return K8sRemoteBuildpack{}, fmt.Errorf("buildpack or cluster buildpack '%v' not found", ref.Name)
			}

			matchingBuildpacks, err = r.resolveFromBuildpack(ref.Id, []*duckbuildpack.DuckBuildpack{bp})
			if err != nil {
				return K8sRemoteBuildpack{}, err
			}
		}
	case ref.Kind != "":
		bp, err := r.resolveFromBuildpackReference(ref.ObjectReference)
		if err != nil {
			return K8sRemoteBuildpack{}, err
		}

		matchingBuildpacks = []K8sRemoteBuildpack{bp}
	case ref.Id != "":
		bp, err := r.resolveFromBuildpack(ref.Id, r.buildpacks)
		if err != nil {
			return K8sRemoteBuildpack{}, err
		}
		matchingBuildpacks = append(matchingBuildpacks, bp...)

		cs, err := r.resolveFromClusterStore(ref.Id, r.clusterstore)
		if err != nil {
			return K8sRemoteBuildpack{}, err
		}
		matchingBuildpacks = append(matchingBuildpacks, cs...)
	case ref.Image != "":
		// TODO(chenbh):
		return K8sRemoteBuildpack{}, fmt.Errorf("using images in builders not currently supported")
	default:
		return K8sRemoteBuildpack{}, fmt.Errorf("invalid buildpack reference")
	}

	if len(matchingBuildpacks) == 0 {
		return K8sRemoteBuildpack{}, errors.Errorf("could not find buildpack with id '%s'", ref.Id)
	}

	if ref.Version == "" {
		bp, err := highestVersion(matchingBuildpacks)
		if err != nil {
			return K8sRemoteBuildpack{}, err
		}
		r.usedObjects = append(r.usedObjects, bp.source)
		return bp, nil
	}

	for _, result := range matchingBuildpacks {
		if result.Buildpack.Version == ref.Version {
			r.usedObjects = append(r.usedObjects, result.source)
			return result, nil
		}
	}

	return K8sRemoteBuildpack{}, errors.Errorf("could not find buildpack with id '%s' and version '%s'", ref.Id, ref.Version)
}

func (r *buildpackResolver) resolveFromBuildpack(id string, buildpacks []*duckbuildpack.DuckBuildpack) ([]K8sRemoteBuildpack, error) {
	var matchingBuildpacks []K8sRemoteBuildpack
	for _, bp := range buildpacks {
		for _, status := range bp.Status.Buildpacks {
			if status.Id == id {
				secretRef := registry.SecretRef{}

				if bp.Spec.ServiceAccountRef != nil {
					secretRef = registry.SecretRef{
						ServiceAccount: bp.Spec.ServiceAccountRef.Name,
						Namespace:      bp.Spec.ServiceAccountRef.Namespace,
					}
				}
				matchingBuildpacks = append(matchingBuildpacks, K8sRemoteBuildpack{
					Buildpack: status,
					SecretRef: secretRef,
					source:    v1.ObjectReference{Name: bp.Name, Namespace: bp.Namespace, Kind: bp.Kind},
				})
			}
		}
	}
	return matchingBuildpacks, nil
}

func (r *buildpackResolver) resolveFromClusterStore(id string, store *v1alpha2.ClusterStore) ([]K8sRemoteBuildpack, error) {
	if store == nil {
		return nil, nil
	}

	var matchingBuildpacks []K8sRemoteBuildpack
	for _, status := range store.Status.Buildpacks {
		if status.Id == id {
			secretRef := registry.SecretRef{}

			if store.Spec.ServiceAccountRef != nil {
				secretRef = registry.SecretRef{
					ServiceAccount: store.Spec.ServiceAccountRef.Name,
					Namespace:      store.Spec.ServiceAccountRef.Namespace,
				}
			}
			matchingBuildpacks = append(matchingBuildpacks, K8sRemoteBuildpack{
				Buildpack: status,
				SecretRef: secretRef,
				source:    v1.ObjectReference{Name: store.Name, Namespace: store.Namespace, Kind: store.Kind},
			})
		}
	}
	return matchingBuildpacks, nil
}

// resolveFromBuildpackReference will get the object and figure out the root
// buildpack by converting it to a buildpack dependency tree
func (r *buildpackResolver) resolveFromBuildpackReference(ref v1.ObjectReference) (K8sRemoteBuildpack, error) {
	var (
		bps       []corev1alpha1.BuildpackStatus
		secretRef registry.SecretRef
		objRef    v1.ObjectReference
	)
	if ref.Kind != v1alpha2.BuildpackKind && ref.Kind != v1alpha2.ClusterBuildpackKind {
		return K8sRemoteBuildpack{}, fmt.Errorf("kind must be either %v or %v", v1alpha2.BuildpackKind, v1alpha2.ClusterBuildpackKind)
	}

	bp := findBuildpack(ref, r.buildpacks)
	if bp == nil {
		return K8sRemoteBuildpack{}, fmt.Errorf("no buildpack or cluster buildpack with name '%v'", ref.Name)
	}

	bps = bp.Status.Buildpacks
	objRef = v1.ObjectReference{Name: bp.Name, Namespace: bp.Namespace, Kind: bp.Kind}
	if bp.Spec.ServiceAccountRef != nil {
		secretRef = registry.SecretRef{
			ServiceAccount: bp.Spec.ServiceAccountRef.Name,
			Namespace:      bp.Spec.ServiceAccountRef.Namespace,
		}
	}

	trees := NewTree(bps)
	if len(trees) != 1 {
		return K8sRemoteBuildpack{}, fmt.Errorf("unexpected number of root buildpacks: %v", len(trees))
	}

	return K8sRemoteBuildpack{
		Buildpack: *trees[0].Buildpack,
		SecretRef: secretRef,
		source:    objRef,
	}, nil
}

func findBuildpack(ref v1.ObjectReference, buildpacks []*duckbuildpack.DuckBuildpack) *duckbuildpack.DuckBuildpack {
	for _, bp := range buildpacks {
		if bp.Name == ref.Name && bp.Kind == ref.Kind {
			return bp
		}
	}
	return nil
}

func getSecretRef(bp duckbuildpack.DuckBuildpack) registry.SecretRef {
	if bp.Spec.ServiceAccountRef != nil {
		return registry.SecretRef{
			ServiceAccount: bp.Spec.ServiceAccountRef.Name,
			Namespace:      bp.Spec.ServiceAccountRef.Namespace,
		}
	}
	return registry.SecretRef{}
}

// TODO: error if the highest version has multiple diff ids
func highestVersion(matchingBuildpacks []K8sRemoteBuildpack) (K8sRemoteBuildpack, error) {
	for _, bp := range matchingBuildpacks {
		if _, err := semver.NewVersion(bp.Buildpack.Version); err != nil {
			return K8sRemoteBuildpack{}, errors.Errorf("cannot find buildpack '%s' with latest version due to invalid semver '%s'", bp.Buildpack.Id, bp.Buildpack.Version)
		}
	}

	sort.SliceStable(matchingBuildpacks, func(i, j int) bool {
		return semver.MustParse(matchingBuildpacks[i].Buildpack.Version).
			LessThan(semver.MustParse(matchingBuildpacks[j].Buildpack.Version))
	})
	return matchingBuildpacks[len(matchingBuildpacks)-1], nil
}
