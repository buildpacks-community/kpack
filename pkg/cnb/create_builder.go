package cnb

import (
	"context"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/sigstore/cosign/cmd/cosign/cli/sign"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"

	cosign "github.com/pivotal/kpack/pkg/cosign"

	corev1 "k8s.io/api/core/v1"
)

type RegistryClient interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
	Save(keychain authn.Keychain, tag string, image v1.Image) (string, error)
}

type BuildpackRepository interface {
	FindByIdAndVersion(id, version string) (RemoteBuildpackInfo, error)
}

type LifecycleProvider interface {
	LayerForOS(os string) (v1.Layer, LifecycleMetadata, error)
}

type NewBuildpackRepository func(clusterStore *buildapi.ClusterStore) BuildpackRepository

type RemoteBuilderCreator struct {
	RegistryClient         RegistryClient
	NewBuildpackRepository NewBuildpackRepository
	LifecycleProvider      LifecycleProvider
	KpackVersion           string
	SigningSecrets         []corev1.Secret
}

var (
	cosignSigner = cosign.NewImageSigner(log.New(os.Stdout, "", 0), sign.SignCmd)
)

func (r *RemoteBuilderCreator) CreateBuilder(keychain authn.Keychain, clusterStore *buildapi.ClusterStore, clusterStack *buildapi.ClusterStack, spec buildapi.BuilderSpec) (buildapi.BuilderRecord, error) {
	buildpackRepo := r.NewBuildpackRepository(clusterStore)

	buildImage, _, err := r.RegistryClient.Fetch(keychain, clusterStack.Status.BuildImage.LatestImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	builderBldr := newBuilderBldr(r.KpackVersion)

	err = builderBldr.AddStack(buildImage, clusterStack)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	lifecycleLayer, lifecycleMetadata, err := r.LifecycleProvider.LayerForOS(builderBldr.os)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	builderBldr.AddLifecycle(lifecycleLayer, lifecycleMetadata)

	for _, group := range spec.Order {
		buildpacks := make([]RemoteBuildpackRef, 0, len(group.Group))

		for _, buildpack := range group.Group {
			remoteBuildpack, err := buildpackRepo.FindByIdAndVersion(buildpack.Id, buildpack.Version)
			if err != nil {
				return buildapi.BuilderRecord{}, err
			}

			buildpacks = append(buildpacks, remoteBuildpack.Optional(buildpack.Optional))
		}
		builderBldr.AddGroup(buildpacks...)
	}

	writeableImage, err := builderBldr.WriteableImage()
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	identifier, err := r.RegistryClient.Save(keychain, spec.Tag, writeableImage)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	signaturePaths, err := cosignSigner.SignBuilder(context.TODO(), identifier, r.SigningSecrets)
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	config, err := writeableImage.ConfigFile()
	if err != nil {
		return buildapi.BuilderRecord{}, err
	}

	return buildapi.BuilderRecord{
		Image: identifier,
		Stack: corev1alpha1.BuildStack{
			RunImage: clusterStack.Status.RunImage.LatestImage,
			ID:       clusterStack.Status.Id,
		},
		Buildpacks:              buildpackMetadata(builderBldr.buildpacks()),
		Order:                   builderBldr.order,
		ObservedStackGeneration: clusterStack.Status.ObservedGeneration,
		ObservedStoreGeneration: clusterStore.Status.ObservedGeneration,
		OS:                      config.OS,
		SignaturePaths:          signaturePaths,
		Signed:                  len(signaturePaths) > 0, // if signing fails, then creating the builder also fails
	}, nil
}

func (r *RemoteBuilderCreator) WithSecrets(secrets []corev1.Secret) {
	r.SigningSecrets = secrets
}

func buildpackMetadata(buildpacks []DescriptiveBuildpackInfo) corev1alpha1.BuildpackMetadataList {
	m := make(corev1alpha1.BuildpackMetadataList, 0, len(buildpacks))
	for _, b := range buildpacks {
		m = append(m, corev1alpha1.BuildpackMetadata{
			Id:       b.Id,
			Version:  b.Version,
			Homepage: b.Homepage,
		})
	}
	return m
}
