package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

var (
	gitURL      = flag.String("git-url", os.Getenv("GIT_URL"), "The url of the Git repository to initialize.")
	gitRevision = flag.String("git-revision", os.Getenv("GIT_REVISION"), "The Git revision to make the repository HEAD")
	blobURL     = flag.String("blob-url", os.Getenv("BLOB_URL"), "The url of the source code blob.")
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

	dir, err := os.Getwd()
	if err != nil {
		logger.Fatal("Failed to get current dir", err)
	}

	if *gitURL != "" {
		checkoutGitSource(dir, logger)
	} else if *blobURL != "" {
		downloadBlob(dir, logger)
	} else {
		logger.Fatal("no git url or blob url provided")
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
