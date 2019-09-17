package registry_test

import (
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/registry"
)

func TestGGCRImage(t *testing.T) {
	spec.Run(t, "GGCR Image", testGGCRImage)
}

func testGGCRImage(t *testing.T, when spec.G, it spec.S) {
	when("#CreatedAt", func() {
		it("returns created at from the image", func() {
			image, err := registry.NewGoContainerRegistryImage("cloudfoundry/cnb:bionic@sha256:33c3ad8676530f864d51d78483b510334ccc4f03368f7f5bb9d517ff4cbd630f", authn.DefaultKeychain)
			require.NoError(t, err)

			createdAt, err := image.CreatedAt()
			require.NoError(t, err)

			require.NotEqual(t, time.Time{}, createdAt)
		})
	})

	when("#Label", func() {
		it("returns created at from the image", func() {
			image, err := registry.NewGoContainerRegistryImage("cloudfoundry/cnb:bionic@sha256:33c3ad8676530f864d51d78483b510334ccc4f03368f7f5bb9d517ff4cbd630f", authn.DefaultKeychain)
			require.NoError(t, err)

			metadata, err := image.Label("io.buildpacks.builder.metadata")
			require.NoError(t, err)

			require.NotEmpty(t, metadata)
		})
	})

	when("#Env", func() {
		it("returns created at from the image", func() {
			image, err := registry.NewGoContainerRegistryImage("cloudfoundry/cnb:bionic@sha256:33c3ad8676530f864d51d78483b510334ccc4f03368f7f5bb9d517ff4cbd630f", authn.DefaultKeychain)
			require.NoError(t, err)

			cnbUserId, err := image.Env("CNB_USER_ID")
			require.NoError(t, err)

			require.NotEmpty(t, cnbUserId)
		})
	})

	when("#identifer", func() {
		it("includes digest if repoName does not have a digest", func() {
			image, err := registry.NewGoContainerRegistryImage("cloudfoundry/cnb:bionic", authn.DefaultKeychain)
			require.NoError(t, err)

			identifier, err := image.Identifier()
			require.NoError(t, err)
			require.Len(t, identifier, 104)
			require.Equal(t, identifier[0:40], "index.docker.io/cloudfoundry/cnb@sha256:")
		})

		it("includes digest if repoName already has a digest", func() {
			image, err := registry.NewGoContainerRegistryImage("cloudfoundry/cnb:bionic@sha256:33c3ad8676530f864d51d78483b510334ccc4f03368f7f5bb9d517ff4cbd630f", authn.DefaultKeychain)
			require.NoError(t, err)

			identifier, err := image.Identifier()
			require.NoError(t, err)
			require.Equal(t, identifier, "index.docker.io/cloudfoundry/cnb@sha256:33c3ad8676530f864d51d78483b510334ccc4f03368f7f5bb9d517ff4cbd630f")
		})
	})
}
