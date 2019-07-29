package testhelpers

import (
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/secret"
)

func SaveGitSecrets(client k8sclient.Interface, namespace, serviceAccount string, users []secret.URLAndUser) error {
	return saveSecrets(client, namespace, serviceAccount, users, v1alpha1.GITSecretAnnotationPrefix)
}

func SaveDockerSecrets(client k8sclient.Interface, namespace, serviceAccount string, users []secret.URLAndUser) error {
	return saveSecrets(client, namespace, serviceAccount, users, v1alpha1.DOCKERSecretAnnotationPrefix)
}

func saveSecrets(client k8sclient.Interface, namespace, serviceAccount string, users []secret.URLAndUser, annotationKey string) error {
	secrets := []v1.ObjectReference{}

	for _, user := range users {
		secret, err := client.CoreV1().Secrets(namespace).Create(&v1.Secret{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: string(uuid.NewUUID()),
				Annotations: map[string]string{
					annotationKey: user.URL,
				},
			},
			Data: map[string][]byte{
				"username": []byte(user.Username),
				"password": []byte(user.Password),
			},
			Type: v1.SecretTypeBasicAuth,
		})
		if err != nil {
			return err
		}

		secrets = append(secrets, v1.ObjectReference{
			Name: secret.Name,
		})
	}

	_, err := client.CoreV1().ServiceAccounts(namespace).Create(&v1.ServiceAccount{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: serviceAccount,
		},
		Secrets: secrets,
	})
	return err
}
