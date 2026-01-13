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

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

// Event 原因常量
const (
	// 测试生命周期
	EventReasonLoadTestStarted   = "LoadTestStarted"
	EventReasonLoadTestRunning   = "LoadTestRunning"
	EventReasonLoadTestSucceeded = "LoadTestSucceeded"
	EventReasonLoadTestFailed    = "LoadTestFailed"

	// Target 相关
	EventReasonTargetApplied      = "TargetApplied"
	EventReasonTargetReady        = "TargetReady"
	EventReasonTargetApplyFailed  = "TargetApplyFailed"
	EventReasonReadyConditionWait = "ReadyConditionWait"

	// Workload 相关
	EventReasonWorkloadApplied     = "WorkloadApplied"
	EventReasonWorkloadApplyFailed = "WorkloadApplyFailed"
)

// Condition 类型常量
const (
	// ConditionTypeReady 表示 LoadTest 是否处于正常运行状态
	ConditionTypeReady = "Ready"
	// ConditionTypeTargetReady 表示目标资源是否就绪
	ConditionTypeTargetReady = "TargetReady"
	// ConditionTypeExpectationsMet 表示断言检查是否通过
	ConditionTypeExpectationsMet = "ExpectationsMet"
)

// setCondition 设置或更新 LoadTest 的 Condition。
func setCondition(status *infrav1alpha1.LoadTestStatus, conditionType string, conditionStatus metav1.ConditionStatus, reason, message string, generation int64) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		ObservedGeneration: generation,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	// 查找并更新已有 Condition，或添加新的
	found := false
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			// 只有状态变化时才更新 LastTransitionTime
			if status.Conditions[i].Status != conditionStatus {
				status.Conditions[i] = condition
			} else {
				// 状态未变，只更新 Reason 和 Message
				status.Conditions[i].Reason = reason
				status.Conditions[i].Message = message
				status.Conditions[i].ObservedGeneration = generation
			}
			found = true
			break
		}
	}
	if !found {
		status.Conditions = append(status.Conditions, condition)
	}
}

// getOrDefaultInt32 返回非零值或默认值。
func getOrDefaultInt32(value, defaultVal int32) int32 {
	if value == 0 {
		return defaultVal
	}
	return value
}

// getDurationOrDefault 将秒数转换为 Duration，如果为 0 则返回默认值。
func getDurationOrDefault(seconds int32, defaultDuration time.Duration) time.Duration {
	if seconds == 0 {
		return defaultDuration
	}
	return time.Duration(seconds) * time.Second
}
