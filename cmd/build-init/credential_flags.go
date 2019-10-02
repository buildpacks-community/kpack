package main

import "strings"

type credentialsFlags []string

func (i *credentialsFlags) String() string {
	builder := strings.Builder{}
	for _, v := range *i {
		builder.WriteString(v)
	}
	return builder.String()
}

func (i *credentialsFlags) Set(value string) error {
	*i = append(*i, value)
	return nil
}
