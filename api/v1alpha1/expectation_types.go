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

// Expectation 定义一个业务期望。
// 支持两种模式：
// 1. 内置函数：Function + Params（可选）
// 2. Webhook：Function + Webhook + Params（可选）
type Expectation struct {
	// Function 函数名（必填）。
	// - 无 Webhook 时：调用内置函数
	// - 有 Webhook 时：传给 Webhook 表示执行哪个检查
	Function string `json:"function"`
	// Webhook 外部服务地址（可选）。
	// 有值时调用 Webhook，无值时调用内置函数。
	Webhook string `json:"webhook,omitempty"`
	// Params 函数参数（可选）。
	Params runtime.RawExtension `json:"params,omitempty"`
}

// Extractor 定义值提取器（用于 EnvInjection）。
type Extractor struct {
	// Function 提取函数名。
	Function string `json:"function"`
	// Params 函数参数。
	Params runtime.RawExtension `json:"params,omitempty"`
}

// ExpectationResult 记录单个期望的执行结果。
type ExpectationResult struct {
	// Expect 期望函数名称。
	Expect string `json:"expect"`
	// Params 期望函数的参数。
	Params runtime.RawExtension `json:"params,omitempty"`
	// Passed 是否通过。
	Passed bool `json:"passed"`
	// Actual 实际值。
	Actual string `json:"actual,omitempty"`
	// Message 结果消息。
	Message string `json:"message,omitempty"`
}

// ExpectationResultSummary 期望结果摘要（不含完整参数，用于状态存储优化）。
// 用于在状态中存储历史检查结果，减少状态大小。
type ExpectationResultSummary struct {
	// Expect 期望函数名称。
	Expect string `json:"expect"`
	// Passed 是否通过。
	Passed bool `json:"passed"`
	// Actual 实际值。
	Actual string `json:"actual,omitempty"`
	// Message 结果消息（截断至 256 字符）。
	Message string `json:"message,omitempty"`
}
