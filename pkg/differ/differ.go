package differ

import (
	"fmt"
	"strings"

	"github.com/aryann/difflib"
	"github.com/ghodss/yaml"
	"github.com/mgutz/ansi"
)

type Differ struct {
	o Options
}

type Options struct {
	Prefix string
	Color  bool
	Common bool
}

func DefaultOptions() Options {
	return Options{
		Prefix: "",
		Color:  true,
		Common: true,
	}
}

func NewDiffer(options Options) Differ {
	return Differ{o: options}
}

func Diff(dOld, dNew interface{}) (string, error) {
	return NewDiffer(DefaultOptions()).Diff(dOld, dNew)
}

func (d Differ) Configure(options Options) {
	d.o = options
}

func (d Differ) Diff(dOld, dNew interface{}) (string, error) {
	dataOld, err := d.getData(dOld)
	if err != nil {
		return "", err
	}

	dataNew, err := d.getData(dNew)
	if err != nil {
		return "", err
	}

	if dataOld == dataNew {
		return "", nil
	}

	return d.renderDiff(dataOld, dataNew), nil
}

func (d Differ) getData(obj interface{}) (string, error) {
	if obj == nil {
		return "", nil
	}
	if str, ok := obj.(string); ok {
		return str, nil
	}
	data, err := yaml.Marshal(obj)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (d Differ) renderDiff(a, b string) string {
	diffs := difflib.Diff(strings.Split(a, "\n"), strings.Split(b, "\n"))

	sBuilder := strings.Builder{}
	for _, diff := range diffs {
		text := diff.Payload
		if text == "" {
			continue
		}

		sBuilder.WriteString(d.o.Prefix)

		switch diff.Delta {
		case difflib.RightOnly:
			if d.o.Color {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", ansi.Color("+", "green"), ansi.Color(text, "green")))
			} else {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", "+", text))
			}
		case difflib.LeftOnly:
			if d.o.Color {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", ansi.Color("-", "red"), ansi.Color(text, "red")))
			} else {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", "-", text))
			}
		case difflib.Common:
			if d.o.Common {
				sBuilder.WriteString(fmt.Sprintf("%s\n", text))
			}
		}
	}
	return sBuilder.String()
}
