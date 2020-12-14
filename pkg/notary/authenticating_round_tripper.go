package notary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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

	realm, err := extractBearerOption("realm", parts[1])
	if err != nil {
		return nil, err
	}

	service, err := extractBearerOption("service", parts[1])
	if err != nil {
		return nil, err
	}

	scope, err := extractBearerOption("scope", parts[1])
	if err != nil {
		return nil, err
	}

	authReq, err := http.NewRequest(http.MethodGet, realm, nil)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Add("service", service)
	q.Add("scope", scope)
	q.Add("client_id", "kpack")
	authReq.URL.RawQuery = q.Encode()

	resource, err := parseRegistryAuthResource(realm)
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

func extractBearerOption(kind string, from string) (string, error) {
	r := regexp.MustCompile(fmt.Sprintf(`%s="(.*?)"`, kind))
	m := r.FindStringSubmatch(from)
	if len(m) != 2 {
		return "", errors.Errorf("failed to parse '%s' from bearer response", kind)
	}
	return m[1], nil
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
