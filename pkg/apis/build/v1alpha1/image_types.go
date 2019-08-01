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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Image struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ImageSpec   `json:"spec"`
	Status ImageStatus `json:"status"`
}

type ImageSpec struct {
	Tag                         string             `json:"tag"`
	BuilderRef                  string             `json:"builderRef"`
	ServiceAccount              string             `json:"serviceAccount"`
	Source                      Source             `json:"source"`
	CacheSize                   *resource.Quantity `json:"cacheSize,omitempty"`
	FailedBuildHistoryLimit     *int64             `json:"failedBuildHistoryLimit"`
	SuccessBuildHistoryLimit    *int64             `json:"successBuildHistoryLimit"`
	DisableAdditionalImageNames bool               `json:"disableAdditionalImageNames"`
	Build                       ImageBuild         `json:"build"`
}

type ImageBuild struct {
	Env       []corev1.EnvVar             `json:"env"`
	Resources corev1.ResourceRequirements `json:"resources"`
}

type ImageStatus struct {
	duckv1alpha1.Status `json:",inline"`
	LatestBuildRef      string `json:"latestBuildRef"`
	LatestImage         string `json:"latestImage"`
	BuildCounter        int64  `json:"buildCounter"`
	BuildCacheName      string `json:"buildCacheName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Image `json:"items"`
}

func (*Image) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Image")
}
