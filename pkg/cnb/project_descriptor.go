package cnb

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	ignore "github.com/sabhiram/go-gitignore"
)

const defaultProjectDescriptorPath = "project.toml"

func ProcessProjectDescriptor(appDir, descriptorPath, platformDir string, logger *log.Logger) error {
	file := filepath.Join(appDir, defaultProjectDescriptorPath)
	if descriptorPath != "" {
		file = filepath.Join(appDir, descriptorPath)
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		if descriptorPath != "" {
			return fmt.Errorf("project descriptor path set but no file found: %s", descriptorPath)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("unable to determine if project descriptor file exists: %w", err)
	}

	d, err := parseProjectDescriptor(file, logger)
	if err != nil {
		return err
	}
	if d.IO.Buildpacks.Group != nil {
		logger.Println("info: buildpacks provided in project descriptor file will be ignored")
	}

	if d.IO.Buildpacks.Builder != "" {
		logger.Println("info: builder provided in project descriptor file will be ignored")
	}
	if err := processFiles(appDir, d.IO.Buildpacks.build); err != nil {
		return err
	}
	return serializeEnvVars(d.IO.Buildpacks.Env, platformDir)
}

func parseProjectDescriptor(file string, logger *log.Logger) (descriptorV2, error) {
	var d descriptorV2
	if _, err := toml.DecodeFile(file, &d); err != nil {
		return descriptorV2{}, err
	}

	switch sv := d.Project.SchemaVersion; sv {
	case "0.2":
		return d, nil
	case "": // v1 descriptor
		var dV1 descriptorV1
		if _, err := toml.DecodeFile(file, &dV1); err != nil {
			return descriptorV2{}, err
		}
		return v1ToV2(dV1), nil
	default:
		logger.Println(fmt.Sprintf("warning: project descriptor version %s is unsupported and %s will be ignored", sv, file))
		return descriptorV2{}, nil
	}
}

func v1ToV2(v1 descriptorV1) descriptorV2 {
	return descriptorV2{
		IO: ioTable{
			Buildpacks: cnbTableV2{
				build: v1.Build.build,
				Group: v1.Build.Buildpacks,
			},
		},
	}
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
		return nil, fmt.Errorf("project descriptor cannot have both include and exclude defined")
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

type descriptorV2 struct {
	Project project `toml:"_"`
	IO      ioTable `toml:"io"`
}

type project struct {
	SchemaVersion string `toml:"schema-version"`
}

type ioTable struct {
	Buildpacks cnbTableV2 `toml:"buildpacks"`
}

type cnbTableV2 struct {
	build `toml:",inline"`
	Group []buildpack `toml:"group"`
}

type build struct {
	Include []string      `toml:"include"`
	Exclude []string      `toml:"exclude"`
	Builder string        `toml:"builder"`
	Env     []envVariable `toml:"env"`
}

type buildpack struct {
	Id      string `json:"id" toml:"id"`
	Version string `json:"version" toml:"version"`
	Uri     string `json:"uri" toml:"uri"`
}

type descriptorV1 struct {
	Build cnbTableV1 `toml:"build"`
}

type cnbTableV1 struct {
	build      `toml:",inline"`
	Buildpacks []buildpack `toml:"buildpacks"`
}
