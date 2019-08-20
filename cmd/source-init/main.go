package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	gitURL        = flag.String("git-url", os.Getenv("GIT_URL"), "The url of the Git repository to initialize.")
	gitRevision   = flag.String("git-revision", os.Getenv("GIT_REVISION"), "The Git revision to make the repository HEAD.")
	blobURL       = flag.String("blob-url", os.Getenv("BLOB_URL"), "The url of the source code blob.")
	registryImage = flag.String("registry-image", os.Getenv("REGISTRY_IMAGE"), "The registry location of the source code image.")
)

func run(logger *log.Logger, cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output
	if err := c.Run(); err != nil {
		logger.Printf("Error running %v %v: %v\n%v", cmd, args, err, output.String())
	}
}

func runOrFail(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	var output bytes.Buffer
	c.Stderr = &output
	c.Stdout = &output

	if err := c.Run(); err != nil {
		return err
	}
	return nil
}

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "source-init:", log.Lshortfile)

	usr, err := user.Current() // The user should be root to be able to read .git-credentials and .gitconfig
	if err != nil {
		log.Fatal(err)
	}

	symlinks := []string{".ssh", ".git-credentials", ".gitconfig"}
	for _, path := range symlinks {
		err = os.Symlink("/builder/home/"+path, filepath.Join(usr.HomeDir, path))
		if err != nil {
			logger.Fatalf("Unexpected error creating symlink: %v", err)
		}
	}

	err = os.MkdirAll(filepath.Join(usr.HomeDir, ".docker"), os.ModePerm)
	if err != nil {
		logger.Fatal(err)
	}

	if fileExists("/imagePullSecrets/.dockerconfigjson", logger) {
		err := os.Symlink("/imagePullSecrets/.dockerconfigjson", filepath.Join(usr.HomeDir, ".docker/config.json"))
		if err != nil {
			logger.Fatal(err)
		}
	} else if fileExists("/imagePullSecrets/.dockercfg", logger) {
		file, err := os.Open("/imagePullSecrets/.dockercfg")
		if err != nil {
			logger.Fatal(err)
		}
		defer file.Close()
		fileContents, err := ioutil.ReadAll(file)
		if err != nil {
			logger.Fatal(err)
		}
		configJson := fmt.Sprintf(`{ "auths" : %s }`, string(fileContents))
		tempFile, err := ioutil.TempFile("", "")
		if err != nil {
			logger.Fatal(err)
		}
		defer tempFile.Close()
		err = ioutil.WriteFile(tempFile.Name(), []byte(configJson), os.ModeType)
		if err != nil {
			logger.Fatal(err)
		}
		err = os.Symlink(tempFile.Name(), filepath.Join(usr.HomeDir, ".docker/config.json"))
		if err != nil {
			logger.Fatal(err)
		}
	} else if fileExists("/builder/home/.docker/config.json", logger) {
		err := os.Symlink("/builder/home/.docker/config.json", filepath.Join(usr.HomeDir, ".docker/config.json"))
		if err != nil {
			logger.Fatal(err)
		}
	}

	err = os.Setenv("DOCKER_CONFIG", filepath.Join(usr.HomeDir, ".docker"))
	if err != nil {
		logger.Fatal(err)
	}

	dir, err := os.Getwd()
	if err != nil {
		logger.Fatal("Failed to get current dir", err)
	}

	if *gitURL != "" {
		checkoutGitSource(dir, logger)
	} else if *blobURL != "" {
		downloadBlob(dir, logger)
	} else if *registryImage != "" {
		fetchImage(dir, logger)
	} else {
		logger.Fatal("no git url, blob url, or registry image provided")
	}
}

func fetchImage(dir string, logger *log.Logger) {
	ref, err := name.ParseReference(*registryImage, name.WeakValidation)
	if err != nil {
		logger.Fatal(err)
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		logger.Fatal(err)
	}

	layers, err := img.Layers()
	if err != nil {
		logger.Fatal(err)
	}

	for _, layer := range layers {
		fetchLayer(layer, logger, dir)
	}
	logger.Printf("Successfully pulled %s in path %q", *registryImage, dir)
}

func fetchLayer(layer v1.Layer, logger *log.Logger, dir string) {
	reader, err := layer.Uncompressed()
	if err != nil {
		logger.Fatal(err)
	}
	defer reader.Close()

	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			logger.Fatal(err)
		}

		filePath := filepath.Join(dir, header.Name)
		if header.FileInfo().IsDir() {
			err := os.MkdirAll(filePath, header.FileInfo().Mode())
			if err != nil {
				logger.Fatal(err.Error())
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			logger.Fatal(err.Error())
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
		if err != nil {
			logger.Fatal(err.Error())
		}

		_, err = io.Copy(outFile, tarReader)
		outFile.Close()
		if err != nil {
			logger.Fatal(err.Error())
		}
	}
}

func downloadBlob(dir string, logger *log.Logger) {
	blob, err := url.Parse(*blobURL)
	if err != nil {
		logger.Fatal(err.Error())
	}

	resp, err := http.Get(*blobURL)
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer resp.Body.Close()

	file, err := ioutil.TempFile("", "")
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer os.RemoveAll(file.Name())

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		logger.Fatal(err.Error())
	}

	zipReader, err := zip.OpenReader(file.Name())
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		filePath := filepath.Join(dir, file.Name)
		if file.FileInfo().IsDir() {
			err := os.MkdirAll(filePath, file.Mode())
			if err != nil {
				logger.Fatal(err.Error())
			}
			continue
		}

		if err = os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
			logger.Fatal(err.Error())
		}

		outFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			logger.Fatal(err.Error())
		}

		srcFile, err := file.Open()
		if err != nil {
			logger.Fatal(err.Error())
		}

		_, err = io.Copy(outFile, srcFile)

		outFile.Close()
		srcFile.Close()

		if err != nil {
			logger.Fatal(err.Error())
		}
	}
	logger.Printf("Successfully downloaded %s in path %q", blob.Host+blob.Path, dir)
}

func checkoutGitSource(dir string, logger *log.Logger) {
	run(logger, "git", "init")
	run(logger, "git", "remote", "add", "origin", *gitURL)
	err := runOrFail("git", "fetch", "--depth=1", "--recurse-submodules=yes", "origin", *gitRevision)
	if err != nil {
		run(logger, "git", "pull", "--recurse-submodules=yes", "origin")
		err = runOrFail("git", "checkout", *gitRevision)
		if err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		run(logger, "git", "reset", "--hard", "FETCH_HEAD")
	}
	logger.Printf("Successfully cloned %q @ %q in path %q", *gitURL, *gitRevision, dir)
}

func fileExists(file string, logger *log.Logger) bool {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		logger.Fatal(err.Error())
	}

	return true
}
