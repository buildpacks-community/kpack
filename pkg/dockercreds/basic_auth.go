package dockercreds

type Auth string

func (a Auth) Authorization() (string, error) {
	return "Basic " + string(a), nil
}
