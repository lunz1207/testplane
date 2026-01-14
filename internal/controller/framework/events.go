package framework

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// 共享的 Event 原因
const (
	EventReasonExpectationPassed = "ExpectationPassed"
	EventReasonExpectationFailed = "ExpectationFailed"
)

// IntegrationTest Event 原因常量
const (
	EventReasonIntegrationTestStarted   = "IntegrationTestStarted"
	EventReasonIntegrationTestSucceeded = "IntegrationTestSucceeded"
	EventReasonIntegrationTestFailed    = "IntegrationTestFailed"
	EventReasonIntegrationTestTimeout   = "IntegrationTestTimeout"

	EventReasonStepStarted   = "StepStarted"
	EventReasonStepSucceeded = "StepSucceeded"
	EventReasonStepFailed    = "StepFailed"
)

// LoadTest Event 原因常量
const (
	EventReasonLoadTestStarted   = "LoadTestStarted"
	EventReasonLoadTestRunning   = "LoadTestRunning"
	EventReasonLoadTestSucceeded = "LoadTestSucceeded"
	EventReasonLoadTestFailed    = "LoadTestFailed"

	EventReasonTargetApplied      = "TargetApplied"
	EventReasonTargetReady        = "TargetReady"
	EventReasonTargetApplyFailed  = "TargetApplyFailed"
	EventReasonReadyConditionWait = "ReadyConditionWait"

	EventReasonWorkloadApplied     = "WorkloadApplied"
	EventReasonWorkloadApplyFailed = "WorkloadApplyFailed"
)

// EventRecorder 定义事件记录器接口
type EventRecorder interface {
	Event(object runtime.Object, eventtype, reason, message string)
}

// EmitEvent 发送 Kubernetes Event
func EmitEvent(recorder EventRecorder, obj runtime.Object, eventType, reason, message string) {
	if recorder == nil || obj == nil {
		return
	}
	recorder.Event(obj, eventType, reason, message)
}

// EmitNormalEvent 发送 Normal 类型事件
func EmitNormalEvent(recorder EventRecorder, obj runtime.Object, reason, message string) {
	EmitEvent(recorder, obj, corev1.EventTypeNormal, reason, message)
}

// EmitWarningEvent 发送 Warning 类型事件
func EmitWarningEvent(recorder EventRecorder, obj runtime.Object, reason, message string) {
	EmitEvent(recorder, obj, corev1.EventTypeWarning, reason, message)
}
