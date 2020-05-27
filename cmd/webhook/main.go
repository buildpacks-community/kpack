package main

import (
	"context"
	"log"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/storage/v1"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	v1alpha1.SchemeGroupVersion.WithKind("Image"):                &v1alpha1.Image{},
	v1alpha1.SchemeGroupVersion.WithKind("Build"):                &v1alpha1.Build{},
	v1alpha1.SchemeGroupVersion.WithKind("Builder"):              &v1alpha1.Builder{},
	v1alpha1.SchemeGroupVersion.WithKind("ClusterBuilder"):       &v1alpha1.ClusterBuilder{},
	v1alpha1.SchemeGroupVersion.WithKind("CustomBuilder"):        &expv1alpha1.CustomBuilder{},
	v1alpha1.SchemeGroupVersion.WithKind("CustomClusterBuilder"): &expv1alpha1.CustomClusterBuilder{},
	v1alpha1.SchemeGroupVersion.WithKind("Store"):                &expv1alpha1.Store{},
	v1alpha1.SchemeGroupVersion.WithKind("Stack"):                &expv1alpha1.Stack{},
}

var (
	storageClassLister v1.StorageClassLister
)

func main() {
	restConfig := sharedmain.ParseAndGetConfigOrDie()

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err)
	}

	k8sInformerFactory := informers.NewSharedInformerFactory(k8sClient, 10*time.Hour)
	storageClassLister = k8sInformerFactory.Storage().V1().StorageClasses().Lister()

	stopChan := make(chan struct{})
	k8sInformerFactory.Start(stopChan)
	k8sInformerFactory.WaitForCacheSync(stopChan)

	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "kpack-webhook",
		Port:        8443,
		SecretName:  "webhook-certs",
	})

	sharedmain.WebhookMainWithConfig(ctx, "webhook",
		restConfig,
		certificates.NewController,
		defaultingAdmissionController,
		validatingAdmissionController,
	)
}

func defaultingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"defaults.webhook.kpack.pivotal.io",
		// The path on which to serve the webhook.
		"/defaults",
		// The resources to default.
		types,
		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			storageClasses, err := storageClassLister.List(labels.NewSelector())
			if err != nil {
				log.Printf("failed to list storage classes: %s\n", err)
				return ctx
			}

			for _, sc := range storageClasses {
				if sc.Annotations == nil {
					continue
				}

				if val, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && val == "true" {
					ctx = context.WithValue(ctx, v1alpha1.HasDefaultStorageClass, true)
					break
				}
			}

			return ctx
		},
		// Whether to disallow unknown fields.
		false,
	)
}

func validatingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	return validation.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"validation.webhook.kpack.pivotal.io",
		// The path on which to serve the webhook.
		"/validate",
		// The resources to default.
		types,
		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},
		// Whether to disallow unknown fields.
		true,
	)
}
