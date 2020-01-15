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
	"knative.dev/pkg/controller"

	"github.com/pivotal/kpack/cmd"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	"github.com/pivotal/kpack/pkg/client/informers/externalversions"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds"
	"github.com/pivotal/kpack/pkg/duckbuilder"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/build"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/builder"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/clusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/custombuilder"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/customclusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/image"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/sourceresolver"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/stack"
	"github.com/pivotal/kpack/pkg/reconciler/v1alpha1/store"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	routinesPerController = 2
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	buildInitImage  = flag.String("build-init-image", os.Getenv("BUILD_INIT_IMAGE"), "The image used to initialize a build")
	rebaseImage     = flag.String("rebase-image", os.Getenv("REBASE_IMAGE"), "The image used to perform rebases")
	completionImage = flag.String("completion-image", os.Getenv("COMPLETION_IMAGE"), "The image used to finish a build")
	lifecycleImage  = flag.String("lifecycle-image", os.Getenv("LIFECYCLE_IMAGE"), "The image used to provide lifecycle binaries")
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
	customBuilderInformer := informerFactory.Experimental().V1alpha1().CustomBuilders()
	customClusterBuilderInformer := informerFactory.Experimental().V1alpha1().CustomClusterBuilders()
	storeInformer := informerFactory.Experimental().V1alpha1().Stores()
	stackInformer := informerFactory.Experimental().V1alpha1().Stacks()

	duckBuilderInformer := &duckbuilder.DuckBuilderInformer{
		BuilderInformer:              builderInformer,
		ClusterBuilderInformer:       clusterBuilderInformer,
		CustomBuilderInformer:        customBuilderInformer,
		CustomClusterBuilderInformer: customClusterBuilderInformer,
	}

	k8sInformerFactory := informers.NewSharedInformerFactory(k8sClient, options.ResyncPeriod)
	pvcInformer := k8sInformerFactory.Core().V1().PersistentVolumeClaims()
	podInformer := k8sInformerFactory.Core().V1().Pods()

	keychainFactory, err := k8sdockercreds.NewSecretKeychainFactory(k8sClient)
	if err != nil {
		log.Fatalf("could not create k8s keychain factory: %s", err)
	}

	metadataRetriever := &cnb.RemoteMetadataRetriever{
		KeychainFactory: keychainFactory,
		ImageFetcher:    &registry.Client{},
	}

	buildpodGenerator := &buildpod.Generator{
		BuildPodConfig: v1alpha1.BuildPodImages{
			BuildInitImage:  *buildInitImage,
			CompletionImage: *completionImage,
			RebaseImage:     *rebaseImage,
		},
		K8sClient:       k8sClient,
		KeychainFactory: keychainFactory,
		ImageFetcher:    &registry.Client{},
	}

	builderCreator := &cnb.RemoteBuilderCreator{
		RegistryClient: &registry.Client{},
		LifecycleImage: *lifecycleImage,
		KpackVersion:   cmd.Version,
	}

	gitResolver := git.NewResolver(k8sClient)
	blobResolver := &blob.Resolver{}
	registryResolver := &registry.Resolver{}
	remoteStoreReader := &cnb.RemoteStoreReader{
		RegistryClient: &registry.Client{},
	}

	buildController := build.NewController(options, k8sClient, buildInformer, podInformer, metadataRetriever, buildpodGenerator)
	imageController := image.NewController(options, k8sClient, imageInformer, buildInformer, duckBuilderInformer, sourceResolverInformer, pvcInformer)
	builderController := builder.NewController(options, builderInformer, metadataRetriever)
	clusterBuilderController := clusterbuilder.NewController(options, clusterBuilderInformer, metadataRetriever)
	sourceResolverController := sourceresolver.NewController(options, sourceResolverInformer, gitResolver, blobResolver, registryResolver)
	customBuilderController := custombuilder.NewController(options, customBuilderInformer, newBuildpackRepository(keychainFactory), builderCreator, keychainFactory, storeInformer, stackInformer)
	customClusterBuilderController := customclusterbuilder.NewController(options, customClusterBuilderInformer, newBuildpackRepository(keychainFactory), builderCreator, keychainFactory, storeInformer, stackInformer)
	storeController := store.NewController(options, storeInformer, remoteStoreReader, keychainFactory)
	stackController := stack.NewController(options, stackInformer, keychainFactory, &registry.Client{})

	stopChan := make(chan struct{})
	informerFactory.Start(stopChan)
	k8sInformerFactory.Start(stopChan)

	waitForSync(stopChan,
		buildInformer.Informer(),
		imageInformer.Informer(),
		builderInformer.Informer(),
		clusterBuilderInformer.Informer(),
		sourceResolverInformer.Informer(),
		pvcInformer.Informer(),
		podInformer.Informer(),
		customBuilderInformer.Informer(),
		customClusterBuilderInformer.Informer(),
		storeInformer.Informer(),
		stackInformer.Informer(),
	)

	err = runGroup(
		run(imageController, routinesPerController),
		run(buildController, routinesPerController),
		run(builderController, routinesPerController),
		run(clusterBuilderController, routinesPerController),
		run(customBuilderController, routinesPerController),
		run(customClusterBuilderController, routinesPerController),
		run(storeController, routinesPerController),
		run(stackController, routinesPerController),
		run(sourceResolverController, 2*routinesPerController),
	)
	if err != nil {
		logger.Fatalw("Error running controller", zap.Error(err))
	}
}

func waitForSync(stopCh <-chan struct{}, indexFormers ...cache.SharedIndexInformer) {
	for _, informer := range indexFormers {
		cache.WaitForCacheSync(stopCh, informer.HasSynced)
	}
}

func run(ctrl *controller.Impl, threadiness int) doneFunc {
	return func(done <-chan struct{}) error {
		return ctrl.Run(threadiness, done)
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

func newBuildpackRepository(keychainFactory registry.KeychainFactory) func(store *expv1alpha1.Store) cnb.BuildpackRepository {
	storeKeychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{})
	if err != nil {
		log.Fatalf("could not create empty keychain %s", err)
	}
	return func(store *expv1alpha1.Store) cnb.BuildpackRepository {
		return &cnb.StoreBuildpackRepository{
			Keychain: storeKeychain,
			Store:    store,
		}
	}
}
