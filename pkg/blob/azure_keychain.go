package blob

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	blobsas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	filesas "github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/sas"
)

var (
	azScope = "https://storage.azure.com/.default"
	azRegex = regexp.MustCompile(`.*[a-z0-9]+\.([a-z]+)\.core\.windows\.net\/.*`)
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
		"x-ms-date": time.Now().Format(http.TimeFormat),
	}

	switch service {
	case "file":
		headers["x-ms-version"] = filesas.Version
		// https://learn.microsoft.com/en-us/rest/api/storageservices/get-file
		headers["x-ms-file-request-intent"] = "backup"
	case "blob":
		headers["x-ms-version"] = blobsas.Version
	default:
		return "", nil, fmt.Errorf("only azure blob and filestore are supported")
	}

	return "Bearer " + tk.Token, headers, nil
}
