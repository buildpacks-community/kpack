package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/signals"

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
	"github.com/pivotal/kpack/pkg/reconciler/build"
	"github.com/pivotal/kpack/pkg/reconciler/builder"
	"github.com/pivotal/kpack/pkg/reconciler/clusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstack"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstore"
	"github.com/pivotal/kpack/pkg/reconciler/custombuilder"
	"github.com/pivotal/kpack/pkg/reconciler/customclusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/image"
	"github.com/pivotal/kpack/pkg/reconciler/sourceresolver"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/s3"
)

const (
	routinesPerController = 2
	component             = "controller"
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

	clusterConfig, err := clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	ctx := signals.NewContext()
	logger, configMapWatcher, profilingServer := genericControllerSetup(ctx, clusterConfig)
	defer logger.Sync()
	defer metrics.FlushExporter()

	client, err := versioned.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get Build client: %s", err)
	}

	k8sClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err)
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
	clusterStoreInformer := informerFactory.Experimental().V1alpha1().ClusterStores()
	clusterStackInformer := informerFactory.Experimental().V1alpha1().ClusterStacks()

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
	s3Resolver := &s3.Resolver{}

	kpackKeychain, err := keychainFactory.KeychainForSecretRef(registry.SecretRef{})
	if err != nil {
		log.Fatalf("could not create empty keychain %s", err)
	}

	remoteStoreReader := &cnb.RemoteStoreReader{
		RegistryClient: &registry.Client{},
		Keychain:       kpackKeychain,
	}

	remoteStackReader := &cnb.RemoteStackReader{
		RegistryClient: &registry.Client{},
		Keychain:       kpackKeychain,
	}

	buildController := build.NewController(options, k8sClient, buildInformer, podInformer, metadataRetriever, buildpodGenerator)
	imageController := image.NewController(options, k8sClient, imageInformer, buildInformer, duckBuilderInformer, sourceResolverInformer, pvcInformer)
	builderController := builder.NewController(options, builderInformer, metadataRetriever)
	clusterBuilderController := clusterbuilder.NewController(options, clusterBuilderInformer, metadataRetriever)
	sourceResolverController := sourceresolver.NewController(options, sourceResolverInformer, gitResolver, blobResolver, registryResolver, s3Resolver)
	customBuilderController := custombuilder.NewController(options, customBuilderInformer, newBuildpackRepository(kpackKeychain), builderCreator, keychainFactory, clusterStoreInformer, clusterStackInformer)
	customClusterBuilderController := customclusterbuilder.NewController(options, customClusterBuilderInformer, newBuildpackRepository(kpackKeychain), builderCreator, keychainFactory, clusterStoreInformer, clusterStackInformer)
	clusterStoreController := clusterstore.NewController(options, clusterStoreInformer, remoteStoreReader)
	clusterStackController := clusterstack.NewController(options, clusterStackInformer, remoteStackReader)

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
		clusterStoreInformer.Informer(),
		clusterStackInformer.Informer(),
	)

	err = runGroup(
		ctx,
		run(clusterStackController, routinesPerController),
		run(imageController, routinesPerController),
		run(buildController, routinesPerController),
		run(builderController, routinesPerController),
		run(clusterBuilderController, routinesPerController),
		run(customBuilderController, routinesPerController),
		run(customClusterBuilderController, routinesPerController),
		run(clusterStoreController, routinesPerController),
		run(sourceResolverController, 2*routinesPerController),
		configMapWatcher.Start,
		func(done <-chan struct{}) error {
			return profilingServer.ListenAndServe()
		},
		func(done <-chan struct{}) error {
			<-done
			return profilingServer.Shutdown(ctx)
		},
	)
	if err != nil && err != http.ErrServerClosed {
		logger.Fatalw("Error running controller", zap.Error(err))
	}
}

func run(ctrl *controller.Impl, threadiness int) doneFunc {
	return func(done <-chan struct{}) error {
		return ctrl.Run(threadiness, done)
	}
}

type doneFunc func(done <-chan struct{}) error

func runGroup(ctx context.Context, fns ...doneFunc) error {
	eg, egCtx := errgroup.WithContext(ctx)
	for _, fn := range fns {
		fnCopy := fn
		eg.Go(func() error {
			return fnCopy(egCtx.Done())
		})
	}

	return eg.Wait()
}

func newBuildpackRepository(keychain authn.Keychain) func(clusterStore *expv1alpha1.ClusterStore) cnb.BuildpackRepository {
	return func(clusterStore *expv1alpha1.ClusterStore) cnb.BuildpackRepository {
		return &cnb.StoreBuildpackRepository{
			Keychain:     keychain,
			ClusterStore: clusterStore,
		}
	}
}

const controllerCount = 9

//lifted from knative.dev/pkg/injection/sharedmain
func genericControllerSetup(ctx context.Context, cfg *rest.Config) (*zap.SugaredLogger, *configmap.InformedWatcher, *http.Server) {
	sharedmain.MemStatsOrDie(ctx)

	// Adjust our client's rate limits based on the number of controllers we are running.
	cfg.QPS = float32(controllerCount) * rest.DefaultQPS
	cfg.Burst = controllerCount * rest.DefaultBurst
	ctx, _ = injection.Default.SetupInformers(ctx, cfg)

	logger, atomicLevel := sharedmain.SetupLoggerOrDie(ctx, component)
	ctx = logging.WithLogger(ctx, logger)
	profilingHandler := profiling.NewHandler(logger, false)
	profilingServer := profiling.NewServer(profilingHandler)

	sharedmain.CheckK8sClientMinimumVersionOrDie(ctx, logger)
	cmw := sharedmain.SetupConfigMapWatchOrDie(ctx, logger)
	sharedmain.WatchLoggingConfigOrDie(ctx, cmw, logger, atomicLevel, component)
	sharedmain.WatchObservabilityConfigOrDie(ctx, cmw, profilingHandler, logger, component)

	return logger, cmw, profilingServer
}

func waitForSync(stopCh <-chan struct{}, indexFormers ...cache.SharedIndexInformer) {
	for _, informer := range indexFormers {
		cache.WaitForCacheSync(stopCh, informer.HasSynced)
	}
}
