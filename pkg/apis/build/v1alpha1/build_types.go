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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

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

type BuildBuilderSpec struct {
	Image            string                        `json:"image,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`
}

type BuildSpec struct {
	Tags           []string                    `json:"tags,omitempty"`
	Builder        BuildBuilderSpec            `json:"builder,omitempty"`
	ServiceAccount string                      `json:"serviceAccount,omitempty"`
	Source         SourceConfig                `json:"source"`
	CacheName      string                      `json:"cacheName,omitempty"`
	Env            []corev1.EnvVar             `json:"env,omitempty"`
	Resources      corev1.ResourceRequirements `json:"resources,omitempty"`
	LastBuild      *LastBuild                  `json:"lastBuild,omitempty"`
}

type LastBuild struct {
	Image   string `json:"image,omitempty"`
	StackID string `json:"stackId,omitempty"`
}

type BuildStack struct {
	RunImage string `json:"runImage,omitempty"`
	ID       string `json:"id,omitempty"`
}

type BuildStatus struct {
	duckv1alpha1.Status `json:",inline"`
	BuildMetadata       BuildpackMetadataList   `json:"buildMetadata,omitempty"`
	Stack               BuildStack              `json:"stack,omitempty"`
	LatestImage         string                  `json:"latestImage,omitempty"`
	PodName             string                  `json:"podName,omitempty"`
	StepStates          []corev1.ContainerState `json:"stepStates,omitempty"`
	StepsCompleted      []string                `json:"stepsCompleted,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BuildList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Build `json:"items"`
}
