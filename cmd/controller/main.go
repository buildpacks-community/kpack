package main

import (
	"flag"
	"log"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/blob"
	"github.com/pivotal/build-service-system/pkg/buildpod"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-system/pkg/cnb"
	"github.com/pivotal/build-service-system/pkg/git"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/builder"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/image"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/sourceresolver"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	routinesPerController = 2
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	buildInitImage  = flag.String("build-init-image", os.Getenv("BUILD_INIT_IMAGE"), "The image used to initialize a build")
	sourceInitImage = flag.String("source-init-image", os.Getenv("SOURCE_INIT_IMAGE"), "The image used to fetch the app source")
	credInitImage   = flag.String("cred-init-image", os.Getenv("CRED_INIT_IMAGE"), "The image used to setup build credentials")
	nopImage        = flag.String("nop-image", os.Getenv("NOP_IMAGE"), "The image used to finish a build")
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

	k8sClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err.Error())
	}

	options := reconciler.Options{
		Logger:                  logger,
		Client:                  client,
		ResyncPeriod:            10 * time.Hour,
		SourcePollingFrequency:  1 * time.Minute,
		BuilderPollingFrequency: 1 * time.Minute,
	}

	informerFactory := externalversions.NewSharedInformerFactory(client, options.ResyncPeriod)
	buildInformer := informerFactory.Build().V1alpha1().Builds()
	imageInformer := informerFactory.Build().V1alpha1().Images()
	builderInformer := informerFactory.Build().V1alpha1().Builders()
	sourceResolverInformer := informerFactory.Build().V1alpha1().SourceResolvers()

	k8sInformerFactory := informers.NewSharedInformerFactory(k8sClient, options.ResyncPeriod)
	pvcInformer := k8sInformerFactory.Core().V1().PersistentVolumeClaims()
	podInformer := k8sInformerFactory.Core().V1().Pods()

	metadataRetriever := &cnb.RemoteMetadataRetriever{
		LifecycleImageFactory: &registry.ImageFactory{
			KeychainFactory: registry.NewSecretKeychainFactory(k8sClient),
		},
	}

	buildpodGenerator := &buildpod.Generator{
		BuildPodConfig: v1alpha1.BuildPodConfig{
			BuildInitImage:  *buildInitImage,
			SourceInitImage: *sourceInitImage,
			CredsInitImage:  *credInitImage,
			NopImage:        *nopImage,
		},
		K8sClient: k8sClient,
	}

	gitResolver := git.NewResolver(k8sClient)
	blobResolver := &blob.Resolver{}

	buildController := build.NewController(options, k8sClient, buildInformer, podInformer, metadataRetriever, buildpodGenerator)
	imageController := image.NewController(options, k8sClient, imageInformer, buildInformer, builderInformer, sourceResolverInformer, pvcInformer)
	builderController := builder.NewController(options, builderInformer, metadataRetriever)
	sourceResolverController := sourceresolver.NewController(options, sourceResolverInformer, gitResolver, blobResolver)

	stopChan := make(chan struct{})
	informerFactory.Start(stopChan)
	k8sInformerFactory.Start(stopChan)

	cache.WaitForCacheSync(stopChan, buildInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, imageInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, builderInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, sourceResolverInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, pvcInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, podInformer.Informer().HasSynced)

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
		func(done <-chan struct{}) error {
			return sourceResolverController.Run(2*routinesPerController, done)
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
