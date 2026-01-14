/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// TemplateAction 定义资源操作类型。
// +kubebuilder:validation:Enum=Apply;Delete
type TemplateAction string

const (
	TemplateActionApply  TemplateAction = "Apply"
	TemplateActionDelete TemplateAction = "Delete"
)

// ResourceSelector 资源选择器（只读引用）。
// 支持三种互斥的选择方式：
// 1. Name：按名称精确选择单个资源
// 2. LabelSelector：按标签选择资源
// 3. AnnotationSelector：按注解选择资源
type ResourceSelector struct {
	// APIVersion 资源的 API 版本。
	APIVersion string `json:"apiVersion"`
	// Kind 资源的类型。
	Kind string `json:"kind"`
	// Namespace 资源的命名空间，为空时使用父资源的命名空间。
	Namespace string `json:"namespace,omitempty"`
	// Name 资源名称（与 LabelSelector/AnnotationSelector 互斥）。
	Name string `json:"name,omitempty"`
	// LabelSelector 标签选择器（与 Name、AnnotationSelector 互斥）。
	LabelSelector map[string]string `json:"labelSelector,omitempty"`
	// AnnotationSelector 注解选择器（与 Name、LabelSelector 互斥）。
	AnnotationSelector map[string]string `json:"annotationSelector,omitempty"`
}

// ResourceRef 单资源引用（扁平化）。
// Manifest 和 Selector 互斥，指定其中一个。
type ResourceRef struct {
	// Manifest K8s 资源清单（与 Selector 互斥）。
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Manifest runtime.RawExtension `json:"manifest,omitempty"`
	// Selector 资源选择器（与 Manifest 互斥）。
	// +optional
	Selector *ResourceSelector `json:"selector,omitempty"`
	// Action 操作类型（仅 Manifest 有效，默认 Apply）。
	// +kubebuilder:default=Apply
	// +optional
	Action TemplateAction `json:"action,omitempty"`
}
