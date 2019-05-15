package main

import (
	"flag"
	"github.com/pivotal/build-service-system/pkg/registry"
	"log"
	"os"
	"path/filepath"
	"time"

	knversioned "github.com/knative/build/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	knexternalversions "github.com/knative/build/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/cnbbuild"
)

const (
	routinesPerController = 2
)

func main() {
	devLogger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Couldn't create logger: %s", err)
	}
	logger := devLogger.Sugar()

	clusterConfig, err := retrieveLocalConfiguration()

	buildClient, err := versioned.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get CNBBuild client: %s", err.Error())
	}

	knbuildClient, err := knversioned.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get Knative Build client: %s", err.Error())
	}

	k8sClient, err := v1.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err.Error())
	}

	stopChan := make(chan struct{})

	options := reconciler.Options{
		Logger:         logger,
		CNBBuildClient: buildClient,
		StopChannel:    stopChan,
	}

	cnbBuildInformerFactory := externalversions.NewSharedInformerFactory(buildClient, 10*time.Hour)
	cnbbuildInformer := cnbBuildInformerFactory.Build().V1alpha1().CNBBuilds()

	knBuildInformerFactory := knexternalversions.NewSharedInformerFactory(knbuildClient, 10*time.Hour)
	knBuildInformer := knBuildInformerFactory.Build().V1alpha1().Builds()

	metadataRetriever := &registry.RemoteMetadataRetriever{
		LifecycleImageFactory: &registry.ImageFactory{
			KeychainFactory: &registry.SecretKeychainFactory{
				SecretManager: &registry.SecretManager{
					Client: k8sClient,
				},
			},
		},
	}

	ctrlr := cnbbuild.NewController(options, knbuildClient, cnbbuildInformer, knBuildInformer, metadataRetriever)

	cnbBuildInformerFactory.Start(stopChan)
	knBuildInformerFactory.Start(stopChan)

	cache.WaitForCacheSync(stopChan, cnbbuildInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, knBuildInformer.Informer().HasSynced)

	if runErr := ctrlr.Run(routinesPerController, stopChan); runErr != nil {
		logger.Fatalw("Error running controller", zap.Error(runErr))
	}
}

func retrieveLocalConfiguration() (*rest.Config, error) {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	// use the current context in kubeconfig
	clusterConfig, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return nil, err
	}
	return clusterConfig, err
}

func homeDir() string {
	return os.Getenv("HOME")
}
