package cnb

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	ignore "github.com/sabhiram/go-gitignore"
)

const fileDescriptorName = "project.toml"

func ProcessProjectDescriptor(appDir, platformDir string, logger *log.Logger) error {
	var d descriptor
	file := filepath.Join(appDir, fileDescriptorName)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to determine if project descriptor file exists: %w", err)
	}
	_, err := toml.DecodeFile(file, &d)
	if err != nil {
		return err
	}
	if d.Build.Buildpacks != nil {
		logger.Println("warning: buildpacks provided in project descriptor file will be ignored")
	}

	if d.Build.Builder != "" {
		logger.Println("warning: builder provided in project descriptor file will be ignored")
	}
	if err := processFiles(appDir, d); err != nil {
		return err
	}
	return serializeEnvVars(d.Build.Env, platformDir)
}

func processFiles(appDir string, d descriptor) error {
	fileFilter, err := getFileFilter(d)
	if err != nil {
		return err
	}
	if fileFilter == nil {
		return nil
	}
	return filepath.Walk(appDir, func(path string, f os.FileInfo, fileError error) error {
		if fileError != nil {
			return fileError
		}
		relPath, err := filepath.Rel(appDir, path)
		if err != nil {
			return err
		}
		// We only want to remove paths that don't match the patterns and are
		// files otherwise we will end up removing too much.
		// For eg if the include = ["*jar"]
		// All the directories will not match the pattern and hence be removed.
		// On the other hand if a directory is excluded/included,
		// for eg include = "my-dir" files under "my-dir" will match the pattern and not be removed.
		if !fileFilter(relPath) && !f.IsDir() {
			return os.Remove(path)
		}
		return nil
	})
}

func getFileFilter(d descriptor) (func(string) bool, error) {
	if d.Build.Exclude != nil && d.Build.Include != nil {
		return nil, fmt.Errorf("%s: cannot have both include and exclude defined", fileDescriptorName)
	}

	if len(d.Build.Exclude) > 0 {
		excludes := ignore.CompileIgnoreLines(d.Build.Exclude...)
		return func(fileName string) bool {
			return !excludes.MatchesPath(fileName)
		}, nil
	}
	if len(d.Build.Include) > 0 {
		includes := ignore.CompileIgnoreLines(d.Build.Include...)
		return includes.MatchesPath, nil
	}

	return nil, nil
}

type buildpack struct {
	Id      string `json:"id" toml:"id"`
	Version string `json:"version" toml:"version"`
	Uri     string `json:"uri" uri:"uri"`
}

type build struct {
	Include    []string      `toml:"include"`
	Exclude    []string      `toml:"exclude"`
	Buildpacks []buildpack   `toml:"buildpacks"`
	Builder    string        `toml:"builder"`
	Env        []envVariable `toml:"env"`
}

type descriptor struct {
	Build build `toml:"build"`
}
