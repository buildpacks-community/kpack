package slsa

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	ggcrfake "github.com/google/go-containerregistry/pkg/v1/fake"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestImageReader(t *testing.T) {
	spec.Run(t, "Test image metadata reading", testImageReader)
}

func testImageReader(t *testing.T, when spec.G, it spec.S) {
	it("returns the correct image repository and sha", func() {
		image := ggcrfake.FakeImage{}
		image.ConfigFileReturns(&ggcrv1.ConfigFile{
			Config: ggcrv1.Config{
				Labels: map[string]string{"some-label": "some-value"},
			},
		}, nil)

		r := NewImageReader(&fakeFetcher{
			images: map[string]ggcrv1.Image{
				"some-registry.io/some/repo@sha256:8cc53b8113f3a2fba8bd5683a69178d44e38bb172787713cd6a5d21ac3a7ad13": &image,
			},
		})

		repo, sha, _, err := r.Read(authn.DefaultKeychain, "some-registry.io/some/repo@sha256:8cc53b8113f3a2fba8bd5683a69178d44e38bb172787713cd6a5d21ac3a7ad13")
		require.NoError(t, err)

		require.Equal(t, "some-registry.io/some/repo", repo)
		require.Equal(t, "8cc53b8113f3a2fba8bd5683a69178d44e38bb172787713cd6a5d21ac3a7ad13", sha)
	})

	it("can parse project metadata of type git", func() {
		metadata, err := tomlToJsonString(git.Project{
			Source: git.Source{
				Type:     "git",
				Metadata: git.Metadata{Repository: "https://some-git.repo", Revision: "some-branch"},
				Version:  git.Version{Commit: "some-commitsh"},
			},
		})
		require.NoError(t, err)

		labels := map[string]string{
			"some-label":                     "some-value",
			"io.buildpacks.project.metadata": metadata,
		}

		repo, sha, err := extractSourceFromLabel(labels)
		require.NoError(t, err)

		require.Equal(t, "https://some-git.repo", repo)
		require.Equal(t, slsacommon.DigestSet{"sha1": "some-commitsh"}, sha)
	})

	it("can parse project metadata of type blob", func() {
		metadata, err := tomlToJsonString(blob.Project{
			Source: blob.Source{
				Type:     "blob",
				Metadata: blob.Metadata{Url: "https://some-blob.store"},
				Version:  blob.Version{SHA256: "some-sha256sum"},
			},
		})
		require.NoError(t, err)

		labels := map[string]string{
			"some-label":                     "some-value",
			"io.buildpacks.project.metadata": metadata,
		}

		repo, sha, err := extractSourceFromLabel(labels)
		require.NoError(t, err)

		require.Equal(t, "https://some-blob.store", repo)
		require.Equal(t, slsacommon.DigestSet{"sha256": "some-sha256sum"}, sha)
	})

	it("can parse project metadata of type image", func() {
		metadata, err := tomlToJsonString(registry.Project{
			Source: registry.Source{
				Type:     "image",
				Metadata: registry.Metadata{Image: "some-registry.io/repo"},
				Version:  registry.Version{Digest: "sha256:some-image-digest"},
			},
		})
		require.NoError(t, err)

		labels := map[string]string{
			"some-label":                     "some-value",
			"io.buildpacks.project.metadata": metadata,
		}

		repo, sha, err := extractSourceFromLabel(labels)
		require.NoError(t, err)

		require.Equal(t, "some-registry.io/repo", repo)
		require.Equal(t, slsacommon.DigestSet{"sha256": "some-image-digest"}, sha)
	})
}

type fakeFetcher struct {
	images map[string]ggcrv1.Image
}

func (f *fakeFetcher) Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error) {
	return f.images[repoName], repoName, nil
}

type projectMetadata struct {
	Source *projectSource `toml:"source" json:"source,omitempty"`
}

type projectSource struct {
	Type     string                 `toml:"type" json:"type,omitempty"`
	Version  map[string]interface{} `toml:"version" json:"version,omitempty"`
	Metadata map[string]interface{} `toml:"metadata" json:"metadata,omitempty"`
}

// This emulates the process of the git/blob/image fetch.go  writing out toml, cnb parsing in toml
// and writing out json
func tomlToJsonString(val interface{}) (string, error) {
	buf := &bytes.Buffer{}
	err := toml.NewEncoder(buf).Encode(val)
	if err != nil {
		return "", err
	}

	var metadata projectMetadata
	_, err = toml.NewDecoder(buf).Decode(&metadata)
	if err != nil {
		return "", err
	}

	b, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
