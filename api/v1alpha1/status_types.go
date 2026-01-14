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
