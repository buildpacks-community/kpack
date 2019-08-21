package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
)

var (
	builder         = flag.String("builder", os.Getenv("BUILDER"), "the builder to initialize the env for a build")
	platformEnvVars = flag.String("platformEnvVars", os.Getenv("PLATFORM_ENV_VARS"), "a JSON string of build time environment variables formatted as key/value pairs")
)

func main() {
	flag.Parse()

	logger := log.New(os.Stdout, "prepare:", log.Lshortfile)

	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	err = os.MkdirAll(filepath.Join(usr.HomeDir, ".docker"), os.ModePerm)
	if err != nil {
		logger.Fatal(err)
	}

	if fileExists("/builderPullSecrets/.dockerconfigjson", logger) {
		err := os.Symlink("/builderPullSecrets/.dockerconfigjson", filepath.Join(usr.HomeDir, ".docker/config.json"))
		if err != nil {
			logger.Fatal(err)
		}
	} else if fileExists("/builderPullSecrets/.dockercfg", logger) {
		file, err := os.Open("/builderPullSecrets/.dockercfg")
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
	}

	err = os.Setenv("DOCKER_CONFIG", filepath.Join(usr.HomeDir, ".docker"))
	if err != nil {
		logger.Fatal(err)
	}

	remoteImageFactory := &registry.ImageFactory{
		KeychainFactory: defaultKeychainFactory{},
	}

	filePermissionSetup := &cnb.FilePermissionSetup{
		RemoteImageFactory: remoteImageFactory,
		Chowner:            realOs{},
	}
	err = filePermissionSetup.Setup(
		*builder,
		"/builder/home", "/layersDir", "/cache", "/workspace",
	)
	if err != nil {
		logger.Fatalf("error setting up permissions %s", err)
	}

	err = cnb.SetupPlatformEnvVars("/platform", *platformEnvVars)
	if err != nil {
		logger.Fatalf("error setting up platform env vars %s", err)
	}
}

type defaultKeychainFactory struct {
}

func (defaultKeychainFactory) KeychainForImageRef(registry.ImageRef) authn.Keychain {
	return authn.DefaultKeychain
}

type realOs struct {
}

func (realOs) Chown(volume string, uid, gid int) error {
	return os.Chown(volume, uid, gid)
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
