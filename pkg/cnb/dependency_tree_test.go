package cnb_test

import (
	"testing"

	"github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
)

func TestDependencyTree(t *testing.T) {
	spec.Run(t, "Dependency Tree", testNewTree)
}

func testNewTree(t *testing.T, when spec.G, it spec.S) {
	when("there is 1 root tree", func() {
		it("builds the tree", func() {
			buildpacks := []v1alpha1.BuildpackStatus{
				{
					BuildpackInfo: v1alpha1.BuildpackInfo{Id: "parent-buildpack"},
					Order: []v1alpha1.OrderEntry{
						{
							Group: []v1alpha1.BuildpackRef{
								{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-1"}},
								{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-2"}},
							},
						},
						{
							Group: []v1alpha1.BuildpackRef{
								{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-3"}},
							},
						},
					},
				},
				{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-1"}},
				{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-2"}},
				{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-3"}},
			}
			tree := cnb.NewTree(buildpacks)
			assert.Len(t, tree, 1)
			assert.NotNil(t, tree[0].Buildpack)
			assert.Equal(t, tree[0].Buildpack.Id, "parent-buildpack")
			assert.Len(t, tree[0].Children, 3)
		})

		it("handles nested children", func() {
			buildpacks := []v1alpha1.BuildpackStatus{
				{
					BuildpackInfo: v1alpha1.BuildpackInfo{Id: "parent-buildpack"},
					Order: []v1alpha1.OrderEntry{{
						Group: []v1alpha1.BuildpackRef{{
							BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-1"}},
						}},
					}},
				{
					BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-1"},
					Order: []v1alpha1.OrderEntry{{
						Group: []v1alpha1.BuildpackRef{
							{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "grandchild-buildpack-1"}},
						},
					}},
				},
				{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "grandchild-buildpack-1"}},
			}

			tree := cnb.NewTree(buildpacks)
			assert.Len(t, tree, 1)
			assert.Equal(t, tree[0].Buildpack.Id, "parent-buildpack")
			assert.Len(t, tree[0].Children[0].Children, 1)
		})
	})

	when("There is multiple root tree", func() {
		it("builds the tree", func() {
			buildpacks := []v1alpha1.BuildpackStatus{
				{
					BuildpackInfo: v1alpha1.BuildpackInfo{Id: "parent-buildpack-1"},
					Order: []v1alpha1.OrderEntry{{
						Group: []v1alpha1.BuildpackRef{{
							BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-1"}},
						}},
					}},
				{
					BuildpackInfo: v1alpha1.BuildpackInfo{Id: "parent-buildpack-2"},
				},
				{BuildpackInfo: v1alpha1.BuildpackInfo{Id: "child-buildpack-1"}},
			}

			tree := cnb.NewTree(buildpacks)
			assert.Len(t, tree, 2)
		})
	})
}
