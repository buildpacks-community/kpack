package secret

const (
	SSHAuthKnownHostsKey = "known_hosts"
)

type BasicAuth struct {
	Username string
	Password string
}

type SSH struct {
	PrivateKey     string
	KnownHostsFile []string
}
