package main

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
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

func defaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	return defaulting.NewAdmissionController(ctx,
		// Name of the resource webhook.
		"defaults.webhook.kpack.pivotal.io",
		// The path on which to serve the webhook.
		"/defaults",
		// The resources to default.
		types,
		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		func(ctx context.Context) context.Context {
			return ctx
		},
		// Whether to disallow unknown fields.
		false,
	)
}

func validatingAdmissionController(ctx context.Context, watcher configmap.Watcher) *controller.Impl {
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
		false,
	)
}
