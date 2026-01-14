package integrationtest

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/shared"
	"github.com/lunz1207/testplane/internal/controller/shared/logging"
	"github.com/lunz1207/testplane/internal/controller/shared/resource"
)

// 注意：发送 Event 前先用 APIReader 检查 API Server 最新状态，避免缓存延迟导致重复事件

// executeSequential 顺序执行测试步骤。
func (r *IntegrationTestReconciler) executeSequential(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	baseLog := logf.FromContext(ctx)
	steps := it.Spec.Steps
	currentIdx := nextStepIndex(it.Status.Steps)
	it.Status.CurrentStepIndex = &currentIdx

	// 当前轮所有步骤已完成，开始下一轮
	if currentIdx >= len(steps) {
		return r.startNextRound(ctx, it)
	}

	step := steps[currentIdx]
	log := logging.WithStep(logging.WithRound(baseLog, it.Status.CurrentRound), step.Name, currentIdx)
	logging.StepStarted(log)

	stepStatus := r.ensureStepStatus(&it.Status, currentIdx, step)

	// 展开资源模板
	manifest, err := r.expandStepResource(it, step)
	if err != nil {
		setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("expand manifest failed: %v", err))
		// 先 patch，成功后再发 Event
		if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
			return ctrl.Result{}, patchErr
		}
		shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 扩展资源失败: %s - %s", it.Status.CurrentRound, currentIdx+1, step.Name, err.Error()))
		return r.handleStepFailure(ctx, it)
	}

	// 判断是否首次执行：状态为空表示首次
	isFirstExecution := stepStatus.State == ""

	// 1. 应用资源（仅首次执行）
	if isFirstExecution {
		if err := r.applyResource(ctx, it, manifest); err != nil {
			setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("apply failed: %v", err))
			// 先 patch，成功后再发 Event
			if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
				return ctrl.Result{}, patchErr
			}
			shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 执行失败: %s - %s", it.Status.CurrentRound, currentIdx+1, step.Name, err.Error()))
			return r.handleStepFailure(ctx, it)
		}
		stepStatus.State = shared.StateRunning
		// 先 patch，成功后再发 Event
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, err
		}
		shared.EmitNormalEvent(r.Recorder, it, shared.EventReasonStepStarted, fmt.Sprintf("[Round %d] 开始执行步骤 %d: %s", it.Status.CurrentRound, currentIdx+1, step.Name))
		logging.ResourceApplied(log, manifest.Object.GetKind(), manifest.Object.GetName())
	}

	// 2. 等待资源收敛
	if err := r.waitResourceConverge(ctx, manifest); err != nil {
		logging.WaitingFor(log, "convergence", "targetKind", manifest.Object.GetKind(), "targetName", manifest.Object.GetName())
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	// 3. ReadyCondition（可选）
	if step.ReadyCondition != nil {
		result, err := r.checkStepReadyCondition(ctx, it, stepStatus, step, manifest)
		if err != nil || result.RequeueAfter > 0 {
			return result, err
		}
	}

	// 4. 执行期望检查
	return r.checkStepExpectations(ctx, it, stepStatus, step, manifest)
}

