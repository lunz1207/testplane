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

package plugin

import "strings"

// GetMap 获取 map 字段。
func GetMap(data map[string]interface{}, key string) map[string]interface{} {
	if data == nil {
		return nil
	}
	if v, ok := data[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

// GetString 获取字符串字段。
func GetString(data map[string]interface{}, key string) string {
	if data == nil {
		return ""
	}
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

// GetInt 获取整数字段（支持 int, int64, float64）。
func GetInt(data map[string]interface{}, key string) int {
	if data == nil {
		return 0
	}
	switch v := data[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

// GetBoolOr 获取布尔字段，不存在时返回默认值。
func GetBoolOr(data map[string]interface{}, key string, def bool) bool {
	if data == nil {
		return def
	}
	if _, ok := data[key]; !ok {
		return def
	}
	if v, ok := data[key].(bool); ok {
		return v
	}
	return def
}

// GetSlice 获取数组字段。
func GetSlice(data map[string]interface{}, key string) []interface{} {
	if data == nil {
		return nil
	}
	if v, ok := data[key].([]interface{}); ok {
		return v
	}
	return nil
}

// GetNestedMap 获取嵌套 map 字段（支持 "a.b.c" 形式的路径）。
func GetNestedMap(data map[string]interface{}, path string) map[string]interface{} {
	if data == nil || path == "" {
		return nil
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil
		}
		if i == len(parts)-1 {
			if v, ok := current[part].(map[string]interface{}); ok {
				return v
			}
			return nil
		}
		current = GetMap(current, part)
	}

	return current
}

// GetNestedString 获取嵌套字符串字段（支持 "a.b.c" 形式的路径）。
func GetNestedString(data map[string]interface{}, path string) string {
	if data == nil || path == "" {
		return ""
	}

	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return GetString(data, path)
	}

	// 获取除最后一个 key 外的路径
	parentPath := strings.Join(parts[:len(parts)-1], ".")
	parent := GetNestedMap(data, parentPath)
	if parent == nil {
		return ""
	}

	return GetString(parent, parts[len(parts)-1])
}

// GetNestedInt 获取嵌套整数字段（支持 "a.b.c" 形式的路径）。
func GetNestedInt(data map[string]interface{}, path string) int {
	if data == nil || path == "" {
		return 0
	}

	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return GetInt(data, path)
	}

	parentPath := strings.Join(parts[:len(parts)-1], ".")
	parent := GetNestedMap(data, parentPath)
	if parent == nil {
		return 0
	}

	return GetInt(parent, parts[len(parts)-1])
}

// GetNestedSlice 获取嵌套数组字段（支持 "a.b.c" 形式的路径）。
func GetNestedSlice(data map[string]interface{}, path string) []interface{} {
	if data == nil || path == "" {
		return nil
	}

	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		return GetSlice(data, path)
	}

	parentPath := strings.Join(parts[:len(parts)-1], ".")
	parent := GetNestedMap(data, parentPath)
	if parent == nil {
		return nil
	}

	return GetSlice(parent, parts[len(parts)-1])
}
