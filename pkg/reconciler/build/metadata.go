package build

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type BuildStatusMetadata struct {
	BuildpackMetadata corev1alpha1.BuildpackMetadataList `json:"buildpackMetadata"`
	LatestImage       string                             `json:"latestImage"`
	StackRunImage     string                             `json:"stackRunImage"`
	StackID           string                             `json:"stackID"`
}

type GzipMetadataCompressor struct{}

func (*GzipMetadataCompressor) Compress(metadata *BuildStatusMetadata) (string, error) {
	data, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write(data); err != nil {
		return "", err
	}
	if err := gz.Flush(); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

func (*GzipMetadataCompressor) Decompress(compressedMeta string) (*BuildStatusMetadata, error) {
	zipData, err := base64.StdEncoding.DecodeString(compressedMeta)
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
	bm := &BuildStatusMetadata{}
	if err := json.Unmarshal(data, bm); err != nil {
		return nil, err
	}
	return bm, nil
}
