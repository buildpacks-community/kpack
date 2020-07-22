package secret

type BasicAuth struct {
	Username string
	Password string
}

type SSH struct {
	PrivateKey []byte
}

type OpaqueSecret struct {
	StringData map[string]string
}
