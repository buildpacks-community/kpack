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

const (
	cosignRepositoryEnv       = "COSIGN_REPOSITORY"
	cosignDockerMediaTypesEnv = "COSIGN_DOCKER_MEDIA_TYPES"
)

var (
	cliSignCmd     = cli.SignCmd
	secretLocation = "/var/build-secrets"
)

func NewImageSigner(logger *log.Logger) *ImageSigner {
	return &ImageSigner{
		Logger: logger,
	}
}

func (s *ImageSigner) Sign(reportFilePath string, annotations map[string]interface{}, cosignRepositories map[string]interface{}, cosignDockerMediaTypes map[string]interface{}) error {
	var report lifecycle.ExportReport
	_, err := toml.DecodeFile(reportFilePath, &report)
	if err != nil {
		return fmt.Errorf("toml decode: %v", err)
	}

	if len(report.Image.Tags) < 1 {
		s.Logger.Println("no image tag to sign")
		return nil
	}

	cosignSecrets := findCosignSecrets(secretLocation)
	if len(cosignSecrets) == 0 {
		s.Logger.Println("no keys found for cosign signing")
		return nil
	}

	refImage := report.Image.Tags[0]

	ctx := context.Background()
	for _, cosignSecret := range cosignSecrets {
		cosignKeyFile := fmt.Sprintf("%s/%s/cosign.key", secretLocation, cosignSecret)
		cosignPasswordFile := fmt.Sprintf("%s/%s/cosign.password", secretLocation, cosignSecret)

		ko := cli.KeyOpts{KeyRef: cosignKeyFile, PassFunc: func(bool) ([]byte, error) {
			content, err := ioutil.ReadFile(cosignPasswordFile)
			if err != nil {
				return []byte(""), nil
			}

			return content, nil
		}}

		if cosignRepository, ok := cosignRepositories[cosignSecret]; ok {
			os.Setenv(cosignRepositoryEnv, fmt.Sprintf("%s", cosignRepository))
		}

		if cosignDockerMediaType, ok := cosignDockerMediaTypes[cosignSecret]; ok {
			os.Setenv(cosignDockerMediaTypesEnv, fmt.Sprintf("%s", cosignDockerMediaType))
		}

		if err := cliSignCmd(ctx, ko, annotations, refImage, "", true, "", false, false); err != nil {
			os.Unsetenv(cosignRepositoryEnv)
			os.Unsetenv(cosignDockerMediaTypesEnv)

			return fmt.Errorf("unable to sign image with %s: %v", cosignKeyFile, err)
		}

		os.Unsetenv(cosignRepositoryEnv)
		os.Unsetenv(cosignDockerMediaTypesEnv)
	}

	return nil
}

// Only look at `/secretLocation/folder/cosign.key` folder/file structure
// Returns list of the secret `folder` name only
func findCosignSecrets(dir string) []string {
	var files []string
	filepath.Walk(dir, func(fullpath string, f os.FileInfo, err error) error {
		if err != nil || f == nil {
			return nil
		}

		if !f.IsDir() {
			r, err := regexp.MatchString(`^`+regexp.QuoteMeta(secretLocation)+`\/[^\/]+\/cosign.key`, fullpath)
			if err == nil && r {
				files = append(files, filepath.Base(filepath.Dir(fullpath)))
			}
		}
		return nil
	})
	return files
}
