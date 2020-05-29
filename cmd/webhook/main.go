package main

import (
	"context"
	"log"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1 "k8s.io/client-go/informers/storage/v1"
	"knative.dev/pkg/client/injection/kube/informers/factory"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
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

func init() {
	injection.Default.RegisterInformer(withStorageClassInformer)
}

func main() {
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: "kpack-webhook",
		Port:        8443,
		SecretName:  "webhook-certs",
	})

	sharedmain.WebhookMainWithConfig(ctx, "webhook",
		sharedmain.ParseAndGetConfigOrDie(),
		certificates.NewController,
		defaultingAdmissionController,
		validatingAdmissionController,
	)
}

func defaultingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	storageClassLister := getStorageClassInformer(ctx).Lister()

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

// storageClassInformerKey is used for associating the Informer inside the context.Context.
type storageClassInformerKey struct{}

func withStorageClassInformer(ctx context.Context) (context.Context, controller.Informer) {
	f := factory.Get(ctx)
	inf := f.Storage().V1().StorageClasses()
	return context.WithValue(ctx, storageClassInformerKey{}, inf), inf.Informer()
}

func getStorageClassInformer(ctx context.Context) v1.StorageClassInformer {
	untyped := ctx.Value(storageClassInformerKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic("Unable to storage class informer from context.")
	}
	return untyped.(v1.StorageClassInformer)
}
