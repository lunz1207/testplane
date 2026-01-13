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

package resource

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

// ExpandedManifest 展开后的资源清单。
// 将 ManifestAction 或 Template 转换为可直接操作的格式。
type ExpandedManifest struct {
	// Object 展开后的 Kubernetes 资源对象。
	Object *unstructured.Unstructured
	// Action 操作类型（Apply 或 Delete）。
	Action infrav1alpha1.TemplateAction
}

// StateKey 生成状态 map 的 key，格式为 "{apiVersion}/{kind}/{name}"。
func (e *ExpandedManifest) StateKey() string {
	return fmt.Sprintf("%s/%s/%s",
		e.Object.GetAPIVersion(),
		e.Object.GetKind(),
		e.Object.GetName())
}

// IsApply 判断是否为 Apply 操作。
func (e *ExpandedManifest) IsApply() bool {
	return e.Action == "" || e.Action == infrav1alpha1.TemplateActionApply
}

// IsDelete 判断是否为 Delete 操作。
func (e *ExpandedManifest) IsDelete() bool {
	return e.Action == infrav1alpha1.TemplateActionDelete
}
