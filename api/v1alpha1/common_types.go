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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Expectation 定义一个业务期望。
// 支持两种模式：
// 1. 内置函数：Function + Params（可选）
// 2. Webhook：Function + Webhook + Params（可选）
// 断言的资源由上下文自动确定：
// - IntegrationTest: 使用当前 Step 的资源（template 或 selector）
// - LoadTest: 使用 Target 资源
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

// ResourceSelector 资源选择器（只读）。
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

// TemplateAction 定义资源模板的操作类型。
// +kubebuilder:validation:Enum=Apply;Delete
type TemplateAction string

const (
	TemplateActionApply  TemplateAction = "Apply"
	TemplateActionDelete TemplateAction = "Delete"
)

// ResourceTemplate 资源模板（完整 K8s 对象或 List）。
// 使用 TemplateAction 控制 Apply 或 Delete。
// Deprecated: 使用 ManifestAction 替代。
type ResourceTemplate struct {
	// Name 模板名称（可选，用于错误消息和调试）。
	// +optional
	Name string `json:"name,omitempty"`
	// Template 完整的 K8s 对象或 List。
	// +optional
	Template runtime.RawExtension `json:"template,omitempty"`
	// Action 模板应用动作（默认 Apply）。
	// +kubebuilder:default=Apply
	// +optional
	Action TemplateAction `json:"action,omitempty"`
}

// ManifestAction 资源清单与操作（简化版，用于 IntegrationTest）。
// manifest 支持单个 K8s 对象、List 对象或数组。
type ManifestAction struct {
	// Manifest K8s 资源清单（支持单个对象、List 或数组）。
	// +kubebuilder:pruning:PreserveUnknownFields
	Manifest runtime.RawExtension `json:"manifest"`
	// Action 操作类型（默认 Apply）。
	// +kubebuilder:default=Apply
	// +optional
	Action TemplateAction `json:"action,omitempty"`
}

// ResourcesSpec 资源管理规格（统一 selectors 与 templates）。
type ResourcesSpec struct {
	// Selectors 只读引用的资源列表。
	// +optional
	Selectors []ResourceSelector `json:"selectors,omitempty"`
	// Templates 要创建/更新/删除的资源模板（支持 List/数组）。
	// +optional
	Templates []ResourceTemplate `json:"templates,omitempty"`
}

// WaitCondition 等待条件（统一的断言配置）。
// 支持两种使用模式，业务方按需配置：
//
// 1. 等待模式（用于 IntegrationTest 的 readyCondition、step expectations）：
//   - Step.TimeoutSeconds 控制步骤总超时
//   - WaitCondition.TimeoutSeconds 控制单次断言执行超时
//   - 语义：在步骤超时时间内持续检查直到所有期望通过
//
// 2. 周期模式（用于 LoadTest 运行期断言）：
//   - IntervalSeconds 设置检查间隔
//   - FailureThreshold 设置连续失败阈值
//   - TimeoutSeconds 控制单次检查超时
//   - 语义：按间隔周期检查，连续失败达阈值则失败
type WaitCondition struct {
	// TimeoutSeconds 单次断言执行超时（秒）。
	// 控制单次检查的最大执行时间，超时则本次检查失败。
	// +kubebuilder:default=10
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// IntervalSeconds 检查间隔（秒），仅用于周期模式。
	// +kubebuilder:default=10
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`
	// FailureThreshold 连续失败次数阈值，达到后判定失败，仅用于周期模式。
	// +kubebuilder:default=3
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
	// AllOf 所有期望都必须满足。
	AllOf []Expectation `json:"allOf,omitempty"`
	// AnyOf 任一期望满足即可（可选）。
	AnyOf []Expectation `json:"anyOf,omitempty"`
}

// TargetSpec 定义测试目标资源（单资源）。
// Template 和 Selector 互斥，只能指定其中一个：
// - Template：创建/更新资源
// - Selector：引用已有资源
type TargetSpec struct {
	// Template 目标资源模板（与 Selector 互斥）。
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Template *runtime.RawExtension `json:"template,omitempty"`
	// Selector 目标资源选择器（与 Template 互斥）。
	// +optional
	Selector *ResourceSelector `json:"selector,omitempty"`
	// ReadyCondition 就绪条件（可选）。
	// 创建/更新 Target 后，等待此条件满足才继续执行后续步骤。
	ReadyCondition *WaitCondition `json:"readyCondition,omitempty"`
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

// 标准 Condition 类型常量。
const (
	// ConditionReady 资源就绪条件。
	ConditionReady = "Ready"
	// ConditionProgressing 资源进行中条件。
	ConditionProgressing = "Progressing"
	// ConditionComplete 资源完成条件（用于 IntegrationTest）。
	ConditionComplete = "Complete"
	// ConditionTargetReady 目标资源就绪条件（用于 LoadTest）。
	ConditionTargetReady = "TargetReady"
	// ConditionWorkloadReady 工作负载就绪条件（用于 LoadTest）。
	ConditionWorkloadReady = "WorkloadReady"
)

// Condition Reason 常量。
const (
	// ReasonInitializing 初始化中。
	ReasonInitializing = "Initializing"
	// ReasonRunning 运行中。
	ReasonRunning = "Running"
	// ReasonSucceeded 成功。
	ReasonSucceeded = "Succeeded"
	// ReasonFailed 失败。
	ReasonFailed = "Failed"
	// ReasonTimeout 超时。
	ReasonTimeout = "Timeout"
	// ReasonWaitingForResource 等待资源。
	ReasonWaitingForResource = "WaitingForResource"
)

// ReadyConditionStatus 记录就绪条件检查状态。
type ReadyConditionStatus struct {
	// State 状态：Pending, Passed, Failed。
	State string `json:"state,omitempty"`
	// StartedAt 开始时间。
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// Deadline 截止时间。
	Deadline *metav1.Time `json:"deadline,omitempty"`
	// FinishedAt 完成时间。
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`
	// Results 期望结果。
	Results []ExpectationResult `json:"results,omitempty"`
}
