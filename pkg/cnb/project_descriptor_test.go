package cnb_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/cnb"
)

func TestProcessProjectDescriptor(t *testing.T) {
	spec.Run(t, "ProcessProjectDescriptor", testProcessProjectDescriptor)
}

func testProcessProjectDescriptor(t *testing.T, when spec.G, it spec.S) {
	var buf *bytes.Buffer
	var logger *log.Logger
	var (
		appDir, platformDir, projectToml string
	)

	it.Before(func() {
		var err error
		buf = new(bytes.Buffer)
		logger = log.New(buf, "", 0)
		appDir, err = ioutil.TempDir("", "appDir")
		require.NoError(t, err)
		platformDir, err = ioutil.TempDir("", "platform")
		require.NoError(t, err)
		projectToml = filepath.Join(appDir, "project.toml")
	})

	it.After(func() {
		os.RemoveAll(appDir)
		os.RemoveAll(platformDir)
	})

	when("#process", func() {
		when("the descriptor has build env vars", func() {
			it.Before(func() {
				ioutil.WriteFile(projectToml, []byte(`
[[build.env]]
name = "keyA"
value = "valueA"

[[build.env]]
name = "keyB"
value = "valueB"

[[build.env]]
name = "keyC"
value = "valueC"

# check that later keys override previous ones
[[build.env]]
name = "keyC"
value = "valueAnotherC"

				`), 0644)
			})
			it("writes all env var files to the platform dir", func() {
				assert.Nil(t, cnb.ProcessProjectDescriptor(appDir, platformDir, logger))
				checkEnvVar(t, platformDir, "keyA", "valueA")
				checkEnvVar(t, platformDir, "keyB", "valueB")
				checkEnvVar(t, platformDir, "keyC", "valueAnotherC")
			})
		})

		when("the descriptor has includes and excludes", func() {
			it.Before(func() {
				var err error
				// Create test directories and files:
				//
				// ├── cookie.jar
				// ├── other-cookie.jar
				// ├── nested-cookie.jar
				// ├── nested
				// │   └── nested-cookie.jar
				// ├── secrets
				// │   ├── api_keys.json
				// |   |── user_token
				// ├── media
				// │   ├── mountain.jpg
				// │   └── person.png
				// └── test.sh

				err = os.Mkdir(filepath.Join(appDir, "secrets"), 0755)
				assert.Nil(t, err)
				err = ioutil.WriteFile(filepath.Join(appDir, "secrets", "api_keys.json"), []byte("{}"), 0755)
				assert.Nil(t, err)
				err = ioutil.WriteFile(filepath.Join(appDir, "secrets", "user_token"), []byte("token"), 0755)
				assert.Nil(t, err)

				err = os.Mkdir(filepath.Join(appDir, "nested"), 0755)
				assert.Nil(t, err)
				err = ioutil.WriteFile(filepath.Join(appDir, "nested", "nested-cookie.jar"), []byte("chocolate chip"), 0755)
				assert.Nil(t, err)

				err = ioutil.WriteFile(filepath.Join(appDir, "other-cookie.jar"), []byte("chocolate chip"), 0755)
				assert.Nil(t, err)

				err = ioutil.WriteFile(filepath.Join(appDir, "nested-cookie.jar"), []byte("chocolate chip"), 0755)
				assert.Nil(t, err)

				err = os.Mkdir(filepath.Join(appDir, "media"), 0755)
				assert.Nil(t, err)
				err = ioutil.WriteFile(filepath.Join(appDir, "media", "mountain.jpg"), []byte("fake image bytes"), 0755)
				assert.Nil(t, err)
				err = ioutil.WriteFile(filepath.Join(appDir, "media", "person.png"), []byte("fake image bytes"), 0755)
				assert.Nil(t, err)

				err = ioutil.WriteFile(filepath.Join(appDir, "cookie.jar"), []byte("chocolate chip"), 0755)
				assert.Nil(t, err)
				err = ioutil.WriteFile(filepath.Join(appDir, "test.sh"), []byte("echo test"), 0755)
				assert.Nil(t, err)
			})

			when("it has excludes", func() {
				it.Before(func() {
					ioutil.WriteFile(projectToml, []byte(`
[build]
exclude = ["*.sh", "secrets/", "media/metadata", "/other-cookie.jar" ,"/nested-cookie.jar"]					
					`), 0644)
				})
				it("removes the excluded files", func() {
					assert.Nil(t, cnb.ProcessProjectDescriptor(appDir, platformDir, logger))
					assert.NoFileExists(t, filepath.Join(appDir, "api_keys.json"))
					assert.NoFileExists(t, filepath.Join(appDir, "user_token"))
					assert.NoFileExists(t, filepath.Join(appDir, "test.sh"))
					assert.NoFileExists(t, filepath.Join(appDir, "other-cookie.jar"))
					assert.NoFileExists(t, filepath.Join(appDir, "nested-cookie.jar"))
					assert.FileExists(t, filepath.Join(appDir, "cookie.jar"))
					assert.FileExists(t, filepath.Join(appDir, "nested", "nested-cookie.jar"))
					assert.FileExists(t, filepath.Join(appDir, "media", "mountain.jpg"))
					assert.FileExists(t, filepath.Join(appDir, "media", "person.png"))
				})
			})

			when("it has includes", func() {
				it.Before(func() {
					ioutil.WriteFile(projectToml, []byte(`
[build]
include = [ "*.jar", "media/mountain.jpg", "/media/person.png", ]
					`), 0644)
				})

				it("keeps only the included files", func() {
					assert.Nil(t, cnb.ProcessProjectDescriptor(appDir, platformDir, logger))
					assert.NoFileExists(t, filepath.Join(appDir, "api_keys.json"))
					assert.NoFileExists(t, filepath.Join(appDir, "user_token"))
					assert.NoFileExists(t, filepath.Join(appDir, "test.sh"))
					assert.FileExists(t, filepath.Join(appDir, "other-cookie.jar"))
					assert.FileExists(t, filepath.Join(appDir, "nested-cookie.jar"))
					assert.FileExists(t, filepath.Join(appDir, "cookie.jar"))
					assert.FileExists(t, filepath.Join(appDir, "nested", "nested-cookie.jar"))
					assert.FileExists(t, filepath.Join(appDir, "media", "mountain.jpg"))
					assert.FileExists(t, filepath.Join(appDir, "media", "person.png"))
				})
			})

			when("it has both excludes and includes", func() {
				it.Before(func() {
					ioutil.WriteFile(projectToml, []byte(`
[build]
include = [ "test", ]
exclude = ["test", ]
					`), 0644)
				})
				it("throws an error", func() {
					assert.NotNil(t, cnb.ProcessProjectDescriptor(appDir, platformDir, logger))
				})

			})
		})
		when("the descriptor has builder", func() {
			it.Before(func() {
				ioutil.WriteFile(projectToml, []byte(`
[build]
builder = "my-super-cool-builder"

				`), 0644)
			})
			it("logs a warning that the builder will be ignored", func() {
				assert.Nil(t, cnb.ProcessProjectDescriptor(appDir, platformDir, logger))
				assert.Equal(t, "info: builder provided in project descriptor file will be ignored\n", buf.String())
			})
		})
		when("the descriptor has buildpacks", func() {
			it.Before(func() {
				ioutil.WriteFile(projectToml, []byte(`
[[build.buildpacks]]
id = "cool-buildpack"
version = "v4.2"
uri = "check-this-out.com"
				`), 0644)
			})
			it("logs a warning that the buildpacks will be ignored", func() {
				assert.Nil(t, cnb.ProcessProjectDescriptor(appDir, platformDir, logger))
				assert.Equal(t, "info: buildpacks provided in project descriptor file will be ignored\n", buf.String())
			})
		})

	})
}
