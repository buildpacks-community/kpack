package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/sign"
	ociremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	"knative.dev/pkg/system"

	"github.com/pivotal/kpack/cmd"
	_ "github.com/pivotal/kpack/internal/logrus/fatal"
	"github.com/pivotal/kpack/pkg/blob"
	"github.com/pivotal/kpack/pkg/buildchange"
	"github.com/pivotal/kpack/pkg/buildpod"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	"github.com/pivotal/kpack/pkg/client/informers/externalversions"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/config"
	"github.com/pivotal/kpack/pkg/cosign"
	"github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds"
	"github.com/pivotal/kpack/pkg/duckbuilder"
	"github.com/pivotal/kpack/pkg/flaghelpers"
	"github.com/pivotal/kpack/pkg/git"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/reconciler/build"
	"github.com/pivotal/kpack/pkg/reconciler/builder"
	"github.com/pivotal/kpack/pkg/reconciler/buildpack"
	"github.com/pivotal/kpack/pkg/reconciler/clusterbuilder"
	"github.com/pivotal/kpack/pkg/reconciler/clusterbuildpack"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstack"
	"github.com/pivotal/kpack/pkg/reconciler/clusterstore"
	"github.com/pivotal/kpack/pkg/reconciler/image"
	"github.com/pivotal/kpack/pkg/reconciler/lifecycle"
	"github.com/pivotal/kpack/pkg/reconciler/sourceresolver"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/secret"
	"github.com/pivotal/kpack/pkg/slsa"
)

const (
	defaultRoutinesPerController = 2
	component                    = "controller"
)

var (
	images       config.Images
	cfg          config.Config
	featureFlags config.FeatureFlags
)

var (
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
)

