package cnbbuild

import (
	"context"

	knv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knversioned "github.com/knative/build/pkg/client/clientset/versioned"
	knv1alpha1informer "github.com/knative/build/pkg/client/informers/externalversions/build/v1alpha1"
	knv1alpha1lister "github.com/knative/build/pkg/client/listers/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1informer "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1lister "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/reconciler"
	"github.com/pivotal/build-service-system/pkg/registry"
)

const (
	ReconcilerName = "CNBBuilds"
	Kind           = "CNBBuild"
)

type MetadataRetriever interface {
	GetBuiltImage(repoName registry.ImageRef) (registry.BuiltImage, error)
}

func NewController(opt reconciler.Options, knClient knversioned.Interface, cnbinformer v1alpha1informer.CNBBuildInformer, kninformer knv1alpha1informer.BuildInformer, metadataRetriever MetadataRetriever) *controller.Impl {
	c := &Reconciler{
		KNClient:          knClient,
		CNBBuildClient:    opt.CNBBuildClient,
		CNBLister:         cnbinformer.Lister(),
		KnLister:          kninformer.Lister(),
		MetadataRetriever: metadataRetriever,
	}

	impl := controller.NewImpl(c, opt.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, opt.Logger))

	cnbinformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	kninformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind(Kind)),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})

	return impl
}

type Reconciler struct {
	KNClient          knversioned.Interface
	CNBBuildClient    versioned.Interface
	CNBLister         v1alpha1lister.CNBBuildLister
	KnLister          knv1alpha1lister.BuildLister
	MetadataRetriever MetadataRetriever
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, buildName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	build, err := c.CNBLister.CNBBuilds(namespace).Get(buildName)
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
	}

	build.Status.Conditions = knBuild.Status.Conditions
	build.Status.ObservedGeneration = build.Generation

	_, err = c.CNBBuildClient.BuildV1alpha1().CNBBuilds(namespace).UpdateStatus(build)
	if err != nil {
		return err
	}

	return nil
}

func (c *Reconciler) createKNBuild(namespace string, build *v1alpha1.CNBBuild) (*knv1alpha1.Build, error) {
	return c.KNClient.BuildV1alpha1().Builds(namespace).Create(&knv1alpha1.Build{
		ObjectMeta: v1.ObjectMeta{
			Name: build.Name,
			OwnerReferences: []v1.OwnerReference{
				*kmeta.NewControllerRef(build),
			},
		},
		Spec: knv1alpha1.BuildSpec{
			ServiceAccountName: build.Spec.ServiceAccount,
			Source: &knv1alpha1.SourceSpec{
				Git: &knv1alpha1.GitSourceSpec{
					Url:      build.Spec.GitURL,
					Revision: build.Spec.GitRevision,
				},
			},
			Template: &knv1alpha1.TemplateInstantiationSpec{
				Name: "buildpacks-cnb",
				Arguments: []knv1alpha1.ArgumentSpec{
					{Name: "IMAGE", Value: build.Spec.Image},
					{Name: "BUILDER_IMAGE", Value: build.Spec.Builder},
				},
			},
		},
	})
}

func buildMetadataFromBuiltImage(image registry.BuiltImage) []v1alpha1.CNBBuildpackMetadata {
	buildpackMetadata := make([]v1alpha1.CNBBuildpackMetadata, 0, len(image.BuildpackMetadata))
	for _, metadata := range image.BuildpackMetadata {
		buildpackMetadata = append(buildpackMetadata, v1alpha1.CNBBuildpackMetadata{
			ID:      metadata.ID,
			Version: metadata.Version,
		})
	}
	return buildpackMetadata
}
