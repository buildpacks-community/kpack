package build

import (
	"context"

	knv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knversioned "github.com/knative/build/pkg/client/clientset/versioned"
	knv1alpha1informer "github.com/knative/build/pkg/client/informers/externalversions/build/v1alpha1"
	knv1alpha1lister "github.com/knative/build/pkg/client/listers/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informer "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1lister "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/cnb"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	ReconcilerName = "Builds"
	Kind           = "Build"
)

type MetadataRetriever interface {
	GetBuiltImage(repoName registry.ImageRef) (cnb.BuiltImage, error)
}

type Options struct {
	reconciler.Options
	BuildInitImage string
}

func NewController(opt Options, knClient knversioned.Interface, informer v1alpha1informer.BuildInformer, kninformer knv1alpha1informer.BuildInformer, metadataRetriever MetadataRetriever) *controller.Impl {
	c := &Reconciler{
		KNClient:          knClient,
		Client:            opt.Client,
		Lister:            informer.Lister(),
		KnLister:          kninformer.Lister(),
		MetadataRetriever: metadataRetriever,
		BuildInitImage:    opt.BuildInitImage,
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	informer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	kninformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

type Reconciler struct {
	KNClient          knversioned.Interface
	Client            versioned.Interface
	Lister            v1alpha1lister.BuildLister
	KnLister          knv1alpha1lister.BuildLister
	MetadataRetriever MetadataRetriever
	BuildInitImage    string
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, buildName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	build, err := c.Lister.Builds(namespace).Get(buildName)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	build = build.DeepCopy()

	knBuild, err := c.KnLister.Builds(namespace).Get(buildName)
	if err != nil && !errors.IsNotFound(err) {
		return err
	} else if errors.IsNotFound(err) {
		knBuild, err = c.createKNBuild(namespace, build)
		if err != nil {
			return err
		}
	}

	if knBuild.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() && !build.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() {
		image, err := c.MetadataRetriever.GetBuiltImage(build)
		if err != nil {
			return err
		}

		build.Status.BuildMetadata = buildMetadataFromBuiltImage(image)
		build.Status.SHA = image.SHA
	}

	build.Status.Conditions = knBuild.Status.Conditions
	build.Status.ObservedGeneration = build.Generation

	_, err = c.Client.BuildV1alpha1().Builds(namespace).UpdateStatus(build)
	if err != nil {
		return err
	}

	return nil
}

func (c *Reconciler) createKNBuild(namespace string, build *v1alpha1.Build) (*knv1alpha1.Build, error) {
	const cacheDirName = "empty-dir"
	const layersDirName = "layers-dir"
	var root int64 = 0
	return c.KNClient.BuildV1alpha1().Builds(namespace).Create(&knv1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: build.Name,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(build),
			},
		},
		Spec: knv1alpha1.BuildSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			Source: &knv1alpha1.SourceSpec{
				Git: &knv1alpha1.GitSourceSpec{
					Url:      build.Spec.Source.Git.URL,
					Revision: build.Spec.Source.Git.Revision,
				},
			},
			Steps: []corev1.Container{
				{
					Name:  "prepare",
					Image: c.BuildInitImage,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  &root,
						RunAsGroup: &root,
					},
					Env: []corev1.EnvVar{
						{
							Name:  "BUILDER",
							Value: build.Spec.Builder,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layersDir", //layers is already in buildpack built image
						},
						{
							Name:      cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "detect",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/detector"},
					Args: []string{
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "restore",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/restorer"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name:      cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "analyze",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/analyzer"},
					Args: []string{
						"-layers=/layers",
						"-helpers=false",
						"-group=/layers/group.toml",
						build.Spec.Image,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "build",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/builder"},
					Args: []string{
						"-layers=/layers",
						"-app=/workspace",
						"-group=/layers/group.toml",
						"-plan=/layers/plan.toml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "export",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/exporter"},
					Args:    buildExporterArgs(build),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
					},
					ImagePullPolicy: "Always",
				},
				{
					Name:    "cache",
					Image:   build.Spec.Builder,
					Command: []string{"/lifecycle/cacher"},
					Args: []string{
						"-group=/layers/group.toml",
						"-layers=/layers",
						"-path=/cache",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      layersDirName,
							MountPath: "/layers",
						},
						{
							Name:      cacheDirName,
							MountPath: "/cache",
						},
					},
					ImagePullPolicy: "Always",
				},
			},
			Volumes: []corev1.Volume{
				{
					Name:         cacheDirName,
					VolumeSource: c.createCacheVolume(build),
				},
				{
					Name: layersDirName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	})
}

func buildExporterArgs(build *v1alpha1.Build) []string {
	args := []string{
		"-layers=/layers",
		"-helpers=false",
		"-app=/workspace",
		"-group=/layers/group.toml",
		build.Spec.Image,}
	args = append(args, build.Spec.AdditionalImageNames...)
	return args
}

func (c *Reconciler) createCacheVolume(build *v1alpha1.Build) corev1.VolumeSource {
	if build.Spec.CacheName != "" {
		return corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: build.Spec.CacheName},
		}
	} else {
		return corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	}
}

func buildMetadataFromBuiltImage(image cnb.BuiltImage) []v1alpha1.BuildpackMetadata {
	buildpackMetadata := make([]v1alpha1.BuildpackMetadata, 0, len(image.BuildpackMetadata))
	for _, metadata := range image.BuildpackMetadata {
		buildpackMetadata = append(buildpackMetadata, v1alpha1.BuildpackMetadata{
			ID:      metadata.ID,
			Version: metadata.Version,
		})
	}
	return buildpackMetadata
}
