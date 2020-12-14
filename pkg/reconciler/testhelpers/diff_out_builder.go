package testhelpers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mgutz/ansi"
	"github.com/stretchr/testify/assert"
)

const defaultPrefix = "\t"

type DiffOutBuilder struct {
	t  *testing.T
	sb strings.Builder
	o  DiffOptions
}

type DiffOptions struct {
	Prefix string
	Color  bool
}

func DefaultDiffOptions() DiffOptions {
	return DiffOptions{
		Prefix: defaultPrefix,
		Color:  true,
	}
}

func NewDiffOutBuilder(t *testing.T) *DiffOutBuilder {
	builder := &DiffOutBuilder{t: t, sb: strings.Builder{}}
	builder.Configure(DefaultDiffOptions())
	return builder
}

func (d *DiffOutBuilder) Configure(options DiffOptions) *DiffOutBuilder {
	d.o = options
	return d
}

func (d *DiffOutBuilder) Reset() *DiffOutBuilder {
	d.sb.Reset()
	return d
}

func (d *DiffOutBuilder) Txt(str string) *DiffOutBuilder {
	textLine := fmt.Sprintf("%s\n", str)
	_, err := d.sb.WriteString(textLine)
	assert.NoError(d.t, err)
	return d
}

func (d *DiffOutBuilder) NoD(str string) *DiffOutBuilder {
	noDiffLine := fmt.Sprintf("%s%s\n", d.o.Prefix, str)
	_, err := d.sb.WriteString(noDiffLine)
	assert.NoError(d.t, err)
	return d
}

func (d *DiffOutBuilder) Old(str string) *DiffOutBuilder {
	var oldLine string
	if d.o.Color {
		oldLine = fmt.Sprintf("%s%s %s\n", d.o.Prefix, ansi.Color("-", "red"), ansi.Color(str, "red"))
	} else {
		oldLine = fmt.Sprintf("%s%s %s\n", d.o.Prefix, "-", str)
	}
	_, err := d.sb.WriteString(oldLine)
	assert.NoError(d.t, err)
	return d
}

func (d *DiffOutBuilder) New(str string) *DiffOutBuilder {
	var newLine string
	if d.o.Color {
		newLine = fmt.Sprintf("%s%s %s\n", d.o.Prefix, ansi.Color("+", "green"), ansi.Color(str, "green"))
	} else {
		newLine = fmt.Sprintf("%s%s %s\n", d.o.Prefix, "+", str)
	}
	_, err := d.sb.WriteString(newLine)
	assert.NoError(d.t, err)
	return d
}

func (d *DiffOutBuilder) Out() string {
	return d.sb.String()
}
