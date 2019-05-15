package registry

type RegistryUser struct {
	URL      string
	Username string
	Password string
}

func NewRegistryUser(url, username, password string) RegistryUser {
	return RegistryUser{
		URL:      url,
		Username: username,
		Password: password,
	}
}
