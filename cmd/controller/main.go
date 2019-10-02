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

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	"github.com/pivotal/kpack/pkg/client/informers/externalversions"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/dockercreds"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/builder"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/clusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/image"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/sourceresolver"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	routinesPerController = 2
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	buildInitImage = flag.String("build-init-image", os.Getenv("BUILD_INIT_IMAGE"), "The image used to initialize a build")
	nopImage       = flag.String("nop-image", os.Getenv("NOP_IMAGE"), "The image used to finish a build")
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
	clusterBuilderInformer := informerFactory.Build().V1alpha1().ClusterBuilders()
	sourceResolverInformer := informerFactory.Build().V1alpha1().SourceResolvers()

	k8sInformerFactory := informers.NewSharedInformerFactory(k8sClient, options.ResyncPeriod)
	pvcInformer := k8sInformerFactory.Core().V1().PersistentVolumeClaims()
	podInformer := k8sInformerFactory.Core().V1().Pods()

	imageFactory := &registry.ImageFactory{
		KeychainFactory: dockercreds.NewSecretKeychainFactory(k8sClient),
	}

	imageUtilFactory := &cnb.ImageFactory{
		KeychainFactory: dockercreds.NewSecretKeychainFactory(k8sClient),
	}

	metadataRetriever := &cnb.RemoteMetadataRetriever{
		RemoteImageFactory: imageFactory,
	}

	rebaser := cnb.ImageRebaser{
		RemoteImageFactory: imageUtilFactory,
	}

	buildpodGenerator := &buildpod.Generator{
		BuildPodConfig: v1alpha1.BuildPodConfig{
			BuildInitImage: *buildInitImage,
			NopImage:       *nopImage,
		},
		K8sClient:          k8sClient,
		RemoteImageFactory: imageFactory,
	}

	gitResolver := git.NewResolver(k8sClient)
	blobResolver := &blob.Resolver{}
	registryResolver := &registry.Resolver{}

	buildController := build.NewController(options, k8sClient, buildInformer, podInformer, metadataRetriever, buildpodGenerator, rebaser)
	imageController := image.NewController(options, k8sClient, imageInformer, buildInformer, builderInformer, clusterBuilderInformer, sourceResolverInformer, pvcInformer)
	builderController := builder.NewController(options, builderInformer, metadataRetriever)
	clusterBuilderController := clusterbuilder.NewController(options, clusterBuilderInformer, metadataRetriever)
	sourceResolverController := sourceresolver.NewController(options, sourceResolverInformer, gitResolver, blobResolver, registryResolver)

	stopChan := make(chan struct{})
	informerFactory.Start(stopChan)
	k8sInformerFactory.Start(stopChan)

	cache.WaitForCacheSync(stopChan, buildInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, imageInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, builderInformer.Informer().HasSynced)
	cache.WaitForCacheSync(stopChan, clusterBuilderInformer.Informer().HasSynced)
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
			return clusterBuilderController.Run(routinesPerController, done)
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
