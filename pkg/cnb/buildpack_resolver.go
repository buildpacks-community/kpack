package cnb

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
)

// BuildpackResolver will attempt to resolve a Buildpack reference to a
// Buildpack from either the ClusterStore, Buildpacks, or ClusterBuildpacks
type BuildpackResolver interface {
	resolveBuildpack(ref v1alpha2.BuilderBuildpackRef) (K8sRemoteBuildpack, error)
	resolveExtension(ref v1alpha2.BuilderBuildpackRef) (K8sRemoteBuildpack, error)
	ClusterStoreObservedGeneration() int64
}

type buildpackResolver struct {
	clusterStore      *v1alpha2.ClusterStore
	buildpacks        []ModuleResource
	clusterBuildpacks []ModuleResource
	extensions        []ModuleResource
	clusterExtensions []ModuleResource
}

func NewBuildpackResolver(
	clusterStore *v1alpha2.ClusterStore,
	buildpacks []ModuleResource,
	clusterBuildpacks []ModuleResource,
	extensions []ModuleResource,
	clusterExtensions []ModuleResource,
) BuildpackResolver {
	return &buildpackResolver{
		clusterStore:      clusterStore,
		buildpacks:        buildpacks,
		clusterBuildpacks: clusterBuildpacks,
		extensions:        extensions,
		clusterExtensions: clusterExtensions,
	}
}

func (r *buildpackResolver) ClusterStoreObservedGeneration() int64 {
	if r.clusterStore != nil {
		return r.clusterStore.Status.ObservedGeneration
	}
	return 0
}

func (r *buildpackResolver) resolveBuildpack(ref v1alpha2.BuilderBuildpackRef) (K8sRemoteBuildpack, error) {
	resolveFromBuildpacks := func(id string) ([]K8sRemoteBuildpack, error) {
		return resolveFromID(id, r.buildpacks)
	}
	resolveFromClusterBuildpacks := func(id string) ([]K8sRemoteBuildpack, error) {
		return resolveFromID(id, r.clusterBuildpacks)
	}
	resolveFromStore := func(id string) ([]K8sRemoteBuildpack, error) {
		return resolveFromClusterStore(ref.Id, r.clusterStore)
	}
	return r.resolveModule(
		ref,
		"buildpack",
		[]string{v1alpha2.BuildpackKind, v1alpha2.ClusterBuildpackKind},
		resolveFromBuildpacks, resolveFromClusterBuildpacks, resolveFromStore,
	)
}

type resolveByID func(string) ([]K8sRemoteBuildpack, error)

func (r *buildpackResolver) resolveExtension(ref v1alpha2.BuilderBuildpackRef) (K8sRemoteBuildpack, error) {
	resolveFromExtensions := func(id string) ([]K8sRemoteBuildpack, error) {
		return resolveFromID(id, r.extensions)
	}
	resolveFromClusterExtensions := func(id string) ([]K8sRemoteBuildpack, error) {
		return resolveFromID(id, r.clusterExtensions)
	}
	return r.resolveModule(
		ref,
		"extension",
		[]string{v1alpha2.ExtensionKind, v1alpha2.ClusterExtensionKind},
		resolveFromExtensions, resolveFromClusterExtensions,
	)
}

