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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type Image struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImageSpec   `json:"spec"`
	Status ImageStatus `json:"status,omitempty"`
}

// +k8s:openapi-gen=true
type ImageSpec struct {
	Tag                      string                            `json:"tag"`
	Builder                  corev1.ObjectReference            `json:"builder,omitempty"`
	ServiceAccount           string                            `json:"serviceAccount,omitempty"`
	Source                   corev1alpha1.SourceConfig         `json:"source"`
	Cache                    *ImageCacheConfig                 `json:"cache,omitempty"`
	FailedBuildHistoryLimit  *int64                            `json:"failedBuildHistoryLimit,omitempty"`
	SuccessBuildHistoryLimit *int64                            `json:"successBuildHistoryLimit,omitempty"`
	ImageTaggingStrategy     corev1alpha1.ImageTaggingStrategy `json:"imageTaggingStrategy,omitempty"`
	ProjectDescriptorPath    string                            `json:"projectDescriptorPath,omitempty"`
	Build                    *corev1alpha1.ImageBuild          `json:"build,omitempty"`
	Notary                   *corev1alpha1.NotaryConfig        `json:"notary,omitempty"`
	DefaultProcess           string                            `json:"defaultProcess,omitempty"`
}

// +k8s:openapi-gen=true
type ImageCacheConfig struct {
	Volume   *ImagePersistentVolumeCache `json:"volume,omitempty"`
	Registry *RegistryCache              `json:"registry,omitempty"`
}

// +k8s:openapi-gen=true
type ImagePersistentVolumeCache struct {
	Size *resource.Quantity `json:"size,omitempty"`
}

// +k8s:openapi-gen=true
type RegistryCache struct {
	Tag string `json:"tag"`
}

// +k8s:openapi-gen=true
type ImageBuilder struct {
	metav1.TypeMeta `json:",inline"`
	Name            string `json:"name"`
}

// +k8s:openapi-gen=true
type ImageStatus struct {
	corev1alpha1.Status        `json:",inline"`
	LatestBuildRef             string `json:"latestBuildRef,omitempty"`
	LatestBuildImageGeneration int64  `json:"latestBuildImageGeneration,omitempty"`
	LatestImage                string `json:"latestImage,omitempty"`
	LatestStack                string `json:"latestStack,omitempty"`
	BuildCounter               int64  `json:"buildCounter,omitempty"`
	BuildCacheName             string `json:"buildCacheName,omitempty"`
	LatestBuildReason          string `json:"latestBuildReason,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []Image `json:"items"`
}

func (*Image) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Image")
}

func (i *Image) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: i.Namespace, Name: i.Name}
}

const ConditionBuilderReady corev1alpha1.ConditionType = "BuilderReady"
