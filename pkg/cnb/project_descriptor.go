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

func parseProjectDescriptor(file string, logger *log.Logger) (build, error) {
	var dv2 descriptorV2
	if _, err := toml.DecodeFile(file, &dv2); err != nil {
		return build{}, err
	}

	if dv2.Project.SchemaVersion != "" {
		if dv2.Project.SchemaVersion == "0.2" {
			// Normalizing the buildpacks table to a common schema
			dv2.IO.Buildpacks.Buildpacks = dv2.IO.Buildpacks.Group
			dv2.IO.Buildpacks.Group = nil
			return dv2.IO.Buildpacks, nil
		} else {
			logger.Println(fmt.Sprintf("warning: project descriptor version %s is unsupported and %s will be ignored", dv2.Project.SchemaVersion, file))
			return build{}, nil
		}
	}
	var d descriptor
	if _, err := toml.DecodeFile(file, &d); err != nil {
		return build{}, err
	}
	// Removing groups from v1 descriptor if it exists
	d.Build.Group = nil
	return d.Build, nil
}

func ProcessProjectDescriptor(appDir, platformDir string, logger *log.Logger) error {

	file := filepath.Join(appDir, fileDescriptorName)
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to determine if project descriptor file exists: %w", err)
	}
	d, err := parseProjectDescriptor(file, logger)
	if err != nil {
		return err
	}
	if d.Buildpacks != nil {
		logger.Println("info: buildpacks provided in project descriptor file will be ignored")
	}

	if d.Builder != "" {
		logger.Println("info: builder provided in project descriptor file will be ignored")
	}
	if err := processFiles(appDir, d); err != nil {
		return err
	}
	return serializeEnvVars(d.Env, platformDir)
}

func processFiles(appDir string, d build) error {
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

func getFileFilter(d build) (func(string) bool, error) {
	if d.Exclude != nil && d.Include != nil {
		return nil, fmt.Errorf("%s: cannot have both include and exclude defined", fileDescriptorName)
	}

	if len(d.Exclude) > 0 {
		excludes := ignore.CompileIgnoreLines(d.Exclude...)
		return func(fileName string) bool {
			return !excludes.MatchesPath(fileName)
		}, nil
	}
	if len(d.Include) > 0 {
		includes := ignore.CompileIgnoreLines(d.Include...)
		return includes.MatchesPath, nil
	}

	return nil, nil
}

type buildpack struct {
	Id      string `json:"id" toml:"id"`
	Version string `json:"version" toml:"version"`
	Uri     string `json:"uri" toml:"uri"`
}

type build struct {
	Include    []string      `toml:"include"`
	Exclude    []string      `toml:"exclude"`
	Buildpacks []buildpack   `toml:"buildpacks"`
	Group      []buildpack   `toml:"group"`
	Builder    string        `toml:"builder"`
	Env        []envVariable `toml:"env"`
}

type descriptor struct {
	Build build `toml:"build"`
}

type descriptorV2 struct {
	Project project `toml:"_"`
	IO      ioTable `toml:"io"`
}

type project struct {
	SchemaVersion string `toml:"schema-version"`
}

type ioTable struct {
	Buildpacks build `toml:"buildpacks"`
}
