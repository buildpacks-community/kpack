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
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/kmeta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CNBBuild struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CNBBuildSpec   `json:"spec"`
	Status CNBBuildStatus `json:"status"`
}

var (
	_ apis.Validatable   = (*CNBBuild)(nil)
	_ apis.Defaultable   = (*CNBBuild)(nil)
	_ kmeta.OwnerRefable = (*CNBBuild)(nil)
)

type CNBBuildSpec struct {
	Image          string `json:"image"`
	Builder        string `json:"builder"`
	ServiceAccount string `json:"serviceAccount"`
	GitURL         string `json:"gitUrl"`
	GitRevision    string `json:"gitRevision"`
}

type CNBBuildStatus struct {
	duckv1alpha1.Status `json:",inline"`
	BuildMetadata       []CNBBuildpackMetadata `json:"buildMetadata"`
}

type CNBBuildpackMetadata struct {
	ID      string `json:"key"`
	Version string `json:"version"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CNBBuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CNBBuild `json:"items"`
}

func (*CNBBuild) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CNBBuild")
}

func (b *CNBBuild) ServiceAccount() string {
	return b.Spec.ServiceAccount
}

func (b *CNBBuild) RepoName() string {
	return b.Spec.Image
}

func (b *CNBBuild) Namespace() string {
	return b.ObjectMeta.Namespace
}

func (in *CNBBuild) IsRunning() bool {
	if in == nil {
		return false
	}

	return in.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsUnknown()
}
