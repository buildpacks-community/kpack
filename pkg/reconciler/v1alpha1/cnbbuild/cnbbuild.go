package cnbbuild

import (
	"context"

	knv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	knversioned "github.com/knative/build/pkg/client/clientset/versioned"
	knv1alpha1lister "github.com/knative/build/pkg/client/listers/build/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	v1alpha1lister "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

type Reconciler struct {
	KNClient       knversioned.Interface
	CNBBuildClient versioned.Interface
	CNBLister      v1alpha1lister.CNBBuildLister
	KnLister       knv1alpha1lister.BuildLister
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, buildName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	build, err := c.CNBLister.CNBBuilds(namespace).Get(buildName)
	if err != nil {
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

func (c *Reconciler) reconcileStatus(s string, build *v1alpha1.CNBBuild, knBuild *knv1alpha1.Build) {
	if knBuild.Status.GetCondition(knv1alpha1.BuildSucceeded).IsTrue() {
		build.Status.Conditions = duckv1alpha1.Conditions{
			{
				Type:   "BuildSucceeded",
				Status: corev1.ConditionTrue,
			},
		}
	}
}
