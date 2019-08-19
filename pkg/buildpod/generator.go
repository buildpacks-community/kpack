package buildpod

import (
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type Generator struct {
	BuildPodConfig v1alpha1.BuildPodConfig
	K8sClient      k8sclient.Interface
}

func (g *Generator) Generate(build *v1alpha1.Build) (*v1.Pod, error) {
	secrets, err := g.getBuildSecrets(build)
	if err != nil {
		return nil, err
	}
	return build.BuildPod(g.BuildPodConfig, secrets)
}

func (g *Generator) getBuildSecrets(build *v1alpha1.Build) ([]corev1.Secret, error) {
	var secrets []corev1.Secret
	serviceAccount, err := g.K8sClient.CoreV1().ServiceAccounts(build.Namespace()).Get(build.ServiceAccount(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, secretRef := range serviceAccount.Secrets {
		secret, err := g.K8sClient.CoreV1().Secrets(build.Namespace()).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}
	return secrets, nil
}
