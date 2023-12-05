package slsa

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
)

const (
	ProjectMetadataLabel = "io.buildpacks.project.metadata"
)

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
}

type reader struct {
	fetcher ImageFetcher
}

func NewImageReader(fetcher ImageFetcher) *reader {
	return &reader{
		fetcher: fetcher,
	}
}

func (r *reader) Read(keychain authn.Keychain, repoName string) (string, string, map[string]string, error) {
	img, id, err := r.fetcher.Fetch(keychain, repoName)
	if err != nil {
		return "", "", nil, fmt.Errorf("fetching image: %v", err)
	}

	ref, err := name.NewDigest(id)
	if err != nil {
		return "", "", nil, fmt.Errorf("parsing digest: %v", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return "", "", nil, fmt.Errorf("getting image config: %v", err)
	}

	sha, found := strings.CutPrefix(ref.DigestStr(), "sha256:")
	if !found {
		return "", "", nil, fmt.Errorf("unknown digest format '%v'", ref.DigestStr())
	}

	return ref.Context().Name(), sha, configFile.Config.Labels, nil
}

func extractSourceFromLabel(labels map[string]string) (string, slsacommon.DigestSet, error) {
	metadata, found := labels[ProjectMetadataLabel]
	if !found {
		return "", nil, fmt.Errorf("label not found: '%v'", ProjectMetadataLabel)
	}

	var p project
	err := json.Unmarshal([]byte(metadata), &p)
	if err != nil {
		return "", nil, fmt.Errorf("unmarshalling json: %v", err)
	}

	switch p.Source.Type {
	case "git":
		// while sha256 support is available, go-git still defaults to sha1 for now
		// https://github.com/go-git/go-git/issues/706
		return p.Source.Metadata.Repository, map[string]string{"sha1": p.Source.Version.Commit}, nil
	case "blob":
		return p.Source.Metadata.Url, map[string]string{"sha256": p.Source.Version.SHA256}, nil
	case "image":
		sha, found := strings.CutPrefix(p.Source.Version.Digest, "sha256:")
		if !found {
			return "", nil, fmt.Errorf("unknown digest format '%v'", p.Source.Version.Digest)
		}
		return p.Source.Metadata.Image, map[string]string{"sha256": sha}, nil
	default:
		return "", nil, fmt.Errorf("unknown project type: '%v'", p.Source.Type)
	}
}

type project struct {
	Source source `json:"source"`
}

type source struct {
	Type     string   `json:"type"`
	Metadata metadata `json:"metadata"`
	Version  version  `json:"version"`
}

type metadata struct {
	Repository string `json:"repository"`
	Revision   string `json:"revision"`
	Image      string `json:"image"`
	Url        string `json:"url"`
}

type version struct {
	Commit string `json:"commit"`
	Digest string `json:"digest"`
	SHA256 string `json:"sha256sum"`
}
