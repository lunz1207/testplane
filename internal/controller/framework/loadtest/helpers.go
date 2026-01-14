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

package loadtest

// Condition 类型常量
const (
	// ConditionTypeReady 表示 LoadTest 是否处于正常运行状态
	ConditionTypeReady = "Ready"
	// ConditionTypeTargetReady 表示目标资源是否就绪
	ConditionTypeTargetReady = "TargetReady"
	// ConditionTypeExpectationsMet 表示断言检查是否通过
	ConditionTypeExpectationsMet = "ExpectationsMet"
)

// getOrDefaultInt32 返回非零值或默认值。
func getOrDefaultInt32(value, defaultVal int32) int32 {
	if value == 0 {
		return defaultVal
	}
	return value
}
