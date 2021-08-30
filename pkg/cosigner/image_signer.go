package cosigner

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle"
	"github.com/sigstore/cosign/cmd/cosign/cli"
)

type ImageSigner struct {
	Logger *log.Logger
}

var (
	cliSignCmd     = cli.SignCmd
	secretLocation = "/var/build-secrets"
)

func NewImageSigner(logger *log.Logger) *ImageSigner {
	return &ImageSigner{
		Logger: logger,
	}
}

func (s *ImageSigner) Sign(reportFilePath string, annotations map[string]interface{}) error {
	var report lifecycle.ExportReport
	_, err := toml.DecodeFile(reportFilePath, &report)
	if err != nil {
		return fmt.Errorf("toml decode: %v", err)
	}

	if len(report.Image.Tags) < 1 {
		s.Logger.Println("no image tag to sign")
		return nil
	}

	cosignFolders := findCosignFolders(secretLocation)
	if len(cosignFolders) == 0 {
		s.Logger.Println("no keys found for cosign signing")
		return nil
	}

	refImage := report.Image.Tags[0]

	ctx := context.Background()
	for _, cosignFolder := range cosignFolders {
		cosignKeyFile := fmt.Sprintf("%s/cosign.key", cosignFolder)
		cosignPasswordFile := fmt.Sprintf("%s/cosign.password", cosignFolder)

		ko := cli.KeyOpts{KeyRef: cosignKeyFile, PassFunc: func(bool) ([]byte, error) {
			content, err := ioutil.ReadFile(cosignPasswordFile)
			if err != nil {
				return []byte(""), nil
			}

			return content, nil
		}}

		if err := cliSignCmd(ctx, ko, annotations, refImage, "", true, "", false, false); err != nil {
			return fmt.Errorf("unable to sign image with %s: %v", cosignKeyFile, err)
		}
	}

	return nil
}

func findCosignFolders(dir string) []string {
	var files []string
	filepath.Walk(dir, func(fullpath string, f os.FileInfo, err error) error {
		if err != nil || f == nil {
			return nil
		}

		if !f.IsDir() {
			// Only look at /secretLocation/folder/cosign.key folder/file structure
			r, err := regexp.MatchString(`^`+regexp.QuoteMeta(secretLocation)+`\/[^\/]+\/cosign.key`, fullpath)
			if err == nil && r {
				files = append(files, filepath.Dir(fullpath))
			}
		}
		return nil
	})
	return files
}
