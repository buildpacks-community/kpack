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

package v1alpha1

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
	Tag                      string                 `json:"tag"`
	Builder                  corev1.ObjectReference `json:"builder,omitempty"`
	ServiceAccount           string                 `json:"serviceAccount,omitempty"`
	Source                   SourceConfig           `json:"source"`
	CacheSize                *resource.Quantity     `json:"cacheSize,omitempty"`
	FailedBuildHistoryLimit  *int64                 `json:"failedBuildHistoryLimit,omitempty"`
	SuccessBuildHistoryLimit *int64                 `json:"successBuildHistoryLimit,omitempty"`
	ImageTaggingStrategy     ImageTaggingStrategy   `json:"imageTaggingStrategy,omitempty"`
	Build                    *ImageBuild            `json:"build,omitempty"`
	Notary                   NotaryConfig           `json:"notary,omitempty"`
}

// +k8s:openapi-gen=true
type ImageBuilder struct {
	metav1.TypeMeta `json:",inline"`
	Name            string `json:"name"`
}

type ImageTaggingStrategy string

const (
	None        ImageTaggingStrategy = "None"
	BuildNumber ImageTaggingStrategy = "BuildNumber"
)

// +k8s:openapi-gen=true
type ImageBuild struct {
	// +listType
	Bindings Bindings `json:"bindings,omitempty"`
	// +listType
	Env       []corev1.EnvVar             `json:"env,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
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
