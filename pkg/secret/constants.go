package secret

const (
	CosignSecretPrivateKey = "cosign.key"
	CosignSecretPassword   = "cosign.password"
	CosignSecretPublicKey  = "cosign.pub"

	CosignDockerMediaTypesAnnotation = "kpack.io/cosign.docker-media-types"
	CosignRepositoryAnnotation       = "kpack.io/cosign.repository"

	PKCS8SecretKey = "ssh-privatekey"

	SLSASecretAnnotation           = "kpack.io/slsa"
	SLSADockerMediaTypesAnnotation = "kpack.io/slsa.docker-media-types"
)