func main() {
	flag.StringVar(&images.BuildInitImage, "build-init-image", os.Getenv("BUILD_INIT_IMAGE"), "The image used to initialize a build")
	flag.StringVar(&images.BuildInitWindowsImage, "build-init-windows-image", os.Getenv("BUILD_INIT_WINDOWS_IMAGE"), "The image used to initialize a build on windows")
	flag.StringVar(&images.RebaseImage, "rebase-image", os.Getenv("REBASE_IMAGE"), "The image used to perform rebases")
	flag.StringVar(&images.CompletionImage, "completion-image", os.Getenv("COMPLETION_IMAGE"), "The image used to finish a build")
	flag.StringVar(&images.CompletionWindowsImage, "completion-windows-image", os.Getenv("COMPLETION_WINDOWS_IMAGE"), "The image used to finish a build on windows")
	flag.StringVar(&images.BuildWaiterImage, "build-waiter-image", os.Getenv("BUILD_WAITER_IMAGE"), "The image used to initialize a build")

	flag.StringVar(&cfg.SystemNamespace, "system-namespace", os.Getenv("SYSTEM_NAMESPACE"), "Namespace for the the controller, this will be used to lookup secrets for image signing and attestation.")
	flag.StringVar(&cfg.SystemServiceAccount, "system-service-account", os.Getenv("SYSTEM_SERVICE_ACCOUNT"), "Service account for the the controller, this will be used to lookup secrets for image signing and attestation.")
	flag.BoolVar(&cfg.EnablePriorityClasses, "enable-priority-classes", flaghelpers.GetEnvBool("ENABLE_PRIORITY_CLASSES", false), "if set to true, enables different pod priority classes for normal builds and automated builds")
	flag.StringVar(&cfg.MaximumPlatformApiVersion, "maximum-platform-api-version", os.Getenv("MAXIMUM_PLATFORM_API_VERSION"), "The maximum allowed platform api version a build can utilize")
	flag.BoolVar(&cfg.SshTrustUnknownHosts, "insecure-ssh-trust-unknown-hosts", flaghelpers.GetEnvBool("INSECURE_SSH_TRUST_UNKNOWN_HOSTS", true), "if set to true, automatically trust unknown hosts when using git ssh source")
	flag.IntVar(&cfg.ScalingFactor, "scaling-factor", flaghelpers.GetEnvInt("SCALING_FACTOR", 1), "The scaling factor to scale client-side rate limits by")

	flag.BoolVar(&featureFlags.InjectedSidecarSupport, "injected-sidecar-support", flaghelpers.GetEnvBool("INJECTED_SIDECAR_SUPPORT", false), "if set to true, all builds will execute in standard containers instead of init containers to support injected sidecars")
	flag.BoolVar(&featureFlags.GenerateSlsaAttestation, "experimental-generate-slsa-attestation", flaghelpers.GetEnvBool("EXPERIMENTAL_GENERATE_SLSA_ATTESTATION", false), "if set to true, SLSA attestations will be generated for each build")

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
	buildpackInformer := informerFactory.Kpack().V1alpha2().Buildpacks()
	clusterBuilderInformer := informerFactory.Kpack().V1alpha2().ClusterBuilders()
	clusterBuildpackInformer := informerFactory.Kpack().V1alpha2().ClusterBuildpacks()
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
	lifecycleConfigmapInformerFactory := informers.NewSharedInformerFactoryWithOptions(
		k8sClient,
		options.ResyncPeriod,
		informers.WithNamespace(system.Namespace()),
		informers.WithTweakListOptions(func(options *metav1.ListOptions) {
			options.FieldSelector = fmt.Sprintf("metadata.namespace=%s,metadata.name=%s", system.Namespace(), config.LifecycleConfigName)
		}),
	)
	lifecycleConfigmapInformer := lifecycleConfigmapInformerFactory.Core().V1().ConfigMaps()

	metadataRetriever := &cnb.RemoteMetadataRetriever{
		ImageFetcher: &registry.Client{},
	}

	dynamicClient, err := dynamic.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get dynamic client: %s", err)
	}

	maxPlatformApi, err := parseMaxPlatformApiVersion()
	if err != nil {
		log.Fatalf("could not resolve provided maximum platform api version: %s", err)
	}

	buildpodGenerator := &buildpod.Generator{
		BuildPodConfig:            images.ToBuildPodImages(),
		K8sClient:                 k8sClient,
		KeychainFactory:           keychainFactory,
		ImageFetcher:              &registry.Client{},
		DynamicClient:             dynamicClient,
		MaximumPlatformApiVersion: maxPlatformApi,
		InjectedSidecarSupport:    featureFlags.InjectedSidecarSupport,
		SSHTrustUnknownHost:       cfg.SshTrustUnknownHosts,
	}

	gitResolver := git.NewResolver(k8sClient, cfg.SshTrustUnknownHosts)
	blobResolver := &blob.Resolver{}
	registryResolver := &registry.Resolver{}

	remoteStoreReader := &cnb.RemoteBuildpackReader{
		RegistryClient: &registry.Client{},
	}

	remoteStackReader := &cnb.RemoteStackReader{
		RegistryClient: &registry.Client{},
	}

	lifecycleProvider := config.NewLifecycleProvider(&registry.Client{}, keychainFactory)

	builderCreator := &cnb.RemoteBuilderCreator{
		RegistryClient:    &registry.Client{},
		KpackVersion:      cmd.Identifer,
		LifecycleProvider: lifecycleProvider,
		KeychainFactory:   keychainFactory,
		ImageSigner:       cosign.NewImageSigner(sign.SignCmd, ociremote.SignatureTag),
	}

	podProgressLogger := &buildchange.ProgressLogger{
		K8sClient: k8sClient,
	}

	slsaAttester := slsa.Attester{
		Version: cmd.Version,

		LifecycleProvider: lifecycleProvider,
		ImageReader:       slsa.NewImageReader(&registry.Client{}),

		Images:   images,
		Features: featureFlags,
		Config:   cfg,
	}

	secretFetcher := &secret.Fetcher{
		Client:                   k8sClient,
		SystemNamespace:          cfg.SystemNamespace,
		SystemServiceAccountName: cfg.SystemServiceAccount,
	}

	buildController := build.NewController(ctx, options, k8sClient, buildInformer, podInformer, metadataRetriever, buildpodGenerator, podProgressLogger, keychainFactory, &slsaAttester, secretFetcher, featureFlags)
	imageController := image.NewController(ctx, options, k8sClient, imageInformer, buildInformer, duckBuilderInformer, sourceResolverInformer, pvcInformer, cfg.EnablePriorityClasses)
	sourceResolverController := sourceresolver.NewController(ctx, options, sourceResolverInformer, gitResolver, blobResolver, registryResolver)
	builderController, builderResync := builder.NewController(ctx, options, builderInformer, builderCreator, keychainFactory, clusterStoreInformer, buildpackInformer, clusterBuildpackInformer, clusterStackInformer, secretFetcher)
	buildpackController := buildpack.NewController(ctx, options, keychainFactory, buildpackInformer, remoteStoreReader)
	clusterBuilderController, clusterBuilderResync := clusterbuilder.NewController(ctx, options, clusterBuilderInformer, builderCreator, keychainFactory, clusterStoreInformer, clusterBuildpackInformer, clusterStackInformer, secretFetcher)
	clusterBuildpackController := clusterbuildpack.NewController(ctx, options, keychainFactory, clusterBuildpackInformer, remoteStoreReader)
	clusterStoreController := clusterstore.NewController(ctx, options, keychainFactory, clusterStoreInformer, remoteStoreReader)
	clusterStackController := clusterstack.NewController(ctx, options, keychainFactory, clusterStackInformer, remoteStackReader)
	lifecycleController := lifecycle.NewController(ctx, options, k8sClient, config.LifecycleConfigName, lifecycleConfigmapInformer, lifecycleProvider)

	lifecycleProvider.AddEventHandler(builderResync)
	lifecycleProvider.AddEventHandler(clusterBuilderResync)

	stopChan := make(chan struct{})
	informerFactory.Start(stopChan)
	k8sInformerFactory.Start(stopChan)
	lifecycleConfigmapInformerFactory.Start(stopChan)

	waitForSync(stopChan,
		buildInformer.Informer(),
		imageInformer.Informer(),
		sourceResolverInformer.Informer(),
		pvcInformer.Informer(),
		podInformer.Informer(),
		lifecycleConfigmapInformer.Informer(),
		builderInformer.Informer(),
		buildpackInformer.Informer(),
		clusterBuilderInformer.Informer(),
		clusterBuildpackInformer.Informer(),
		clusterStoreInformer.Informer(),
		clusterStackInformer.Informer(),
	)

	routinesPerController := defaultRoutinesPerController * cfg.ScalingFactor
	err = runGroup(
		ctx,
		run(clusterStackController, routinesPerController),
		run(imageController, routinesPerController),
		run(buildController, routinesPerController),
		run(builderController, routinesPerController),
		run(buildpackController, routinesPerController),
		run(clusterBuilderController, routinesPerController),
		run(clusterBuildpackController, routinesPerController),
		run(clusterStoreController, routinesPerController),
		run(lifecycleController, routinesPerController),
		run(sourceResolverController, 2*routinesPerController),
		func(ctx context.Context) error {
			return configMapWatcher.Start(ctx.Done())
		},
		func(ctx context.Context) error {
			return profilingServer.ListenAndServe()
		},
		func(ctx context.Context) error {
			<-ctx.Done()
			return profilingServer.Shutdown(ctx)
		},
	)
	if err != nil && err != http.ErrServerClosed {
		logger.Fatalw("Error running controller", zap.Error(err))
	}
}

