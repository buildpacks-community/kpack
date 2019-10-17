package flaghelpers

import "strings"

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
