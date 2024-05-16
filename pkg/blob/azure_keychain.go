package blob

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
)

var (
	azScope      = "https://storage.azure.com/.default"
	azApiVersion = sas.Version
	azRegex      = regexp.MustCompile(`.*[a-z0-9]+\.([a-z]+)\.core\.windows\.net\/.*`)
)

type azKeychain struct{}

func (a azKeychain) Resolve(url string) (string, map[string]string, error) {
	submatches := azRegex.FindStringSubmatch(url)
	if len(submatches) != 2 {
		return "", nil, fmt.Errorf("not an azure url")
	}
	service := submatches[1]

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", nil, err
	}

	tk, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{azScope}})
	if err != nil {
		return "", nil, err
	}

	headers := map[string]string{
		"x-ms-version": azApiVersion,
		"x-ms-date":    time.Now().Format(time.RFC1123),
	}

	// https://learn.microsoft.com/en-us/rest/api/storageservices/get-file
	if service == "file" {
		headers["x-ms-file-request-intent"] = "backup"
	}

	return "Bearer " + tk.Token, headers, nil
}
