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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/configmap/informer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/signals"

	"github.com/pivotal/kpack/cmd"
	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	"github.com/pivotal/kpack/pkg/client/informers/externalversions"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/config"
	"github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds"
	"github.com/pivotal/kpack/pkg/duckbuilder"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/reconciler/build"
	"github.com/pivotal/kpack/pkg/reconciler/builder"
	clusterBuilder "github.com/pivotal/kpack/pkg/reconciler/clusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstack"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstore"
	"github.com/pivotal/kpack/pkg/reconciler/image"
	"github.com/pivotal/kpack/pkg/reconciler/sourceresolver"
	"github.com/pivotal/kpack/pkg/registry"
)

const (
	routinesPerController = 2
	component             = "controller"
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

	buildInitImage         = flag.String("build-init-image", os.Getenv("BUILD_INIT_IMAGE"), "The image used to initialize a build")
	buildInitWindowsImage  = flag.String("build-init-windows-image", os.Getenv("BUILD_INIT_WINDOWS_IMAGE"), "The image used to initialize a build on windows")
	rebaseImage            = flag.String("rebase-image", os.Getenv("REBASE_IMAGE"), "The image used to perform rebases")
	completionImage        = flag.String("completion-image", os.Getenv("COMPLETION_IMAGE"), "The image used to finish a build")
	completionWindowsImage = flag.String("completion-windows-image", os.Getenv("COMPLETION_WINDOWS_IMAGE"), "The image used to finish a build on windows")
	lifecycleImage         = flag.String("lifecycle-image", os.Getenv("LIFECYCLE_IMAGE"), "The image used to provide lifecycle binaries")
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
	buildInformer := informerFactory.Kpack().V1alpha2().Builds()
	imageInformer := informerFactory.Kpack().V1alpha2().Images()
	sourceResolverInformer := informerFactory.Kpack().V1alpha2().SourceResolvers()
	builderInformer := informerFactory.Kpack().V1alpha2().Builders()
	clusterBuilderInformer := informerFactory.Kpack().V1alpha2().ClusterBuilders()
	clusterStoreInformer := informerFactory.Kpack().V1alpha2().ClusterStores()
	clusterStackInformer := informerFactory.Kpack().V1alpha2().ClusterStacks()

	duckBuilderInformer := &duckbuilder.DuckBuilderInformer{
		BuilderInformer:        builderInformer,
		ClusterBuilderInformer: clusterBuilderInformer,
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

	dynamicClient, err := dynamic.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get dynamic client: %s", err)
	}

	buildpodGenerator := &buildpod.Generator{
		BuildPodConfig: buildapi.BuildPodImages{
			BuildInitImage:         *buildInitImage,
			CompletionImage:        *completionImage,
			RebaseImage:            *rebaseImage,
			BuildInitWindowsImage:  *buildInitWindowsImage,
			CompletionWindowsImage: *completionWindowsImage,
		},
		K8sClient:       k8sClient,
		KeychainFactory: keychainFactory,
		ImageFetcher:    &registry.Client{},
		DynamicClient:   dynamicClient,
	}

	gitResolver := git.NewResolver(k8sClient)
	blobResolver := &blob.Resolver{}
	registryResolver := &registry.Resolver{}

	remoteStoreReader := &cnb.RemoteStoreReader{
		RegistryClient: &registry.Client{},
	}

	remoteStackReader := &cnb.RemoteStackReader{
		RegistryClient: &registry.Client{},
	}

	kpackKeychain, err := keychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{})
	if err != nil {
		log.Fatalf("could not create empty keychain %s", err)
	}

	lifecycleProvider := config.NewLifecycleProvider(*lifecycleImage, &registry.Client{}, kpackKeychain)
	configMapWatcher.Watch(config.LifecycleConfigName, lifecycleProvider.UpdateImage)

	builderCreator := &cnb.RemoteBuilderCreator{
		RegistryClient:         &registry.Client{},
		KpackVersion:           cmd.Identifer,
		LifecycleProvider:      lifecycleProvider,
		NewBuildpackRepository: newBuildpackRepository(kpackKeychain),
	}

	buildController := build.NewController(options, k8sClient, buildInformer, podInformer, metadataRetriever, buildpodGenerator)
	imageController := image.NewController(options, k8sClient, imageInformer, buildInformer, duckBuilderInformer, sourceResolverInformer, pvcInformer)
	sourceResolverController := sourceresolver.NewController(options, sourceResolverInformer, gitResolver, blobResolver, registryResolver)
	builderController, builderResync := builder.NewController(options, builderInformer, builderCreator, keychainFactory, clusterStoreInformer, clusterStackInformer)
	clusterBuilderController, clusterBuilderResync := clusterBuilder.NewController(options, clusterBuilderInformer, builderCreator, keychainFactory, clusterStoreInformer, clusterStackInformer)
	clusterStoreController := clusterstore.NewController(options, keychainFactory, clusterStoreInformer, remoteStoreReader)
	clusterStackController := clusterstack.NewController(options, keychainFactory, clusterStackInformer, remoteStackReader)

	lifecycleProvider.AddEventHandler(builderResync)
	lifecycleProvider.AddEventHandler(clusterBuilderResync)

	stopChan := make(chan struct{})
	informerFactory.Start(stopChan)
	k8sInformerFactory.Start(stopChan)

	waitForSync(stopChan,
		buildInformer.Informer(),
		imageInformer.Informer(),
		sourceResolverInformer.Informer(),
		pvcInformer.Informer(),
		podInformer.Informer(),
		builderInformer.Informer(),
		clusterBuilderInformer.Informer(),
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

func newBuildpackRepository(keychain authn.Keychain) func(clusterStore *buildapi.ClusterStore) cnb.BuildpackRepository {
	return func(clusterStore *buildapi.ClusterStore) cnb.BuildpackRepository {
		return &cnb.StoreBuildpackRepository{
			Keychain:     keychain,
			ClusterStore: clusterStore,
		}
	}
}

const controllerCount = 7

//lifted from knative.dev/pkg/injection/sharedmain
func genericControllerSetup(ctx context.Context, cfg *rest.Config) (*zap.SugaredLogger, *informer.InformedWatcher, *http.Server) {
	metrics.MemStatsOrDie(ctx)

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
