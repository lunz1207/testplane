package integrationtest

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
)

// lifecycle.go 包含 IntegrationTest 资源的生命周期管理和状态设置函数

func (r *IntegrationTestReconciler) initializeStatus(status *infrav1alpha1.IntegrationTestStatus) {
	if status.Phase == "" {
		status.Phase = infrav1alpha1.IntegrationTestPhasePending
		now := metav1.Now()
		status.StartTime = &now
	}
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
func (r *IntegrationTestReconciler) finishTest(_ context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) (ctrl.Result, error) {
	// 如果已经失败，发送失败事件
	if status.Phase == infrav1alpha1.IntegrationTestPhaseFailed {
		framework.EmitWarningEvent(r.Recorder, tc, EventReasonIntegrationTestFailed, fmt.Sprintf("测试用例执行失败: %s", status.Message))
		return ctrl.Result{}, nil
	}

	// 设置为成功状态
	setSucceeded(status)
	framework.EmitNormalEvent(r.Recorder, tc, EventReasonIntegrationTestSucceeded, "测试用例执行成功")
	return ctrl.Result{}, nil
}