func (r *buildpackResolver) resolveModule(
	ref v1alpha2.BuilderBuildpackRef,
	moduleName string,
	allowedKinds []string,
	resolveFuncs ...resolveByID,
) (K8sRemoteBuildpack, error) {
	var (
		matching []K8sRemoteBuildpack
		err      error
	)
	var searchCollection []ModuleResource
	switch ref.Kind {
	case v1alpha2.BuildpackKind:
		searchCollection = r.buildpacks
	case v1alpha2.ClusterBuildpackKind:
		searchCollection = r.clusterBuildpacks
	case v1alpha2.ExtensionKind:
		searchCollection = r.extensions
	case v1alpha2.ClusterExtensionKind:
		searchCollection = r.clusterExtensions
	}
	var foundByKindAndID bool
	for _, kind := range allowedKinds {
		if ref.Kind == kind && ref.Id != "" {
			foundByKindAndID = true
			found := findByName(ref.ObjectReference, searchCollection)
			if found == nil {
				return K8sRemoteBuildpack{}, fmt.Errorf("%s not found: %v", kind, ref.Name)
			}
			matching, err = resolveFromID(ref.Id, []ModuleResource{found})
			if err != nil {
				return K8sRemoteBuildpack{}, err
			}
		}
	}
	if !foundByKindAndID {
		switch {
		case ref.Kind != "":
			if searchCollection == nil {
				return K8sRemoteBuildpack{}, fmt.Errorf("kind must be one of: %s", strings.Join(allowedKinds, ", "))
			}
			found, err := r.resolveFromObjectRef(ref.ObjectReference, searchCollection)
			if err != nil {
				return K8sRemoteBuildpack{}, err
			}
			matching = []K8sRemoteBuildpack{found}
		case ref.Id != "":
			for _, resolveFunc := range resolveFuncs {
				found, err := resolveFunc(ref.Id)
				if err != nil {
					return K8sRemoteBuildpack{}, err
				}
				matching = append(matching, found...)
			}
		case ref.Image != "":
			// TODO: add test
			return K8sRemoteBuildpack{}, fmt.Errorf("using images in builders not currently supported")
		default:
			return K8sRemoteBuildpack{}, fmt.Errorf("invalid reference")
		}
	}

	if len(matching) == 0 {
		return K8sRemoteBuildpack{}, errors.Errorf("could not find %s with id '%s'", moduleName, ref.Id)
	}
	if ref.Version == "" {
		resolved, err := highestVersion(matching)
		if err != nil {
			return K8sRemoteBuildpack{}, err
		}
		return resolved, nil
	}
	for _, result := range matching {
		if result.Buildpack.Version == ref.Version {
			return result, nil
		}
	}
	return K8sRemoteBuildpack{}, errors.Errorf("could not find %s with id '%s' and version '%s'", moduleName, ref.Id, ref.Version)
}

type ModuleResource interface {
	ModulesStatus() []corev1alpha1.BuildpackStatus
	NamespacedName() types.NamespacedName
	ServiceAccountName() string
	ServiceAccountNamespace() string
	TypeMD() metav1.TypeMeta
}

func resolveFromID(id string, resources []ModuleResource) ([]K8sRemoteBuildpack, error) {
	var matching []K8sRemoteBuildpack
	for _, resource := range resources {
		for _, status := range resource.ModulesStatus() {
			if status.Id == id {
				matching = append(matching, K8sRemoteBuildpack{
					Buildpack: status,
					SecretRef: registry.SecretRef{
						ServiceAccount: resource.ServiceAccountName(),
						Namespace:      resource.ServiceAccountNamespace(),
					},
					source: v1.ObjectReference{Name: resource.NamespacedName().Name, Namespace: resource.NamespacedName().Namespace, Kind: resource.TypeMD().Kind},
				})
			}
		}
	}
	return matching, nil
}

func resolveFromClusterStore(id string, store *v1alpha2.ClusterStore) ([]K8sRemoteBuildpack, error) {
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

// resolveFromObjectRef will get the object
// and figure out the root buildpack by converting it to a buildpack dependency tree.
func (r *buildpackResolver) resolveFromObjectRef(ref v1.ObjectReference, searchCollection []ModuleResource) (K8sRemoteBuildpack, error) {
	var (
		modules   []corev1alpha1.BuildpackStatus
		objRef    v1.ObjectReference
		secretRef registry.SecretRef
	)
	found := findByName(ref, searchCollection)
	if found == nil {
		return K8sRemoteBuildpack{}, fmt.Errorf("no %s with name '%v'", ref.Kind, ref.Name)
	}
	modules = found.ModulesStatus()
	objRef = v1.ObjectReference{Name: found.NamespacedName().Name, Namespace: found.NamespacedName().Namespace, Kind: found.TypeMD().Kind}
	secretRef = registry.SecretRef{
		ServiceAccount: found.ServiceAccountName(),
		Namespace:      found.ServiceAccountNamespace(),
	}

	trees := NewTree(modules)
	if len(trees) != 1 {
		return K8sRemoteBuildpack{}, fmt.Errorf("unexpected number of root modules: %v", len(trees))
	}
	return K8sRemoteBuildpack{
		Buildpack: *trees[0].Buildpack,
		SecretRef: secretRef,
		source:    objRef,
	}, nil
}

func findByName(ref v1.ObjectReference, resources []ModuleResource) ModuleResource {
	for _, resource := range resources {
		if resource.NamespacedName().Name == ref.Name {
			return resource
		}
	}
	return nil
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
