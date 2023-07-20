package cosignutil

const (
	CosignRepositoryEnv       = "COSIGN_REPOSITORY"
	CosignDockerMediaTypesEnv = "COSIGN_DOCKER_MEDIA_TYPES"

	SecretDataCosignKey              = "cosign.key"
	SecretDataCosignPassword         = "cosign.password"
	SecretDataCosignPublicKey        = "cosign.pub"
	DockerMediaTypesAnnotationPrefix = "kpack.io/cosign.docker-media-types"
	RepositoryAnnotationPrefix       = "kpack.io/cosign.repository"
)
