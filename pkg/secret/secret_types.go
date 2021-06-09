package secret

type BasicAuth struct {
	Username string
	Password string
}

type SSH struct {
	PrivateKey string
}
