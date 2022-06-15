package main

import (
	"context"
	"log"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	informersv1 "k8s.io/client-go/informers/storage/v1"
	listersv1 "k8s.io/client-go/listers/storage/v1"
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
	"knative.dev/pkg/webhook/resourcesemantics/conversion"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

var types = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	v1alpha2.SchemeGroupVersion.WithKind(v1alpha2.ImageKind):          &v1alpha2.Image{},
	v1alpha2.SchemeGroupVersion.WithKind(v1alpha2.BuildKind):          &v1alpha2.Build{},
	v1alpha2.SchemeGroupVersion.WithKind(v1alpha2.BuilderKind):        &v1alpha2.Builder{},
	v1alpha2.SchemeGroupVersion.WithKind(v1alpha2.ClusterBuilderKind): &v1alpha2.ClusterBuilder{},
	v1alpha2.SchemeGroupVersion.WithKind(v1alpha2.ClusterStoreKind):   &v1alpha2.ClusterStore{},
	v1alpha2.SchemeGroupVersion.WithKind(v1alpha2.ClusterStackKind):   &v1alpha2.ClusterStack{},
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
		injection.ParseAndGetRESTConfigOrDie(),
		certificates.NewController,
		defaultingAdmissionController,
		validatingAdmissionController,
		conversionController,
	)
}

func defaultingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	storageClassLister := getStorageClassInformer(ctx).Lister()

	return defaulting.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"defaults.webhook.kpack.io",
		// The path on which to serve the webhook.
		"/defaults",
		// The resources to default.
		types,
		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		withCheckDefaultStorageClass(storageClassLister),
		// Whether to disallow unknown fields.
		false,
	)
}

func validatingAdmissionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	storageClassLister := getStorageClassInformer(ctx).Lister()

	return validation.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"validation.webhook.kpack.io",
		// The path on which to serve the webhook.
		"/validate",
		// The resources to validate.
		types,
		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		withCheckDefaultStorageClass(storageClassLister),
		// Whether to disallow unknown fields.
		true,
	)
}

func conversionController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	storageClassLister := getStorageClassInformer(ctx).Lister()

	conversions := map[schema.GroupKind]conversion.GroupKindConversion{
		v1alpha2.Kind("Image"): {
			DefinitionName: "images.kpack.io",
			HubVersion:     v1alpha2.SchemeGroupVersion.Version,
			Zygotes: map[string]conversion.ConvertibleObject{
				v1alpha2.SchemeGroupVersion.Version: &v1alpha2.Image{},
				v1alpha1.SchemeGroupVersion.Version: &v1alpha1.Image{},
			},
		},
		v1alpha2.Kind("Build"): {
			DefinitionName: "builds.kpack.io",
			HubVersion:     v1alpha2.SchemeGroupVersion.Version,
			Zygotes: map[string]conversion.ConvertibleObject{
				v1alpha2.SchemeGroupVersion.Version: &v1alpha2.Build{},
				v1alpha1.SchemeGroupVersion.Version: &v1alpha1.Build{},
			},
		},
		v1alpha2.Kind("Builder"): {
			DefinitionName: "builders.kpack.io",
			HubVersion:     v1alpha2.SchemeGroupVersion.Version,
			Zygotes: map[string]conversion.ConvertibleObject{
				v1alpha2.SchemeGroupVersion.Version: &v1alpha2.Builder{},
				v1alpha1.SchemeGroupVersion.Version: &v1alpha1.Builder{},
			},
		},
		v1alpha2.Kind("SourceResolver"): {
			DefinitionName: "sourceresolvers.kpack.io",
			HubVersion:     v1alpha2.SchemeGroupVersion.Version,
			Zygotes: map[string]conversion.ConvertibleObject{
				v1alpha2.SchemeGroupVersion.Version: &v1alpha2.SourceResolver{},
				v1alpha1.SchemeGroupVersion.Version: &v1alpha1.SourceResolver{},
			},
		},
	}

	return conversion.NewConversionController(
		ctx,
		"/convert",
		conversions,
		withCheckDefaultStorageClass(storageClassLister),
	)
}

func withCheckDefaultStorageClass(storageClassLister listersv1.StorageClassLister) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
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
				ctx = context.WithValue(ctx, v1alpha2.HasDefaultStorageClass, true)
				if sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion {
					ctx = context.WithValue(ctx, v1alpha2.IsExpandable, true)
				}
				break
			}
		}

		return ctx
	}
}

// storageClassInformerKey is used for associating the Informer inside the context.Context.
type storageClassInformerKey struct{}

func withStorageClassInformer(ctx context.Context) (context.Context, controller.Informer) {
	f := factory.Get(ctx)
	inf := f.Storage().V1().StorageClasses()
	return context.WithValue(ctx, storageClassInformerKey{}, inf), inf.Informer()
}

func getStorageClassInformer(ctx context.Context) informersv1.StorageClassInformer {
	untyped := ctx.Value(storageClassInformerKey{})
	if untyped == nil {
		logging.FromContext(ctx).Panic("Unable to storage class informer from context.")
	}
	return untyped.(informersv1.StorageClassInformer)
}
