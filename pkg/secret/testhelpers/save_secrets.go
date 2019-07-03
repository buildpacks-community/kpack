package testhelpers

import (
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	v12 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/pivotal/build-service-system/pkg/secret"
)

func SaveSecrets(coreV1 v12.CoreV1Interface, namespace, serviceAccount string, users []secret.URLAndUser) error {
	secrets := []v1.ObjectReference{}

	for _, user := range users {
		secret, err := coreV1.Secrets(namespace).Create(&v1.Secret{
			ObjectMeta: meta_v1.ObjectMeta{
				Name: string(uuid.NewUUID()),
				Annotations: map[string]string{
					"build.knative.dev/git-0": user.URL,
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

	_, err := coreV1.ServiceAccounts(namespace).Create(&v1.ServiceAccount{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: serviceAccount,
		},
		Secrets: secrets,
	})
	return err
}
