package cnb

import (
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type Node struct {
	Buildpack *corev1alpha1.BuildpackStatus
	Children  []*Node
}

// NewTree generates a list of dependency trees for the given buildpacks. A
// buildpack's dependency tree is just the buildpack's group[].order[] but in
// proper tree form.
func NewTree(buildpacks []corev1alpha1.BuildpackStatus) []*Node {
	lookup := make(map[string]*corev1alpha1.BuildpackStatus)
	for i := range buildpacks {
		// explictly create a new var here, since using pointers in for-loops gets nasty
		bp := buildpacks[i]
		lookup[bp.Id] = &bp
	}

	usedBuildpacks := make(map[string]bool)
	for _, bp := range buildpacks {
		for _, order := range bp.Order {
			for _, status := range order.Group {
				usedBuildpacks[status.Id] = true
			}
		}
	}

	var unusedBuildpacks []string
	for _, bp := range buildpacks {
		if _, found := usedBuildpacks[bp.Id]; !found {
			unusedBuildpacks = append(unusedBuildpacks, bp.Id)
		}
	}

	trees := make([]*Node, len(unusedBuildpacks))
	for i, id := range unusedBuildpacks {
		trees[i] = makeTree(lookup, id)
	}

	return trees
}

func makeTree(lookup map[string]*corev1alpha1.BuildpackStatus, id string) *Node {
	bp := lookup[id]

	var children []*Node
	for _, order := range bp.Order {
		for _, status := range order.Group {
			children = append(children, makeTree(lookup, status.Id))
		}
	}

	return &Node{
		Buildpack: bp,
		Children:  children,
	}
}
