/*
 * Copyright 2019 The original author or authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	BuildKind   = "Build"
	BuildCRName = "builds.kpack.io"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type Build struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildSpec   `json:"spec"`
	Status BuildStatus `json:"status,omitempty"`
}

var (
	_ apis.Validatable   = (*Build)(nil)
	_ apis.Defaultable   = (*Build)(nil)
	_ kmeta.OwnerRefable = (*Build)(nil)
)

// +k8s:openapi-gen=true
type BuildSpec struct {
	// +listType
	Tags                  []string                      `json:"tags,omitempty"`
	Builder               corev1alpha1.BuildBuilderSpec `json:"builder,omitempty"`
	ServiceAccountName    string                        `json:"serviceAccountName,omitempty"`
	Source                corev1alpha1.SourceConfig     `json:"source"`
	Cache                 *BuildCacheConfig             `json:"cache,omitempty"`
	RunImage              BuildSpecImage                `json:"runImage,omitempty"`
	ActiveDeadlineSeconds *int64                        `json:"activeDeadlineSeconds,omitempty"`
	// +listType
	Services Services `json:"services,omitempty"`
	// +listType
	CNBBindings corev1alpha1.CNBBindings `json:"cnbBindings,omitempty"`
	// +listType
	Env                   []corev1.EnvVar             `json:"env,omitempty"`
	ProjectDescriptorPath string                      `json:"projectDescriptorPath,omitempty"`
	Resources             corev1.ResourceRequirements `json:"resources,omitempty"`
	LastBuild             *LastBuild                  `json:"lastBuild,omitempty"`
	Notary                *corev1alpha1.NotaryConfig  `json:"notary,omitempty"`
	Cosign                *CosignConfig               `json:"cosign,omitempty"`
	DefaultProcess        string                      `json:"defaultProcess,omitempty"`
	// +listType
	Tolerations       []corev1.Toleration `json:"tolerations,omitempty"`
	NodeSelector      map[string]string   `json:"nodeSelector,omitempty"`
	Affinity          *corev1.Affinity    `json:"affinity,omitempty"`
	RuntimeClassName  *string             `json:"runtimeClassName,omitempty"`
	SchedulerName     string              `json:"schedulerName,omitempty"`
	PriorityClassName string              `json:"priorityClassName,omitempty"`
	CreationTime      string              `json:"creationTime,omitempty"`
}

func (bs *BuildSpec) RegistryCacheTag() string {
	if bs.Cache == nil || bs.Cache.Registry == nil {
		return ""
	}
	return bs.Cache.Registry.Tag
}

func (bs *BuildSpec) NeedVolumeCache() bool {
	return bs.Cache != nil && bs.Cache.Volume != nil && bs.Cache.Volume.ClaimName != ""
}

func (bs *BuildSpec) NeedRegistryCache() bool {
	return bs.Cache != nil && bs.Cache.Registry != nil && bs.Cache.Registry.Tag != ""
}

// +k8s:openapi-gen=true
type BuildCacheConfig struct {
	Volume   *BuildPersistentVolumeCache `json:"volume,omitempty"`
	Registry *RegistryCache              `json:"registry,omitempty"`
}

// +k8s:openapi-gen=true
type BuildPersistentVolumeCache struct {
	ClaimName string `json:"persistentVolumeClaimName,omitempty"`
}

// +k8s:openapi-gen=true
type Services []corev1.ObjectReference

// +k8s:openapi-gen=true
type LastBuild struct {
	Image   string     `json:"image,omitempty"`
	Cache   BuildCache `json:"cache,omitempty"`
	StackId string     `json:"stackId,omitempty"`
}

// +k8s:openapi-gen=true
type BuildCache struct {
	Image string `json:"image,omitempty"`
}

// +k8s:openapi-gen=true
type BuildStack struct {
	RunImage string `json:"runImage,omitempty"`
	ID       string `json:"id,omitempty"`
}

// +k8s:openapi-gen=true
type BuildStatus struct {
	corev1alpha1.Status `json:",inline"`
	BuildMetadata       corev1alpha1.BuildpackMetadataList `json:"buildMetadata,omitempty"`
	Stack               corev1alpha1.BuildStack            `json:"stack,omitempty"`
	LatestImage         string                             `json:"latestImage,omitempty"`
	LatestCacheImage    string                             `json:"latestCacheImage,omitempty"`
	PodName             string                             `json:"podName,omitempty"`
	// +listType
	StepStates []corev1.ContainerState `json:"stepStates,omitempty"`
	// +listType
	StepsCompleted []string `json:"stepsCompleted,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []Build `json:"items"`
}

// +k8s:openapi-gen=true
type BuildSpecImage struct {
	Image string `json:"image,omitempty"`
}
