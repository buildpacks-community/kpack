package buildpod

import (
	"strconv"

	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/registry"
)

type Generator struct {
	BuildPodConfig     v1alpha1.BuildPodConfig
	K8sClient          k8sclient.Interface
	RemoteImageFactory registry.RemoteImageFactory
}

func (g *Generator) Generate(build *v1alpha1.Build) (*v1.Pod, error) {
	secrets, err := g.fetchBuildSecrets(build)
	if err != nil {
		return nil, err
	}

	userAndGroup, err := g.fetchUserAndGroup(build)
	if err != nil {
		return nil, err
	}

	return build.BuildPod(g.BuildPodConfig, secrets, build.Spec.Builder, userAndGroup)
}

func (g *Generator) fetchBuildSecrets(build *v1alpha1.Build) ([]corev1.Secret, error) {
	var secrets []corev1.Secret
	serviceAccount, err := g.K8sClient.CoreV1().ServiceAccounts(build.Namespace).Get(build.Spec.ServiceAccount, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, secretRef := range serviceAccount.Secrets {
		secret, err := g.K8sClient.CoreV1().Secrets(build.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}
	return secrets, nil
}

const cnbUserId = "CNB_USER_ID"
const cnbGroupId = "CNB_GROUP_ID"

func (g *Generator) fetchUserAndGroup(build *v1alpha1.Build) (v1alpha1.UserAndGroup, error) {
	image, err := g.RemoteImageFactory.NewRemote(build.Spec.Builder.Image, registry.SecretRef{
		Namespace:        build.Namespace,
		ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
	})
	if err != nil {
		return v1alpha1.UserAndGroup{}, err
	}

	uid, err := parseCNBID(image, cnbUserId)
	if err != nil {
		return v1alpha1.UserAndGroup{}, err
	}

	gid, err := parseCNBID(image, cnbGroupId)
	if err != nil {
		return v1alpha1.UserAndGroup{}, err
	}

	return v1alpha1.UserAndGroup{
		Uid: uid,
		Gid: gid,
	}, nil
}

func parseCNBID(image registry.RemoteImage, env string) (int64, error) {
	v, err := image.Env(env)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}
