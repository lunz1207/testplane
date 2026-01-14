package integrationtest

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/shared"
)

// patchStatus 使用纯正 SSA 更新 IntegrationTest 状态。
func (r *IntegrationTestReconciler) patchStatus(ctx context.Context, it *infrav1alpha1.IntegrationTest, _ infrav1alpha1.IntegrationTestStatus) error {
	return shared.PatchIntegrationTestStatusFromObject(ctx, r.Client, it)
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
		if statuses[i].State != shared.StateSucceeded {
			return i
		}
	}
	return len(statuses)
}

// stepAlreadyFinished 从 API Server 读取最新状态，检查步骤是否已完成。
// 用于在 patch 前检查，避免缓存延迟导致的重复事件。
func (r *IntegrationTestReconciler) stepAlreadyFinished(ctx context.Context, it *infrav1alpha1.IntegrationTest, stepIndex int) bool {
	if r.APIReader == nil {
		return false
	}
	var latest infrav1alpha1.IntegrationTest
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(it), &latest); err != nil {
		return false
	}
	if stepIndex < 0 || stepIndex >= len(latest.Status.Steps) {
		return false
	}
	return latest.Status.Steps[stepIndex].FinishedAt != nil
}

// testAlreadyCompleted 从 API Server 读取最新状态，检查测试是否已完成。
// 用于在 patch 前检查，避免缓存延迟导致的重复事件。
func (r *IntegrationTestReconciler) testAlreadyCompleted(ctx context.Context, it *infrav1alpha1.IntegrationTest) bool {
	if r.APIReader == nil {
		return false
	}
	var latest infrav1alpha1.IntegrationTest
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(it), &latest); err != nil {
		return false
	}
	return latest.Status.CompletionTime != nil
}

// Condition 类型常量
const (
	ConditionTypeSpecChangedIgnored = "SpecChangedIgnored"
)

// detectAndIgnoreSpecChange 检测运行中的 IntegrationTest spec 变更并忽略。
// 返回 true 表示检测到 spec 变更（已被忽略），调用方需要 patch 状态。
func (r *IntegrationTestReconciler) detectAndIgnoreSpecChange(_ context.Context, it *infrav1alpha1.IntegrationTest) bool {
	// 只有在已开始执行后（ObservedGeneration > 0）才检测变更
	if it.Status.ObservedGeneration == 0 {
		return false
	}

	// 检查 Generation 是否变化
	if it.Generation == it.Status.ObservedGeneration {
		return false
	}

	// spec 已变更，设置 Condition 警告用户
	shared.SetCondition(&it.Status.Conditions, ConditionTypeSpecChangedIgnored,
		metav1.ConditionTrue, "SpecModified",
		"spec was modified while integrationtest is running, changes are ignored",
		it.Generation)

	return true
}
