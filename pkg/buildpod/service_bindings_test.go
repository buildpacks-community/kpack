package buildpod_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/buildpod"
)

func TestServiceBindings(t *testing.T) {
	spec.Run(t, "ServiceBindings", testServiceBindings)
}

func testServiceBindings(t *testing.T, when spec.G, it spec.S) {
	when("ServiceBindings", func() {
		it("implements the k8s service bindings spec app projection", func() {
			sb := buildpod.ServiceBindings{
				{Name: "some-binding", SecretRef: &corev1.LocalObjectReference{Name: "some-secret"}},
			}

			expectedVols := []corev1.Volume{
				{
					Name: "service-binding-secret-some-binding",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "some-secret",
						},
					},
				},
			}
			expectedVolMounts := []corev1.VolumeMount{
				{
					Name:      "service-binding-secret-some-binding",
					MountPath: "/some-dir/bindings/some-binding",
					ReadOnly:  true,
				},
			}

			vols, volMounts := sb.AppProjections("/some-dir")

			require.Equal(t, expectedVols, vols)
			require.Equal(t, expectedVolMounts, volMounts)
		})
	})

	when("V1Alpha1ServiceBindings", func() {
		it("implements the cnb service bindings spec", func() {
			sb := buildpod.V1Alpha1ServiceBindings{
				{
					Name:        "some-binding",
					SecretRef:   &corev1.LocalObjectReference{Name: "some-secret"},
					MetadataRef: &corev1.LocalObjectReference{Name: "some-meta"},
				},
			}

			expectedVols := []corev1.Volume{
				{
					Name: "binding-metadata-some-binding",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "some-meta",
							},
						},
					},
				},
				{
					Name: "binding-secret-some-binding",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "some-secret",
						},
					},
				},
			}
			expectedVolMounts := []corev1.VolumeMount{
				{
					Name:      "binding-metadata-some-binding",
					MountPath: "/some-dir/bindings/some-binding/metadata",
					ReadOnly:  true,
				},
				{
					Name:      "binding-secret-some-binding",
					MountPath: "/some-dir/bindings/some-binding/secret",
					ReadOnly:  true,
				},
			}

			vols, volMounts := sb.AppProjections("/some-dir")

			require.Equal(t, expectedVols, vols)
			require.Equal(t, expectedVolMounts, volMounts)
		})
	})
}
