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

// IntegrationTestMode 定义测试执行模式。
// +kubebuilder:validation:Enum=Sequential;Parallel
type IntegrationTestMode string

const (
	// IntegrationTestModeSequential 按步骤顺序执行，每步验证期望后再执行下一步。
	IntegrationTestModeSequential IntegrationTestMode = "Sequential"
	// IntegrationTestModeParallel 并行执行所有步骤，所有步骤同时开始，全部完成后验证期望。
	IntegrationTestModeParallel IntegrationTestMode = "Parallel"
)

// RepeatConfig 重复执行配置。
// 停止条件（满足任意一个即停止）：
// - 达到 Count 轮数（Count > 0 时生效）
// - 达到 MaxDurationSeconds 时间（MaxDurationSeconds > 0 时生效）
// - UntilFailure=true 且发生任何失败（断言失败、资源操作失败、超时等）
// - 如果以上都未设置/未触发，则永远执行直到 IntegrationTest 被删除
type RepeatConfig struct {
	// Count 重复轮数，0 表示不限轮数。
	Count int `json:"count,omitempty"`

	// MaxDurationSeconds 最大持续时间（秒），0 表示不限时间。
	MaxDurationSeconds int `json:"maxDurationSeconds,omitempty"`

	// UntilFailure 遇到任何失败后停止（断言失败、资源操作失败、超时等）。
	UntilFailure bool `json:"untilFailure,omitempty"`

	// DelayBetweenRounds 每轮之间的延迟（秒）。
	DelayBetweenRounds int `json:"delayBetweenRounds,omitempty"`
}

// StepCondition 步骤条件（用于 readyCondition 和 expectations）。
// 在步骤超时时间内持续检查直到所有期望通过。
type StepCondition struct {
	// TimeoutSeconds 单次检查超时（秒）。
	// +kubebuilder:default=10
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	// AllOf 所有期望都必须满足。
	AllOf []Expectation `json:"allOf,omitempty"`
	// AnyOf 任一期望满足即可。
	AnyOf []Expectation `json:"anyOf,omitempty"`
}

// TestStep 定义一个测试步骤（单资源）。
// Resource 中的 Manifest 和 Selector 互斥，只能指定其中一个：
// - Manifest：创建/更新/删除资源
// - Selector：引用已有资源（只读）
type TestStep struct {
	// Name 步骤名称。
	Name string `json:"name"`
	// Resource 步骤资源（单资源）。
	// +optional
	Resource *ResourceRef `json:"resource,omitempty"`
	// ReadyCondition 创建/更新资源后的就绪条件（步骤级）。
	// +optional
	ReadyCondition *StepCondition `json:"readyCondition,omitempty"`
	// Expectations 步骤执行后的业务预期。
	// +optional
	Expectations *StepCondition `json:"expectations,omitempty"`
	// TimeoutSeconds 步骤超时时间（秒），控制整个步骤的超时。
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// IntegrationTestSpec 定义测试用例的规格。
type IntegrationTestSpec struct {
	// Mode 测试执行模式：Sequential（顺序）或 Parallel（并行）。
	// - Sequential：按 steps 顺序依次执行
	// - Parallel：所有 steps 并行执行，全部完成后验证期望
	Mode IntegrationTestMode `json:"mode,omitempty"`
	// Steps 测试步骤列表。
	Steps []TestStep `json:"steps,omitempty"`
	// Repeat 重复执行配置，不设置则只执行一轮。
	Repeat *RepeatConfig `json:"repeat,omitempty"`
}

// IntegrationTestPhase 定义测试用例的阶段。
// +kubebuilder:validation:Enum=Pending;Running;Succeeded;Failed;Aborted
type IntegrationTestPhase string

const (
	IntegrationTestPhasePending   IntegrationTestPhase = "Pending"
	IntegrationTestPhaseRunning   IntegrationTestPhase = "Running"
	IntegrationTestPhaseSucceeded IntegrationTestPhase = "Succeeded"
	IntegrationTestPhaseFailed    IntegrationTestPhase = "Failed"
	IntegrationTestPhaseAborted   IntegrationTestPhase = "Aborted"
)

// StepStatus 记录步骤的执行状态。
type StepStatus struct {
	// Name 步骤名称。
	Name string `json:"name"`
	// Index 步骤序号（从 0 开始）。
	Index int `json:"index,omitempty"`
	// State 步骤状态：Succeeded, Failed, Running。
	State string `json:"state,omitempty"`
	// Reason 步骤失败原因。
	Reason string `json:"reason,omitempty"`
	// Message 步骤摘要。
	Message string `json:"message,omitempty"`
	// StartedAt 步骤开始时间。
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// Deadline 步骤截止时间（StartedAt + timeoutSeconds）。
	// Controller 重启后依据此字段继续计时。
	Deadline *metav1.Time `json:"deadline,omitempty"`
	// FinishedAt 步骤结束时间。
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`
	// ExpectationResults 期望结果摘要。
	ExpectationResults []ExpectationResultSummary `json:"expectationResults,omitempty"`
	// ReadyConditionStatus 就绪条件检查状态。
	ReadyConditionStatus *ReadyConditionStatus `json:"readyConditionStatus,omitempty"`
}

// IntegrationTestStatus 记录测试用例的状态和报告。
type IntegrationTestStatus struct {
	// Phase 测试阶段。
	Phase IntegrationTestPhase `json:"phase,omitempty"`
	// Reason 阶段原因（如 StepFailed、InitialConditionNotMet、Timeout）。
	Reason string `json:"reason,omitempty"`
	// Message 阶段消息。
	Message string `json:"message,omitempty"`
	// StartTime 开始时间。
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// CompletionTime 完成时间。
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`
	// ObservedGeneration 已观察到的 Generation。
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// CurrentStepIndex 当前执行到的步骤索引。
	CurrentStepIndex *int `json:"currentStepIndex,omitempty"`
	// CurrentRound 当前执行轮次（从 1 开始）。
	CurrentRound int `json:"currentRound,omitempty"`
	// CompletedRounds 已完成的轮次数。
	CompletedRounds int `json:"completedRounds,omitempty"`
	// Steps 步骤状态详情（当前轮次）。
	Steps []StepStatus `json:"steps,omitempty"`
	// Conditions 条件列表。
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Round",type=integer,JSONPath=`.status.currentRound`,priority=1
// +kubebuilder:printcolumn:name="Completed",type=integer,JSONPath=`.status.completedRounds`,priority=1
// +kubebuilder:printcolumn:name="Reason",type=string,JSONPath=`.status.reason`,priority=1
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:resource:shortName=it

// IntegrationTest 表示一个集成测试用例。
type IntegrationTest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IntegrationTestSpec   `json:"spec,omitempty"`
	Status IntegrationTestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IntegrationTestList 包含多个 IntegrationTest。
type IntegrationTestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IntegrationTest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IntegrationTest{}, &IntegrationTestList{})
}
