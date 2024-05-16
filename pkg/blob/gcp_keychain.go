package blob

import (
	"context"

	"golang.org/x/oauth2/google"
)

const (
	gcpScope = "https://www.googleapis.com/auth/devstorage.read_only"
)

type gcpKeychain struct{}

func (g gcpKeychain) Resolve(url string) (string, map[string]string, error) {
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, gcpScope)
	if err != nil {
		return "", nil, err
	}

	tk, err := creds.TokenSource.Token()
	if err != nil {
		return "", nil, err
	}

	return "Bearer " + tk.AccessToken, nil, nil
}
