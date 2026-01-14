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
)

// ReadyCondition 就绪条件（用于 Target 就绪检查）。
// 等待模式：持续检查直到通过或超时。
type ReadyCondition struct {
	// TimeoutSeconds 总超时时间（秒）。
	// +kubebuilder:default=300
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// AllOf 所有期望都必须满足。
	AllOf []Expectation `json:"allOf,omitempty"`
	// AnyOf 任一期望满足即可。
	AnyOf []Expectation `json:"anyOf,omitempty"`
}

// HealthCheck 健康检查（用于运行期周期性断言）。
// 周期模式：按间隔检查，连续失败达阈值则失败。
type HealthCheck struct {
	// IntervalSeconds 检查间隔（秒）。
	// +kubebuilder:default=10
	IntervalSeconds int32 `json:"intervalSeconds,omitempty"`
	// TimeoutSeconds 单次检查超时（秒）。
	// +kubebuilder:default=10
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// FailureThreshold 连续失败阈值。
	// +kubebuilder:default=3
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
	// AllOf 所有期望都必须满足。
	AllOf []Expectation `json:"allOf,omitempty"`
	// AnyOf 任一期望满足即可。
	AnyOf []Expectation `json:"anyOf,omitempty"`
}

// TargetSpec 定义测试目标资源（单资源）。
type TargetSpec struct {
	// Resource 目标资源（单资源）。
	Resource ResourceRef `json:"resource"`
	// ReadyCondition 就绪条件（可选）。
	// 创建/更新 Target 后，等待此条件满足才继续执行后续步骤。
	ReadyCondition *ReadyCondition `json:"readyCondition,omitempty"`
}

// EnvInjection 环境变量注入定义。
// 使用 Extractor 从目标资源提取值注入环境变量。
type EnvInjection struct {
	// Name 环境变量名。
	Name string `json:"name"`
	// Extract 值提取器。
	Extract Extractor `json:"extract"`
}

// WorkloadSpec 负载资源定义。
type WorkloadSpec struct {
	// EnvInjection 环境变量注入列表（函数式）。
	EnvInjection []EnvInjection `json:"envInjection,omitempty"`
	// Resources 负载资源（多资源）。
	Resources []ResourceRef `json:"resources"`
}

// LoadTestSpec 定义负载测试规格。
type LoadTestSpec struct {
	// Target 被测目标资源。
	// 使用 Target.ReadyCondition 定义就绪条件，通过后才部署 Workload。
	Target TargetSpec `json:"target"`
	// Workload 负载资源定义。
	Workload WorkloadSpec `json:"workload"`
	// HealthCheck 运行期健康检查（周期性执行）。
	// 使用 IntervalSeconds（检查间隔）和 FailureThreshold（连续失败阈值）。
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
}

// LoadTestPhase 负载测试阶段。
// +kubebuilder:validation:Enum=Pending;Initializing;Running;Succeeded;Failed
type LoadTestPhase string

const (
	// LoadTestPending 等待开始。
	LoadTestPending LoadTestPhase = "Pending"
	// LoadTestInitializing 初始化阶段（应用 Target + 解析注入 + 等待就绪条件）。
	LoadTestInitializing LoadTestPhase = "Initializing"
	// LoadTestRunning 运行中。
	LoadTestRunning LoadTestPhase = "Running"
	// LoadTestSucceeded 成功。
	LoadTestSucceeded LoadTestPhase = "Succeeded"
	// LoadTestFailed 失败。
	LoadTestFailed LoadTestPhase = "Failed"
)

// HealthCheckStatus 健康检查状态。
type HealthCheckStatus struct {
	// LastCheckTime 上次检查时间。
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`
	// CheckCount 已检查次数。
	CheckCount int32 `json:"checkCount,omitempty"`
	// PassCount 通过次数。
	PassCount int32 `json:"passCount,omitempty"`
	// FailCount 失败次数。
	FailCount int32 `json:"failCount,omitempty"`
	// ConsecutiveFailures 连续失败次数。
	ConsecutiveFailures int32 `json:"consecutiveFailures,omitempty"`
	// LastResults 最近一次检查结果摘要。
	LastResults []ExpectationResultSummary `json:"lastResults,omitempty"`
}

// LoadTestStatus 记录负载测试状态。
type LoadTestStatus struct {
	// Phase 测试阶段。
	Phase LoadTestPhase `json:"phase,omitempty"`
	// Reason 阶段原因。
	Reason string `json:"reason,omitempty"`
	// Message 详细消息。
	Message string `json:"message,omitempty"`
	// StartTime 开始时间。
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// CompletionTime 完成时间。
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// InjectedValues 已注入的值（便于调试）。
	InjectedValues map[string]string `json:"injectedValues,omitempty"`
	// ReadyConditionStatus 就绪条件检查状态。
	ReadyConditionStatus *ReadyConditionStatus `json:"readyConditionStatus,omitempty"`
	// HealthCheckStatus 健康检查状态。
	HealthCheckStatus *HealthCheckStatus `json:"healthCheckStatus,omitempty"`
	// ObservedGeneration 已观察的 Generation。
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions 条件列表。
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Checks",type=integer,JSONPath=`.status.healthCheckStatus.checkCount`,priority=1
// +kubebuilder:printcolumn:name="Pass",type=integer,JSONPath=`.status.healthCheckStatus.passCount`,priority=1
// +kubebuilder:printcolumn:name="Fail",type=integer,JSONPath=`.status.healthCheckStatus.failCount`,priority=1
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.reason`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=lt

// LoadTest 表示一个负载测试。
type LoadTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadTestSpec   `json:"spec,omitempty"`
	Status LoadTestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LoadTestList 包含多个 LoadTest。
type LoadTestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LoadTest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&LoadTest{}, &LoadTestList{})
}