func run(ctrl *controller.Impl, threadiness int) doneFunc {
	return func(ctx context.Context) error {
		return ctrl.RunContext(ctx, threadiness)
	}
}

type doneFunc func(ctx context.Context) error

func runGroup(ctx context.Context, fns ...func(ctx context.Context) error) error {
	eg, egCtx := errgroup.WithContext(ctx)
	for _, fn := range fns {
		fnCopy := fn
		eg.Go(func() error {
			return fnCopy(egCtx)
		})
	}

	return eg.Wait()
}

const controllerCount = 7

// lifted from knative.dev/pkg/injection/sharedmain
func genericControllerSetup(ctx context.Context, restCfg *rest.Config) (*zap.SugaredLogger, *informer.InformedWatcher, *http.Server) {
	metrics.MemStatsOrDie(ctx)

	// Adjust our client's rate limits based on the number of controllers we are running.
	restCfg.QPS = float32(controllerCount) * rest.DefaultQPS * float32(cfg.ScalingFactor)
	restCfg.Burst = controllerCount * rest.DefaultBurst * cfg.ScalingFactor
	ctx, _ = injection.Default.SetupInformers(ctx, restCfg)

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

func parseMaxPlatformApiVersion() (*semver.Version, error) {
	if cfg.MaximumPlatformApiVersion != "" {
		return semver.NewVersion(cfg.MaximumPlatformApiVersion)
	}

	return nil, nil
}
