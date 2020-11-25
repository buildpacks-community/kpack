package differ

import (
	"fmt"
	"strings"

	"github.com/aryann/difflib"
	"github.com/ghodss/yaml"
	"github.com/mgutz/ansi"
)

type Differ struct {
	prefix string
	color  bool
}

type Options struct {
	Prefix string
	Color  bool
}

func NewDiffer(options Options) Differ{
	return Differ {
		prefix: options.Prefix,
		color: options.Color,
	}
}

func Diff(dOld, dNew interface{}) (string, error) {
	return Differ{}.Diff(dOld, dNew)
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

		sBuilder.WriteString(d.prefix)

		switch diff.Delta {
		case difflib.RightOnly:
			if d.color {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", ansi.Color("+", "green"), ansi.Color(text, "green")))
			} else {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", "+", text))
			}
		case difflib.LeftOnly:
			if d.color {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", ansi.Color("-", "red"), ansi.Color(text, "red")))
			} else {
				sBuilder.WriteString(fmt.Sprintf("%s %s\n", "-", text))
			}
		case difflib.Common:
			sBuilder.WriteString(fmt.Sprintf("%s\n", text))
		}
	}
	return sBuilder.String()
}