// executeParallel 并行执行：所有步骤同时执行，全部完成后验证期望。
func (r *IntegrationTestReconciler) executeParallel(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	baseLog := logf.FromContext(ctx)
	steps := it.Spec.Steps

	if len(steps) == 0 {
		return r.startNextRound(ctx, it)
	}

	// 检查是否所有步骤都已成功
	if r.allStepsSucceeded(&it.Status, len(steps)) {
		return r.startNextRound(ctx, it)
	}

	log := logging.WithRound(baseLog, it.Status.CurrentRound)
	log.Info("parallel execution started", "totalSteps", len(steps))

	// 1. 确保所有步骤状态已初始化
	for i, step := range steps {
		r.ensureStepStatus(&it.Status, i, step)
	}

	// 1b. 展开所有步骤资源模板
	stepManifests := make([]*resource.ExpandedManifest, len(steps))
	for i, step := range steps {
		manifest, err := r.expandStepResource(it, step)
		if err != nil {
			stepStatus := &it.Status.Steps[i]
			setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("expand manifest failed: %v", err))
			// 先 patch，成功后再发 Event
			if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
				return ctrl.Result{}, patchErr
			}
			shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 扩展资源失败: %s - %s", it.Status.CurrentRound, i+1, step.Name, err.Error()))
			return r.handleStepFailure(ctx, it)
		}
		stepManifests[i] = manifest
	}

	// 2. 并行应用所有步骤的资源
	for i, step := range steps {
		stepStatus := &it.Status.Steps[i]
		// 状态为空表示首次执行
		if stepStatus.State == "" {
			if err := r.applyResource(ctx, it, stepManifests[i]); err != nil {
				setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("apply failed: %v", err))
				// 先 patch，成功后再发 Event
				if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
					return ctrl.Result{}, patchErr
				}
				shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %d 执行失败: %s - %s", it.Status.CurrentRound, i+1, step.Name, err.Error()))
				return r.handleStepFailure(ctx, it)
			}
			stepStatus.State = shared.StateRunning
			// 先 patch，成功后再发 Event
			if err := r.patchStatus(ctx, it, it.Status); err != nil {
				return ctrl.Result{}, err
			}
			shared.EmitNormalEvent(r.Recorder, it, shared.EventReasonStepStarted, fmt.Sprintf("[Round %d] 开始执行步骤 %d: %s", it.Status.CurrentRound, i+1, step.Name))
			stepLog := logging.WithStep(log, step.Name, i)
			logging.ResourceApplied(stepLog, stepManifests[i].Object.GetKind(), stepManifests[i].Object.GetName())
		}
	}

	// 3. 等待所有资源收敛
	allConverged := true
	for i, step := range steps {
		if err := r.waitResourceConverge(ctx, stepManifests[i]); err != nil {
			stepLog := logging.WithStep(log, step.Name, i)
			logging.WaitingFor(stepLog, "convergence", "targetKind", stepManifests[i].Object.GetKind(), "targetName", stepManifests[i].Object.GetName())
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
		stepStatus := &it.Status.Steps[i]
		if stepStatus.State == shared.StateSucceeded {
			continue
		}
		if stepStatus.State == shared.StateFailed {
			anyFailed = true
			continue
		}

		result, stepPassed := r.checkParallelStepExpectations(ctx, it, stepStatus, step, stepManifests[i])
		if !stepPassed {
			allPassed = false
			if stepStatus.State == shared.StateFailed {
				anyFailed = true
			}
		}
		if result.RequeueAfter > 0 {
			allPassed = false
		}
	}

	if anyFailed {
		return r.handleStepFailure(ctx, it)
	}

	if allPassed && r.allStepsSucceeded(&it.Status, len(steps)) {
		log.Info("all parallel steps completed")
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{RequeueAfter: defaultRequeue}, nil
}

// handleStepFailure 处理步骤失败，检查是否应该停止。
// 先 patch 状态，成功后再发送 Event。
func (r *IntegrationTestReconciler) handleStepFailure(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	// 检查 API Server 最新状态，避免重复事件
	if r.testAlreadyCompleted(ctx, it) {
		return ctrl.Result{}, nil
	}

	if it.Spec.Repeat != nil && it.Spec.Repeat.UntilFailure {
		// UntilFailure 模式：设置 CompletionTime 完成测试
		now := metav1.Now()
		it.Status.CompletionTime = &now
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, err
		}
	}
	// 发送失败事件（状态已在调用方或上面 patch）
	shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonIntegrationTestFailed, fmt.Sprintf("测试用例执行失败: %s", it.Status.Message))
	return ctrl.Result{}, nil
}

// allStepsSucceeded 检查是否所有步骤都已成功完成。
func (r *IntegrationTestReconciler) allStepsSucceeded(status *infrav1alpha1.IntegrationTestStatus, totalSteps int) bool {
	if len(status.Steps) != totalSteps {
		return false
	}
	for _, s := range status.Steps {
		if s.State != shared.StateSucceeded {
			return false
		}
	}
	return true
}
