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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CNBImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CNBImageSpec   `json:"spec"`
	Status CNBImageStatus `json:"status"`
}

type CNBImageSpec struct {
	Image          string `json:"image"`
	BuilderRef     string `json:"builderRef"`
	ServiceAccount string `json:"serviceAccount"`
	GitURL         string `json:"gitUrl"`
	GitRevision    string `json:"gitRevision"`
}

type CNBImageStatus struct {
	duckv1alpha1.Status `json:",inline"`
	LastBuildRef        string `json:"lastBuildRef"`
	BuildCounter        int32  `json:"buildCounter"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CNBImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CNBImage `json:"items"`
}

func (*CNBImage) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CNBImage")
}
