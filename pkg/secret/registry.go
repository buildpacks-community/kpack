package secret

type URLAndUser struct {
	URL      string
	Username string
	Password string
}

func NewURLAndUser(url, username, password string) URLAndUser {
	return URLAndUser{
		URL:      url,
		Username: username,
		Password: password,
	}
}
