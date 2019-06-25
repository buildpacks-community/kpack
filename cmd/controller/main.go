package main

import (
	"flag"
	"log"
	"sync"
	"time"

	knversioned "github.com/knative/build/pkg/client/clientset/versioned"
	knexternalversions "github.com/knative/build/pkg/client/informers/externalversions"
	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/builder"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/image"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	routinesPerController = 2
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
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

	client, err := versioned.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get Build client: %s", err.Error())
	}

	knbuildClient, err := knversioned.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get Knative Build client: %s", err.Error())
	}

	k8sClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err.Error())
	}

	options := reconciler.Options{
		Logger:       logger,
		Client:       client,
		ResyncPeriod: 10 * time.Hour,
	}

	informerFactory := externalversions.NewSharedInformerFactory(client, options.ResyncPeriod)
	buildInformer := informerFactory.Build().V1alpha1().Builds()
	imageInformer := informerFactory.Build().V1alpha1().Images()
	builderInformer := informerFactory.Build().V1alpha1().Builders()

	knBuildInformerFactory := knexternalversions.NewSharedInformerFactory(knbuildClient, options.ResyncPeriod)
	knBuildInformer := knBuildInformerFactory.Build().V1alpha1().Builds()

	k8sInformerFactory := informers.NewSharedInformerFactory(k8sClient, options.ResyncPeriod)
	pvcInformer := k8sInformerFactory.Core().V1().PersistentVolumeClaims()

	metadataRetriever := &registry.RemoteMetadataRetriever{
		LifecycleImageFactory: &registry.ImageFactory{
			KeychainFactory: &registry.SecretKeychainFactory{
				SecretManager: &registry.SecretManager{
					Client: k8sClient.CoreV1(),
				},
			},
		},
	}

	buildController := build.NewController(options, knbuildClient, buildInformer, knBuildInformer, metadataRetriever)
	imageController := image.NewController(options, k8sClient, imageInformer, buildInformer, builderInformer, pvcInformer)
	builderController := builder.NewController(options, builderInformer, metadataRetriever)

	stopChan := make(chan struct{})
	informerFactory.Start(stopChan)
	knBuildInformerFactory.Start(stopChan)

	cache.WaitForCacheSync(stopChan, buildInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, imageInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, builderInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, knBuildInformer.Informer().HasSynced)

	err = runGroup(
		func(done <-chan struct{}) error {
			return imageController.Run(routinesPerController, done)
		},
		func(done <-chan struct{}) error {
			return buildController.Run(routinesPerController, done)
		},
		func(done <-chan struct{}) error {
			return builderController.Run(routinesPerController, done)
		},
	)
	if err != nil {
		logger.Fatalw("Error running controller", zap.Error(err))
	}
}

type doneFunc func(done <-chan struct{}) error

func runGroup(fns ...doneFunc) error {
	var wg sync.WaitGroup
	wg.Add(len(fns))

	done := make(chan struct{})
	result := make(chan error, len(fns))
	for _, fn := range fns {
		go func(fn doneFunc) {
			defer wg.Done()
			result <- fn(done)
		}(fn)
	}

	defer close(result)
	defer wg.Wait()
	defer close(done)
	return <-result
}
