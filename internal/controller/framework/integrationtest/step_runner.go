package integrationtest

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
	"github.com/lunz1207/testplane/internal/controller/framework/resource"
)

// executeSequential 顺序执行测试步骤。
func (r *IntegrationTestReconciler) executeSequential(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	steps := tc.Spec.Steps
	currentIdx := nextStepIndex(status.Steps)
	status.CurrentStepIndex = &currentIdx

	// 当前轮所有步骤已完成，开始下一轮
	if currentIdx >= len(steps) {
		return r.startNextRound(ctx, tc, status)
	}

	step := steps[currentIdx]
	log.Info("executing step", "step", step.Name, "index", currentIdx, "round", status.CurrentRound)

	stepStatus := r.ensureStepStatus(status, currentIdx, step)

	// 展开资源模板
	resourceSpecs, err := r.expandStepTemplate(tc, step)
	if err != nil {
		setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("expand manifests failed: %v", err))
		framework.EmitWarningEvent(r.Recorder, tc, EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 扩展资源失败: %s - %s", status.CurrentRound, currentIdx+1, step.Name, err.Error()))
		return r.handleStepFailure(ctx, tc, status)
	}

	// 判断是否首次执行：状态为空表示首次
	isFirstExecution := stepStatus.State == ""

	// 步骤首次执行时发送事件
	if isFirstExecution {
		framework.EmitNormalEvent(r.Recorder, tc, EventReasonStepStarted, fmt.Sprintf("[Round %d] 开始执行步骤 %d: %s", status.CurrentRound, currentIdx+1, step.Name))
	}

	// 1. 应用资源（仅首次执行）
	if isFirstExecution {
		if err := r.applyResources(ctx, tc, resourceSpecs); err != nil {
			setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("apply failed: %v", err))
			framework.EmitWarningEvent(r.Recorder, tc, EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 执行失败: %s - %s", status.CurrentRound, currentIdx+1, step.Name, err.Error()))
			return r.handleStepFailure(ctx, tc, status)
		}
		stepStatus.State = framework.StateRunning
		log.Info("resources applied", "step", step.Name)
	}

	// 2. 等待资源收敛
	if err := r.waitResourcesConverge(ctx, resourceSpecs); err != nil {
		log.Info("waiting for convergence", "step", step.Name)
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	// 3. ReadyCondition（可选）
	if step.ReadyCondition != nil {
		result, err := r.checkStepReadyCondition(ctx, tc, status, stepStatus, step, resourceSpecs)
		if err != nil || result.RequeueAfter > 0 {
			return result, err
		}
	}

	// 4. 执行期望检查
	return r.checkStepExpectations(ctx, tc, status, stepStatus, step, resourceSpecs)
}

// executeParallel 并行执行：所有步骤同时执行，全部完成后验证期望。
func (r *IntegrationTestReconciler) executeParallel(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	steps := tc.Spec.Steps

	if len(steps) == 0 {
		return r.startNextRound(ctx, tc, status)
	}

	// 检查是否所有步骤都已成功
	if r.allStepsSucceeded(status, len(steps)) {
		return r.startNextRound(ctx, tc, status)
	}

	log.Info("parallel execution", "round", status.CurrentRound, "totalSteps", len(steps))

	// 1. 确保所有步骤状态已初始化
	for i, step := range steps {
		r.ensureStepStatus(status, i, step)
	}

	// 1b. 展开所有步骤资源模板
	stepResourceSpecs := make([][]resource.ExpandedManifest, len(steps))
	for i, step := range steps {
		specs, err := r.expandStepTemplate(tc, step)
		if err != nil {
			stepStatus := &status.Steps[i]
			setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("expand manifests failed: %v", err))
			framework.EmitWarningEvent(r.Recorder, tc, EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 扩展资源失败: %s - %s", status.CurrentRound, i+1, step.Name, err.Error()))
			return r.handleStepFailure(ctx, tc, status)
		}
		stepResourceSpecs[i] = specs
	}

	// 2. 并行应用所有步骤的资源
	for i, step := range steps {
		stepStatus := &status.Steps[i]
		// 状态为空表示首次执行
		if stepStatus.State == "" {
			framework.EmitNormalEvent(r.Recorder, tc, EventReasonStepStarted, fmt.Sprintf("[Round %d] 开始执行步骤 %d: %s", status.CurrentRound, i+1, step.Name))

			if err := r.applyResources(ctx, tc, stepResourceSpecs[i]); err != nil {
				setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("apply failed: %v", err))
				framework.EmitWarningEvent(r.Recorder, tc, EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 执行失败: %s - %s", status.CurrentRound, i+1, step.Name, err.Error()))
				return r.handleStepFailure(ctx, tc, status)
			}
			stepStatus.State = framework.StateRunning
			log.Info("resources applied", "step", step.Name)
		}
	}

	// 3. 等待所有资源收敛
	allConverged := true
	for i, step := range steps {
		if err := r.waitResourcesConverge(ctx, stepResourceSpecs[i]); err != nil {
			log.Info("waiting for convergence", "step", step.Name)
			allConverged = false
		}
	}
	if !allConverged {
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	// 4. 并行检查所有步骤的期望
	allPassed := true
	anyFailed := false
	for i, step := range steps {
		stepStatus := &status.Steps[i]
		if stepStatus.State == framework.StateSucceeded {
			continue
		}
		if stepStatus.State == framework.StateFailed {
			anyFailed = true
			continue
		}

		result, stepPassed := r.checkParallelStepExpectations(ctx, tc, status, stepStatus, step, stepResourceSpecs[i])
		if !stepPassed {
			allPassed = false
			if stepStatus.State == framework.StateFailed {
				anyFailed = true
			}
		}
		if result.RequeueAfter > 0 {
			allPassed = false
		}
	}

	if anyFailed {
		return r.handleStepFailure(ctx, tc, status)
	}

	if allPassed && r.allStepsSucceeded(status, len(steps)) {
		log.Info("all parallel steps completed", "round", status.CurrentRound)
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{RequeueAfter: defaultRequeue}, nil
}

// handleStepFailure 处理步骤失败，检查是否应该停止。
// 注意：此函数只修改 status，不负责持久化（由顶层 reconcileNormal() 统一处理）。
func (r *IntegrationTestReconciler) handleStepFailure(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) (ctrl.Result, error) {
	if tc.Spec.Repeat != nil && tc.Spec.Repeat.UntilFailure {
		return r.finishTest(ctx, tc, status)
	}
	framework.EmitWarningEvent(r.Recorder, tc, EventReasonIntegrationTestFailed, fmt.Sprintf("测试用例执行失败: %s", status.Message))
	return ctrl.Result{}, nil
}

// allStepsSucceeded 检查是否所有步骤都已成功完成。
func (r *IntegrationTestReconciler) allStepsSucceeded(status *infrav1alpha1.IntegrationTestStatus, totalSteps int) bool {
	if len(status.Steps) != totalSteps {
		return false
	}
	for _, s := range status.Steps {
		if s.State != framework.StateSucceeded {
			return false
		}
	}
	return true
}
