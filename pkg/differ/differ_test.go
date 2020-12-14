package differ_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"

	"github.com/pivotal/kpack/pkg/differ"
)

func TestDiffer(t *testing.T) {
	spec.Run(t, "TestDiffer", testDiffer)
}

func testDiffer(t *testing.T, when spec.G, it spec.S) {
	type obj struct {
		Foo string
		Bar int
		Baz string
	}

	it("returns the yaml diff of two objects with red and green coloring", func() {
		o := obj{
			Foo: "Old",
			Bar: 1,
			Baz: "Foo",
		}
		n := obj{
			Foo: "New",
			Bar: 2,
			Baz: "Foo",
		}
		diff, err := differ.Diff(o, n)
		require.NoError(t, err)
		expected := "\x1b[31m-\x1b[0m \x1b[31mBar: 1\x1b[0m\n\x1b[32m+\x1b[0m \x1b[32mBar: 2\x1b[0m\nBaz: Foo\n\x1b[31m-\x1b[0m \x1b[31mFoo: Old\x1b[0m\n\x1b[32m+\x1b[0m \x1b[32mFoo: New\x1b[0m\n"
		require.Equal(t, expected, diff)
	})

	it("returns all green if old obj is nil", func() {
		n := obj{
			Foo: "New",
			Bar: 2,
			Baz: "Foo",
		}
		diff, err := differ.Diff(nil, n)
		require.NoError(t, err)
		expected := "\x1b[32m+\x1b[0m \x1b[32mBar: 2\x1b[0m\n\x1b[32m+\x1b[0m \x1b[32mBaz: Foo\x1b[0m\n\x1b[32m+\x1b[0m \x1b[32mFoo: New\x1b[0m\n"
		require.Equal(t, expected, diff)
	})

	it("returns all red if new obj is nil", func() {
		o := obj{
			Foo: "Old",
			Bar: 1,
			Baz: "Foo",
		}
		diff, err := differ.Diff(o, nil)
		require.NoError(t, err)
		expected := "\x1b[31m-\x1b[0m \x1b[31mBar: 1\x1b[0m\n\x1b[31m-\x1b[0m \x1b[31mBaz: Foo\x1b[0m\n\x1b[31m-\x1b[0m \x1b[31mFoo: Old\x1b[0m\n"
		require.Equal(t, expected, diff)
	})

	it("uses the string a one is given", func() {
		o := `OldFoo
Same
OldBar
`

		n := `NewFoo
Same
NewBar
`
		diff, err := differ.Diff(o, n)
		require.NoError(t, err)
		expected := "\x1b[31m-\x1b[0m \x1b[31mOldFoo\x1b[0m\n\x1b[32m+\x1b[0m \x1b[32mNewFoo\x1b[0m\nSame\n\x1b[31m-\x1b[0m \x1b[31mOldBar\x1b[0m\n\x1b[32m+\x1b[0m \x1b[32mNewBar\x1b[0m\n"
		require.Equal(t, expected, diff)
	})
}
