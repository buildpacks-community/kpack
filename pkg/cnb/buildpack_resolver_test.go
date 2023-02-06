package cnb

import (
	"testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry/registryfakes"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildpackResolver(t *testing.T) {
	spec.Run(t, "TestBuildpackResolver", testBuildpackResolver)
}

func testBuildpackResolver(t *testing.T, when spec.G, it spec.S) {
	var (
		keychainFactory = &registryfakes.FakeKeychainFactory{}
		testNamespace   = "some-namespace"

		engineBuildpack = corev1alpha1.BuildpackStatus{
			BuildpackInfo: corev1alpha1.BuildpackInfo{
				Id:      "io.buildpack.engine",
				Version: "1.0.0",
			},
			DiffId: "sha256:1bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:d345d1b12ae6b3f7cfc617f7adaebe06c32ce60b1aa30bb80fb622b65523de8f",
			Size:   50,
			StoreImage: corev1alpha1.ImageSource{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.engine.com",
			API:      "0.1",
			Stacks: []corev1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.engine.works",
				},
			},
		}

		packageManagerBuildpack = corev1alpha1.BuildpackStatus{
			BuildpackInfo: corev1alpha1.BuildpackInfo{
				Id:      "io.buildpack.package-manager",
				Version: "1.0.0",
			},
			DiffId: "sha256:2bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:7c1213a54d20137a7479e72150c058268a6604b98c011b4fc11ca45927923d7b",
			Size:   40,
			StoreImage: corev1alpha1.ImageSource{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.package-manager.com",
			API:      "0.2",
			Stacks: []corev1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.package.works",
				},
			},
		}

		metaBuildpack = corev1alpha1.BuildpackStatus{
			BuildpackInfo: corev1alpha1.BuildpackInfo{
				Id:      "io.buildpack.meta",
				Version: "1.0.0",
			},
			DiffId: "sha256:3bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:07db84e57fdd7101104c2469984217696fdfe51591cb1edee2928514135920d6",
			Size:   30,
			StoreImage: corev1alpha1.ImageSource{
				Image: "some.registry.io/build-package",
			},
			Order: []corev1alpha1.OrderEntry{
				{
					Group: []corev1alpha1.BuildpackRef{
						{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id:      "io.buildpack.engine",
								Version: "1.0.0",
							},
							Optional: false,
						},
						{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id:      "io.buildpack.package-manager",
								Version: "1.0.0",
							},
							Optional: true,
						},
					},
				},
			},
			Homepage: "buildpack.meta.com",
			API:      "0.3",
			Stacks: []corev1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.meta.works",
				},
			},
		}

		v8Buildpack = corev1alpha1.BuildpackStatus{
			BuildpackInfo: corev1alpha1.BuildpackInfo{
				Id:      "io.buildpack.multi",
				Version: "8.0.0",
			},
			DiffId: "sha256:8bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:fc14806eb95d01b6338ba1b9fea605e84db7c8c09561ae360bad5b80b5d0d80b",
			Size:   20,
			StoreImage: corev1alpha1.ImageSource{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.multi.com",
			API:      "0.2",
			Stacks: []corev1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.v8.works",
				},
			},
		}

		v9Buildpack = corev1alpha1.BuildpackStatus{
			BuildpackInfo: corev1alpha1.BuildpackInfo{
				Id:      "io.buildpack.multi",
				Version: "9.0.0",
			},
			DiffId: "sha256:9bf8899667b8d1e6b124f663faca32903b470831e5e4e992644ac5c839ab3462",
			Digest: "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
			Size:   10,
			StoreImage: corev1alpha1.ImageSource{
				Image: "some.registry.io/build-package",
			},
			Order:    nil,
			Homepage: "buildpack.multi.com",
			API:      "0.2",
			Stacks: []corev1alpha1.BuildpackStack{
				{
					ID: "io.custom.stack",
				},
				{
					ID: "io.stack.only.v9.works",
				},
			},
		}
	)

	when("Resolve", func() {
		when("using the clusterstore", func() {
			var (
				resolver BuildpackResolver
				store    = &buildapi.ClusterStore{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-store",
					},
					Spec: buildapi.ClusterStoreSpec{},
					Status: buildapi.ClusterStoreStatus{
						Buildpacks: []corev1alpha1.BuildpackStatus{
							metaBuildpack,
							engineBuildpack,
							packageManagerBuildpack,
							v8Buildpack,
							v9Buildpack,
						},
					},
				}
			)

			it.Before(func() {
				resolver = NewBuildpackResolver(keychainFactory, store, nil, nil)
			})

			it("finds it using id", func() {
				ref := makeRef("io.buildpack.engine", "")
				expectedBuildpack := engineBuildpack

				buildpack, err := resolver.Resolve(ref)
				assert.Nil(t, err)
				assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
			})

			it("finds it using id and version", func() {
				ref := makeRef("io.buildpack.multi", "8.0.0")
				expectedBuildpack := v8Buildpack

				buildpack, err := resolver.Resolve(ref)
				assert.Nil(t, err)
				assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
			})

			it("fails on invalid id", func() {
				ref := makeRef("fake-buildpack", "")
				_, err := resolver.Resolve(ref)
				assert.EqualError(t, err, "could not find buildpack with id 'fake-buildpack'")
			})

			it("fails on unknown version", func() {
				ref := makeRef("io.buildpack.multi", "8.0.1")
				_, err := resolver.Resolve(ref)
				assert.EqualError(t, err, "could not find buildpack with id 'io.buildpack.multi' and version '8.0.1'")
			})
		})

		when("using the buildpack resources", func() {
			var (
				resolver   BuildpackResolver
				buildpacks = []*buildapi.Buildpack{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.meta",
							Namespace: testNamespace,
						},
						Status: buildapi.BuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								metaBuildpack,
								engineBuildpack,
								packageManagerBuildpack,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.multi-8.0.0",
							Namespace: testNamespace,
						},
						Status: buildapi.BuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								v8Buildpack,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.multi-9.0.0",
							Namespace: testNamespace,
						},
						Status: buildapi.BuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								v9Buildpack,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.multi",
							Namespace: testNamespace,
						},
						Status: buildapi.BuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								v8Buildpack,
								v9Buildpack,
							},
						},
					},
				}
			)

			it.Before(func() {
				resolver = NewBuildpackResolver(keychainFactory, nil, buildpacks, nil)
			})

			when("using id", func() {
				it("finds it using id", func() {
					ref := makeRef("io.buildpack.meta", "")
					expectedBuildpack := metaBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds nested ids", func() {
					ref := makeRef("io.buildpack.engine", "")
					expectedBuildpack := engineBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds it using id and version", func() {
					ref := makeRef("io.buildpack.multi", "8.0.0")
					expectedBuildpack := v8Buildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("fails on invalid id", func() {
					ref := makeRef("fake-buildpack", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'fake-buildpack'")
				})

				it("fails on unknown version", func() {
					ref := makeRef("io.buildpack.multi", "8.0.1")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'io.buildpack.multi' and version '8.0.1'")
				})
			})

			when("using object ref", func() {
				it("finds the resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.BuildpackKind, "", "")
					expectedBuildpack := metaBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("fails on invalid kind", func() {
					ref := makeObjectRef("io.buildpack.meta", "FakeBuildpack", "", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "kind must be either Buildpack or ClusterBuildpack")
				})

				it("fails on object not found", func() {
					ref := makeObjectRef("fake-buildpack", buildapi.BuildpackKind, "", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "no buildpack with name 'fake-buildpack'")
				})
			})

			when("using id and object ref together", func() {
				it("finds id in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.BuildpackKind, "io.buildpack.meta", "")
					expectedBuildpack := metaBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds nested id in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.BuildpackKind, "io.buildpack.engine", "")
					expectedBuildpack := engineBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds the correct version in resource", func() {
					ref := makeObjectRef("io.buildpack.multi", buildapi.BuildpackKind, "io.buildpack.multi", "8.0.0")
					expectedBuildpack := v8Buildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("fails on id not found in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.BuildpackKind, "fake-buildpack", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'fake-buildpack'")
				})

				it("fails on version not found in resource", func() {
					ref := makeObjectRef("io.buildpack.multi", buildapi.BuildpackKind, "io.buildpack.multi", "8.0.1")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'io.buildpack.multi' and version '8.0.1'")
				})

				it("fails on id not found in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.BuildpackKind, "fake-buildpack", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'fake-buildpack'")
				})
			})
		})

		when("using the clusterbuildpack resources", func() {
			var (
				resolver          BuildpackResolver
				clusterBuildpacks = []*buildapi.ClusterBuildpack{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.meta",
							Namespace: testNamespace,
						},
						Status: buildapi.ClusterBuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								metaBuildpack,
								engineBuildpack,
								packageManagerBuildpack,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.multi-8.0.0",
							Namespace: testNamespace,
						},
						Status: buildapi.ClusterBuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								v8Buildpack,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.multi-9.0.0",
							Namespace: testNamespace,
						},
						Status: buildapi.ClusterBuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								v9Buildpack,
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "io.buildpack.multi",
							Namespace: testNamespace,
						},
						Status: buildapi.ClusterBuildpackStatus{
							Buildpacks: []corev1alpha1.BuildpackStatus{
								v8Buildpack,
								v9Buildpack,
							},
						},
					},
				}
			)

			it.Before(func() {
				resolver = NewBuildpackResolver(keychainFactory, nil, nil, clusterBuildpacks)
			})

			when("using id", func() {
				it("finds it using id", func() {
					ref := makeRef("io.buildpack.meta", "")
					expectedBuildpack := metaBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds nested ids", func() {
					ref := makeRef("io.buildpack.engine", "")
					expectedBuildpack := engineBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds it using id and version", func() {
					ref := makeRef("io.buildpack.multi", "8.0.0")
					expectedBuildpack := v8Buildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("fails on invalid id", func() {
					ref := makeRef("fake-buildpack", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'fake-buildpack'")
				})

				it("fails on unknown version", func() {
					ref := makeRef("io.buildpack.multi", "8.0.1")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'io.buildpack.multi' and version '8.0.1'")
				})
			})

			when("using object ref", func() {
				it("finds the resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.ClusterBuildpackKind, "", "")
					expectedBuildpack := metaBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("fails on invalid kind", func() {
					ref := makeObjectRef("io.buildpack.meta", "FakeClusterBuildpack", "", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "kind must be either Buildpack or ClusterBuildpack")
				})

				it("fails on object not found", func() {
					ref := makeObjectRef("fake-buildpack", buildapi.ClusterBuildpackKind, "", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "no cluster buildpack with name 'fake-buildpack'")
				})
			})

			when("using id and object ref together", func() {
				it("finds id in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.ClusterBuildpackKind, "io.buildpack.meta", "")
					expectedBuildpack := metaBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds nested id in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.ClusterBuildpackKind, "io.buildpack.engine", "")
					expectedBuildpack := engineBuildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("finds the correct version in resource", func() {
					ref := makeObjectRef("io.buildpack.multi", buildapi.ClusterBuildpackKind, "io.buildpack.multi", "8.0.0")
					expectedBuildpack := v8Buildpack

					buildpack, err := resolver.Resolve(ref)
					assert.Nil(t, err)
					assert.Equal(t, expectedBuildpack, buildpack.Buildpack)
				})

				it("fails on id not found in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.ClusterBuildpackKind, "fake-buildpack", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'fake-buildpack'")
				})

				it("fails on version not found in resource", func() {
					ref := makeObjectRef("io.buildpack.multi", buildapi.ClusterBuildpackKind, "io.buildpack.multi", "8.0.1")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'io.buildpack.multi' and version '8.0.1'")
				})

				it("fails on id not found in resource", func() {
					ref := makeObjectRef("io.buildpack.meta", buildapi.ClusterBuildpackKind, "fake-buildpack", "")
					_, err := resolver.Resolve(ref)
					assert.EqualError(t, err, "could not find buildpack with id 'fake-buildpack'")
				})
			})
		})

		// when("resolving via image", func() {
		// })
	})
}
