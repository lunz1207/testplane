package integrationtest

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
)

// Event 原因常量（IntegrationTest 特有）
const (
	// 测试生命周期
	EventReasonIntegrationTestStarted   = "IntegrationTestStarted"
	EventReasonIntegrationTestSucceeded = "IntegrationTestSucceeded"
	EventReasonIntegrationTestFailed    = "IntegrationTestFailed"
	EventReasonIntegrationTestTimeout   = "IntegrationTestTimeout"

	// 步骤执行
	EventReasonStepStarted   = "StepStarted"
	EventReasonStepSucceeded = "StepSucceeded"
	EventReasonStepFailed    = "StepFailed"
)

// patchStatus 使用纯正 SSA 更新 IntegrationTest 状态。
// 构造干净的 Apply Configuration，不依赖 GET 到的对象。
func (r *IntegrationTestReconciler) patchStatus(ctx context.Context, it *infrav1alpha1.IntegrationTest, status infrav1alpha1.IntegrationTestStatus) error {
	return framework.PatchIntegrationTestStatus(ctx, r.Client, it.Name, it.Namespace, status)
}

// ensureStepStatus 确保步骤状态存在并填充超时信息。
// 注意：首次创建时 State 为空，由调用方在 apply 资源后设置为 Running。
func (r *IntegrationTestReconciler) ensureStepStatus(status *infrav1alpha1.IntegrationTestStatus, idx int, step infrav1alpha1.TestStep) *infrav1alpha1.StepStatus {
	// 初始化
	if len(status.Steps) <= idx {
		now := metav1.Now()
		deadline := metav1.NewTime(stepDeadline(now.Time, step))
		status.Steps = append(status.Steps, infrav1alpha1.StepStatus{
			Name: step.Name,
			// State 初始为空，由调用方在 apply 资源后设置
			StartedAt: &now,
			Deadline:  &deadline,
			Index:     idx,
		})
	}

	st := &status.Steps[idx]
	if st.StartedAt == nil {
		now := metav1.Now()
		st.StartedAt = &now
	}
	if st.Deadline == nil {
		dl := metav1.NewTime(stepDeadline(st.StartedAt.Time, step))
		st.Deadline = &dl
	}
	return st
}

// stepDeadline 计算步骤截止时间，基于 step.timeoutSeconds（未设置则默认 10 分钟）。
func stepDeadline(start time.Time, step infrav1alpha1.TestStep) time.Time {
	return start.Add(stepTimeout(step))
}

// stepTimedOut 检查步骤是否超时。
func (r *IntegrationTestReconciler) stepTimedOut(st *infrav1alpha1.StepStatus) bool {
	if st == nil || st.Deadline == nil {
		return false
	}
	return time.Now().After(st.Deadline.Time)
}

// stepTimeout 获取步骤超时时间（未设置则默认 10 分钟）。
// 步骤超时是整个步骤的总上限，包括 apply、等待收敛、readyCondition 和期望检查。
func stepTimeout(step infrav1alpha1.TestStep) time.Duration {
	if step.TimeoutSeconds > 0 {
		return time.Duration(step.TimeoutSeconds) * time.Second
	}
	return defaultStepTimeout
}

// isTerminalPhase 检查是否为终态。
func isTerminalPhase(phase infrav1alpha1.IntegrationTestPhase) bool {
	return phase == infrav1alpha1.IntegrationTestPhaseSucceeded ||
		phase == infrav1alpha1.IntegrationTestPhaseFailed ||
		phase == infrav1alpha1.IntegrationTestPhaseAborted
}

// nextStepIndex 返回第一个未成功的步骤索引；若都成功则返回 len(statuses)。
func nextStepIndex(statuses []infrav1alpha1.StepStatus) int {
	for i := range statuses {
		if statuses[i].State != framework.StateSucceeded {
			return i
		}
	}
	return len(statuses)
}

// Condition 类型常量
const (
	ConditionTypeSpecChangedIgnored = "SpecChangedIgnored"
)

// detectAndIgnoreSpecChange 检测运行中的 IntegrationTest spec 变更并忽略。
// 返回 true 表示检测到 spec 变更（已被忽略）。
func (r *IntegrationTestReconciler) detectAndIgnoreSpecChange(_ context.Context, it *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) bool {
	// 只有在已开始执行后（ObservedGeneration > 0）才检测变更
	if status.ObservedGeneration == 0 {
		return false
	}

	// 检查 Generation 是否变化
	if it.Generation == status.ObservedGeneration {
		return false
	}

	// spec 已变更，设置 Condition 警告用户
	condition := metav1.Condition{
		Type:               ConditionTypeSpecChangedIgnored,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: it.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             "SpecModified",
		Message:            "spec was modified while integrationtest is running, changes are ignored",
	}

	// 更新或添加 Condition
	found := false
	for i := range status.Conditions {
		if status.Conditions[i].Type == ConditionTypeSpecChangedIgnored {
			status.Conditions[i] = condition
			found = true
			break
		}
	}
	if !found {
		status.Conditions = append(status.Conditions, condition)
	}

	return true
}
