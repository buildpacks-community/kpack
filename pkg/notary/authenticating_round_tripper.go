package notary

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/pkg/errors"
)

type AuthenticatingRoundTripper struct {
	Token               string
	Keychain            authn.Keychain
	WrappedRoundTripper http.RoundTripper
}

func (a *AuthenticatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if a.Token != "" {
		req.Header["Authorization"] = []string{"Bearer " + a.Token}
		return a.WrappedRoundTripper.RoundTrip(req)
	}

	c := http.Client{
		Transport: a.WrappedRoundTripper,
		Timeout:   30 * time.Second,
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	header := resp.Header.Get("www-authenticate")
	parts := strings.SplitN(header, " ", 2)
	parts = strings.Split(parts[1], ",")
	opts := make(map[string]string)

	for _, part := range parts {
		vals := strings.SplitN(part, "=", 2)
		key := vals[0]
		val := strings.Trim(vals[1], "\",")
		opts[key] = val
	}

	authReq, err := http.NewRequest(http.MethodGet, opts["realm"], nil)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Add("service", opts["service"])
	q.Add("scope", opts["scope"])
	q.Add("client_id", "kpack")
	authReq.URL.RawQuery = q.Encode()

	resource, err := parseRegistryAuthResource(opts["realm"])
	if err != nil {
		return nil, err
	}

	auth, err := a.Keychain.Resolve(resource)
	if err != nil {
		return nil, err
	}

	authConfig, err := auth.Authorization()
	if err != nil {
		return nil, err
	}

	authReq.SetBasicAuth(authConfig.Username, authConfig.Password)

	resp, err = c.Do(authReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("received status code '%d'", resp.StatusCode)
	}

	var body map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&body)
	if err != nil {
		return nil, err
	}

	token, ok := body["token"].(string)
	if !ok {
		return nil, errors.New("failed to retrieve token from auth response")
	}

	a.Token = token
	req.Header["Authorization"] = []string{"Bearer " + a.Token}

	return a.WrappedRoundTripper.RoundTrip(req)
}

type registryAuthResource struct {
	URL string
}

func parseRegistryAuthResource(realm string) (registryAuthResource, error) {
	parsedURL, err := url.Parse(realm)
	if err != nil {
		return registryAuthResource{}, err
	}

	return registryAuthResource{URL: parsedURL.Host}, nil
}

func (r registryAuthResource) String() string {
	return r.URL
}

func (r registryAuthResource) RegistryStr() string {
	return r.URL
}
