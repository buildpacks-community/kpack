package cnb

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"

	lifecyclebuildpack "github.com/buildpacks/lifecycle/buildpack"
	"github.com/buildpacks/lifecycle/platform"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

type BuildMetadata struct {
	BuildpackMetadata corev1alpha1.BuildpackMetadataList `json:"buildpackMetadata"`
	LatestCacheImage  string                             `json:"latestCacheImage"`
	LatestImage       string                             `json:"latestImage"`
	StackID           string                             `json:"stackID"`
	StackRunImage     string                             `json:"stackRunImage"`
}

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
}

type RemoteMetadataRetriever struct {
	ImageFetcher ImageFetcher
}

func (r *RemoteMetadataRetriever) GetBuildMetadata(builtImageRef, cacheTag string, keychain authn.Keychain) (*BuildMetadata, error) {
	buildImg, err := r.getBuiltImage(builtImageRef, keychain)
	if err != nil {
		return nil, err
	}
	cacheImageRef, _ := r.getCacheImage(cacheTag, keychain) // if getting cache fails, use empty cache

	return &BuildMetadata{
		BuildpackMetadata: buildMetadataFromBuiltImage(buildImg),
		LatestImage:       buildImg.identifier,
		LatestCacheImage:  cacheImageRef,
		StackRunImage:     buildImg.stack.RunImage,
		StackID:           buildImg.stack.ID,
	}, nil
}

func (r *RemoteMetadataRetriever) getBuiltImage(tag string, keychain authn.Keychain) (builtImage, error) {
	appImage, appImageId, err := r.ImageFetcher.Fetch(keychain, tag)
	if err != nil {
		return builtImage{}, errors.Wrap(err, "unable to fetch app image")
	}

	return readBuiltImage(appImage, appImageId)
}

func (r *RemoteMetadataRetriever) getCacheImage(cacheTag string, keychain authn.Keychain) (string, error) {
	if cacheTag == "" {
		return "", nil
	}
	_, cacheImageId, err := r.ImageFetcher.Fetch(keychain, cacheTag)
	if err != nil {
		return "", errors.Wrap(err, "unable to fetch cache image")
	}

	return cacheImageId, nil
}

func readBuiltImage(appImage ggcrv1.Image, appImageId string) (builtImage, error) {
	stackId, err := imagehelpers.GetStringLabel(appImage, platform.StackIDLabel)
	if err != nil {
		return builtImage{}, nil
	}

	var buildMetadata platform.BuildMetadata
	err = imagehelpers.GetLabel(appImage, platform.BuildMetadataLabel, &buildMetadata)
	if err != nil {
		return builtImage{}, err
	}

	var layerMetadata appLayersMetadata
	err = imagehelpers.GetLabel(appImage, platform.LayerMetadataLabel, &layerMetadata)
	if err != nil {
		return builtImage{}, err
	}

	runImageRef, err := name.ParseReference(layerMetadata.RunImage.Reference)
	if err != nil {
		return builtImage{}, err
	}

	baseImageRef, err := name.ParseReference(layerMetadata.Stack.RunImage.Image)
	if err != nil {
		return builtImage{}, err
	}

	return builtImage{
		identifier:        appImageId,
		buildpackMetadata: buildMetadata.Buildpacks,
		stack: builtImageStack{
			RunImage: baseImageRef.Context().String() + "@" + runImageRef.Identifier(),
			ID:       stackId,
		},
	}, nil
}

type builtImage struct {
	identifier        string
	buildpackMetadata []lifecyclebuildpack.GroupBuildpack
	stack             builtImageStack
}

type appLayersMetadata struct {
	RunImage RunImageAppMetadata `json:"runImage" toml:"run-image"`
	Stack    StackMetadata       `json:"stack" toml:"stack"`
}

type RunImageAppMetadata struct {
	TopLayer  string `json:"topLayer" toml:"top-layer"`
	Reference string `json:"reference" toml:"reference"`
}

func buildMetadataFromBuiltImage(image builtImage) corev1alpha1.BuildpackMetadataList {
	bpMetadata := make([]corev1alpha1.BuildpackMetadata, 0, len(image.buildpackMetadata))
	for _, metadata := range image.buildpackMetadata {
		bpMetadata = append(bpMetadata, corev1alpha1.BuildpackMetadata{
			Id:       metadata.ID,
			Version:  metadata.Version,
			Homepage: metadata.Homepage,
		})
	}
	return bpMetadata
}

func CompressBuildMetadata(metadata *BuildMetadata) ([]byte, error) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Flush(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	src := b.Bytes()
	encodedLength := base64.StdEncoding.EncodedLen(len(src))
	const maxTerminationMessageSize = 4096
	if encodedLength > maxTerminationMessageSize {
		return nil, errors.New("compressed metadata size too large")
	}
	dst := make([]byte, encodedLength)
	base64.StdEncoding.Encode(dst, src)
	return dst, nil
}

func DecompressBuildMetadata(compressedMetadata string) (*BuildMetadata, error) {
	zipData, err := base64.StdEncoding.DecodeString(compressedMetadata)
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewBuffer(zipData))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	data, err := ioutil.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	bm := &BuildMetadata{}
	if err := json.Unmarshal(data, bm); err != nil {
		return nil, err
	}
	return bm, nil
}
