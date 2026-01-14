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
	"encoding/json"
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

// ExpandResourceRef 展开单个 ResourceRef（支持 List/数组）。
func ExpandResourceRef(ref infrav1alpha1.ResourceRef, defaultNamespace string) ([]ExpandedManifest, error) {
	if len(ref.Manifest.Raw) == 0 {
		return nil, fmt.Errorf("manifest is empty")
	}

	action := ref.Action
	if action == "" {
		action = infrav1alpha1.TemplateActionApply
	}

	return expandRaw(ref.Manifest.Raw, defaultNamespace, action)
}

// ExpandSingleResourceRef 展开单个 ResourceRef 为单个 ExpandedManifest。
// 如果 manifest 包含 List 或数组，返回错误。
func ExpandSingleResourceRef(ref infrav1alpha1.ResourceRef, defaultNamespace string) (*ExpandedManifest, error) {
	if len(ref.Manifest.Raw) == 0 {
		return nil, fmt.Errorf("manifest is empty")
	}

	action := ref.Action
	if action == "" {
		action = infrav1alpha1.TemplateActionApply
	}

	var data map[string]interface{}
	if err := json.Unmarshal(ref.Manifest.Raw, &data); err != nil {
		return nil, fmt.Errorf("unmarshal template: %w", err)
	}

	// 检查是否是 List
	if _, ok := data["items"]; ok {
		return nil, fmt.Errorf("manifest contains list, expected single object")
	}

	manifest, err := toExpandedManifest(data, defaultNamespace, action)
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

// ExpandResourceRefs 展开多个 ResourceRef（支持 List/数组）。
// replacements 用于对模板内容进行占位符替换（用于 workload 注入）。
func ExpandResourceRefs(refs []infrav1alpha1.ResourceRef, defaultNamespace string, replacements map[string]string) ([]ExpandedManifest, error) {
	if len(refs) == 0 {
		return nil, nil
	}

	var result []ExpandedManifest
	for _, ref := range refs {
		expanded, err := ExpandResourceRefWithReplacements(ref, defaultNamespace, replacements)
		if err != nil {
			return nil, err
		}
		result = append(result, expanded...)
	}

	return result, nil
}

// ExpandResourceRefWithReplacements 展开单个 ResourceRef 并应用占位符替换。
func ExpandResourceRefWithReplacements(ref infrav1alpha1.ResourceRef, defaultNamespace string, replacements map[string]string) ([]ExpandedManifest, error) {
	if len(ref.Manifest.Raw) == 0 {
		return nil, fmt.Errorf("manifest is empty")
	}

	action := ref.Action
	if action == "" {
		action = infrav1alpha1.TemplateActionApply
	}

	raw := ref.Manifest.Raw
	if len(replacements) > 0 {
		raw = ApplyReplacements(raw, replacements)
	}

	return expandRaw(raw, defaultNamespace, action)
}

// ExpandRawTemplate 展开单个 RawExtension 模板（供 LoadTest Target 使用）。
func ExpandRawTemplate(template *runtime.RawExtension, defaultNamespace string) (*ExpandedManifest, error) {
	if template == nil || len(template.Raw) == 0 {
		return nil, fmt.Errorf("template is empty")
	}

	results, err := expandRaw(template.Raw, defaultNamespace, infrav1alpha1.TemplateActionApply)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no resources found in template")
	}

	if len(results) > 1 {
		return nil, fmt.Errorf("template contains multiple resources, expected single resource")
	}

	return &results[0], nil
}

// ApplyReplacements 使用 ${VAR} 形式的占位符做字符串替换。
func ApplyReplacements(raw []byte, replacements map[string]string) []byte {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	content := re.ReplaceAllStringFunc(string(raw), func(match string) string {
		key := match[2 : len(match)-1]
		if val, ok := replacements[key]; ok {
			return val
		}
		return match
	})
	return []byte(content)
}

// expandRaw 将 JSON 原始数据展开为 ExpandedManifest 列表。
func expandRaw(raw []byte, defaultNamespace string, action infrav1alpha1.TemplateAction) ([]ExpandedManifest, error) {
	var data interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("unmarshal template: %w", err)
	}

	switch obj := data.(type) {
	case map[string]interface{}:
		// 如果包含 items，则按 List 处理
		if items, ok := obj["items"].([]interface{}); ok {
			return expandItemList(items, defaultNamespace, action)
		}
		manifest, err := toExpandedManifest(obj, defaultNamespace, action)
		if err != nil {
			return nil, err
		}
		return []ExpandedManifest{manifest}, nil
	case []interface{}:
		return expandItemList(obj, defaultNamespace, action)
	default:
		return nil, fmt.Errorf("template must be object or list")
	}
}

// expandItemList 将 List/数组展开为 ExpandedManifest 列表。
func expandItemList(items []interface{}, defaultNamespace string, action infrav1alpha1.TemplateAction) ([]ExpandedManifest, error) {
	result := make([]ExpandedManifest, 0, len(items))
	for i, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("list item %d is not an object", i)
		}
		manifest, err := toExpandedManifest(obj, defaultNamespace, action)
		if err != nil {
			return nil, fmt.Errorf("list item %d: %w", i, err)
		}
		result = append(result, manifest)
	}
	return result, nil
}

// toExpandedManifest 将模板对象转换为 ExpandedManifest。
func toExpandedManifest(obj map[string]interface{}, defaultNamespace string, action infrav1alpha1.TemplateAction) (ExpandedManifest, error) {
	apiVersion, _ := obj["apiVersion"].(string)
	kind, _ := obj["kind"].(string)
	if apiVersion == "" || kind == "" {
		return ExpandedManifest{}, fmt.Errorf("apiVersion and kind are required")
	}

	metadata, _ := obj["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	if name == "" {
		return ExpandedManifest{}, fmt.Errorf("metadata.name is required")
	}

	namespace := defaultNamespace
	if ns, ok := metadata["namespace"].(string); ok && ns != "" {
		namespace = ns
	}

	// 构建 Unstructured 对象
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetName(name)
	u.SetNamespace(namespace)

	// 从 metadata 中提取 labels 和 annotations
	if labels, ok := metadata["labels"].(map[string]interface{}); ok {
		labelMap := make(map[string]string, len(labels))
		for k, v := range labels {
			if s, ok := v.(string); ok {
				labelMap[k] = s
			}
		}
		u.SetLabels(labelMap)
	}
	if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
		annotationMap := make(map[string]string, len(annotations))
		for k, v := range annotations {
			if s, ok := v.(string); ok {
				annotationMap[k] = s
			}
		}
		u.SetAnnotations(annotationMap)
	}

	// 对于 Apply 操作，提取内容字段（spec, data 等）
	if action == infrav1alpha1.TemplateActionApply {
		for k, v := range obj {
			if k == "apiVersion" || k == "kind" || k == "metadata" {
				continue
			}
			if err := unstructured.SetNestedField(u.Object, v, k); err != nil {
				return ExpandedManifest{}, fmt.Errorf("set field %q: %w", k, err)
			}
		}
	}

	return ExpandedManifest{
		Object: u,
		Action: action,
	}, nil
}
