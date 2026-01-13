package integrationtest

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
)

// 注意：本文件中的函数采用分散 patch 模式
// 在发送 Event 之前先 patch 状态，避免 Event 重复

// lifecycle.go 包含 IntegrationTest 资源的生命周期管理和状态设置函数

// initializeTest 初始化测试状态并持久化。
func (r *IntegrationTestReconciler) initializeTest(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	now := metav1.Now()
	it.Status.Phase = infrav1alpha1.IntegrationTestPhasePending
	it.Status.StartTime = &now
	it.Status.ObservedGeneration = it.Generation

	if err := r.patchStatus(ctx, it, it.Status); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// setStepSucceeded 设置步骤为成功状态。
func setStepSucceeded(stepStatus *infrav1alpha1.StepStatus) {
	stepStatus.State = framework.StateSucceeded
	stepStatus.Reason = framework.ReasonSucceeded
	now := metav1.Now()
	stepStatus.FinishedAt = &now
}

// setStepFailed 设置步骤为失败状态。
func setStepFailed(status *infrav1alpha1.IntegrationTestStatus, stepStatus *infrav1alpha1.StepStatus, stepName, reason, message string) {
	stepStatus.State = framework.StateFailed
	stepStatus.Reason = reason
	stepStatus.Message = message
	now := metav1.Now()
	stepStatus.FinishedAt = &now

	status.Phase = infrav1alpha1.IntegrationTestPhaseFailed
	status.CompletionTime = &now
	// 传递实际的失败原因（如 Timeout、Failed 等）
	if reason == framework.ReasonTimeout {
		status.Reason = "Timeout"
	} else {
		status.Reason = "StepFailed"
	}
	status.Message = "step " + stepName + " failed: " + message
}

// setSucceeded 设置 IntegrationTest 为成功状态。
func setSucceeded(status *infrav1alpha1.IntegrationTestStatus) {
	status.Phase = infrav1alpha1.IntegrationTestPhaseSucceeded
	now := metav1.Now()
	status.CompletionTime = &now
}

// finishTest 完成测试，根据当前状态设置最终结果。
// 先 patch 状态，成功后再发送 Event。
// 注意：使用 isTerminalPhase 和 CompletionTime 双重检查确保幂等性。
func (r *IntegrationTestReconciler) finishTest(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	// 幂等性检查：如果已经是终态或 CompletionTime 已设置，直接返回
	// 使用双重检查防止缓存部分更新导致的重复事件
	if isTerminalPhase(it.Status.Phase) || it.Status.CompletionTime != nil {
		return ctrl.Result{}, nil
	}

	// 成功情况（Phase 还不是终态）
	setSucceeded(&it.Status)
	if err := r.patchStatus(ctx, it, it.Status); err != nil {
		return ctrl.Result{}, err
	}
	framework.EmitNormalEvent(r.Recorder, it, EventReasonIntegrationTestSucceeded, "测试用例执行成功")
	return ctrl.Result{}, nil
}
