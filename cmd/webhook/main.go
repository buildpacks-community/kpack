package main

import (
	"flag"
	"log"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/system"
	"knative.dev/pkg/webhook"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

func main() {
	flag.Parse()
	devLogger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Couldn't create logger: %s", err)
	}
	logger := devLogger.Sugar()

	clusterConfig, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	client, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err.Error())
	}

	options := webhook.ControllerOptions{
		ServiceName:                     "kpack-webhook",
		DeploymentName:                  "kpack-webhook",
		Namespace:                       system.Namespace(),
		Port:                            8443,
		SecretName:                      "webhook-certs",
		ResourceMutatingWebhookName:     "resource.webhook.kpack.pivotal.io",
		ResourceAdmissionControllerPath: "/",
		ConfigValidationWebhookName:     "config.webhook.kpack.pivotal.io",
		ConfigValidationControllerPath:  "/config-validation",
		ConfigValidationNamespaceLabel:  "kpack.pivotal.io/release",
	}

	resourceHandlers := map[schema.GroupVersionKind]webhook.GenericCRD{
		v1alpha1.SchemeGroupVersion.WithKind("Image"):          &v1alpha1.Image{},
		v1alpha1.SchemeGroupVersion.WithKind("Build"):          &v1alpha1.Build{},
		v1alpha1.SchemeGroupVersion.WithKind("Builder"):        &v1alpha1.Builder{},
		v1alpha1.SchemeGroupVersion.WithKind("ClusterBuilder"): &v1alpha1.ClusterBuilder{},
	}

	admissionControllers := map[string]webhook.AdmissionController{
		options.ResourceAdmissionControllerPath: webhook.NewResourceAdmissionController(resourceHandlers, options, false),
	}

	controller, err := webhook.New(client, options, admissionControllers, logger, nil)
	if err != nil {
		logger.Fatalw("Failed to create admission controller", zap.Error(err))
	}

	if err = controller.Run(signals.SetupSignalHandler()); err != nil {
		logger.Fatalw("Error running admission controller", zap.Error(err))
	}

	logger.Infow("Webhook stopping")
}
