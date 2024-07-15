package flaghelpers

import (
	"os"
	"strconv"
	"strings"
)

type CredentialsFlags []string

func (i *CredentialsFlags) String() string {
	builder := strings.Builder{}
	for _, v := range *i {
		builder.WriteString(v)
	}
	return builder.String()
}

func (i *CredentialsFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func GetEnvBool(key string, defaultValue bool) bool {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}
	v, err := strconv.ParseBool(s)
	if err != nil {
		return defaultValue
	}
	return v
}


func GetEnvInt(key string, defaultValue int) int {
	s := os.Getenv(key)
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultValue
	}
	return v
}
