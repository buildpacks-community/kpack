package testhelpers

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func CompactJSON(spacedJSONStr string) string {
	var compactedBuffer bytes.Buffer
	err := json.Compact(&compactedBuffer, []byte(spacedJSONStr))
	if err != nil {
		fmt.Printf("Error compacting JSON string:\n%s\n", spacedJSONStr)
	}
	return compactedBuffer.String()
}
