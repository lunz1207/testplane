package integrationtest

import (
	"context"
	stderrors "errors"
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

// stepExpectationOutcome 步骤期望检查结果。
type stepExpectationOutcome int

const (
	outcomeWaiting   stepExpectationOutcome = iota // 等待资源就绪或期望满足
	outcomeFailed                                  // 步骤失败（超时或错误）
	outcomeSucceeded                               // 步骤成功
)

// checkStepExpectationsCore 核心期望检查逻辑，被 checkStepExpectations 和 checkParallelStepExpectations 共用。
// 返回 outcome 和是否需要发送 Event（调用方负责 patch 和发送 Event）。
func (r *IntegrationTestReconciler) checkStepExpectationsCore(ctx context.Context, it *infrav1alpha1.IntegrationTest, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifest *resource.ExpandedManifest) (stepExpectationOutcome, string) {
	log := logf.FromContext(ctx)

	selectors := selectorsFromStep(step)
	allExpectations := expectationsFromStepCondition(step.Expectations)

	state, waiting, err := r.buildStepState(ctx, it, selectors, allExpectations, manifest)
	if err != nil {
		setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("gather state failed: %v", err))
		return outcomeFailed, ""
	}

	if waiting {
		if r.stepTimedOut(stepStatus) {
			setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonTimeout, "resources/selectors not ready before timeout")
			return outcomeFailed, ""
		}
		stepStatus.State = shared.StateRunning
		return outcomeWaiting, ""
	}

	// 执行期望检查
	results, err := r.runExpectations(step.Expectations, state)
	if err != nil {
		setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("expectations error: %v", err))
		return outcomeFailed, fmt.Sprintf("[Round %d] 步骤 %s 期望检查错误: %v", it.Status.CurrentRound, step.Name, err)
	}

	allResults := results.All()
	stepStatus.ExpectationResults = shared.ToExpectationResultSummaries(allResults)

	for _, result := range allResults {
		if result.Passed {
			logging.ExpectationPassed(log, result.Expect)
		} else {
			logging.ExpectationFailed(log, result.Expect, result.Actual)
		}
	}

	if !results.Passed() {
		if r.stepTimedOut(stepStatus) {
			setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonTimeout, "expectations not satisfied before timeout")
			return outcomeFailed, fmt.Sprintf("[Round %d] 步骤 %s 期望检查超时", it.Status.CurrentRound, step.Name)
		}
		stepStatus.State = shared.StateRunning
		return outcomeWaiting, ""
	}

	// 步骤成功
	setStepSucceeded(stepStatus)
	logging.StepCompleted(log)
	return outcomeSucceeded, fmt.Sprintf("[Round %d] 步骤 %s 执行成功", it.Status.CurrentRound, step.Name)
}

// checkParallelStepExpectations 检查并行步骤的期望，返回是否通过。
func (r *IntegrationTestReconciler) checkParallelStepExpectations(ctx context.Context, it *infrav1alpha1.IntegrationTest, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifest *resource.ExpandedManifest) (ctrl.Result, bool) {

	// ReadyCondition（可选，仅并行步骤需要）
	if step.ReadyCondition != nil {
		result, err := r.checkStepReadyCondition(ctx, it, stepStatus, step, manifest)
		if err != nil || result.RequeueAfter > 0 {
			return result, false
		}
	}

	outcome, eventMsg := r.checkStepExpectationsCore(ctx, it, stepStatus, step, manifest)
	switch outcome {
	case outcomeWaiting:
		return ctrl.Result{RequeueAfter: defaultRequeue}, false
	case outcomeFailed:
		if r.stepAlreadyFinished(ctx, it, stepStatus.Index) {
			return ctrl.Result{}, false
		}
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, false
		}
		if eventMsg != "" {
			shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, eventMsg)
		}
		return ctrl.Result{}, false
	default: // outcomeSucceeded
		if r.stepAlreadyFinished(ctx, it, stepStatus.Index) {
			return ctrl.Result{}, true
		}
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, false
		}
		if eventMsg != "" {
			shared.EmitNormalEvent(r.Recorder, it, shared.EventReasonStepSucceeded, eventMsg)
		}
		return ctrl.Result{}, true
	}
}

// checkStepExpectations 检查步骤的期望。
func (r *IntegrationTestReconciler) checkStepExpectations(ctx context.Context, it *infrav1alpha1.IntegrationTest, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifest *resource.ExpandedManifest) (ctrl.Result, error) {
	outcome, eventMsg := r.checkStepExpectationsCore(ctx, it, stepStatus, step, manifest)
	switch outcome {
	case outcomeWaiting:
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	case outcomeFailed:
		// patch 前检查 API Server 最新状态，避免重复事件
		if r.stepAlreadyFinished(ctx, it, stepStatus.Index) {
			return r.handleStepFailure(ctx, it)
		}
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, err
		}
		if eventMsg != "" {
			shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, eventMsg)
		}
		return r.handleStepFailure(ctx, it)
	default: // outcomeSucceeded
		// patch 前检查 API Server 最新状态，避免重复事件
		if r.stepAlreadyFinished(ctx, it, stepStatus.Index) {
			return ctrl.Result{Requeue: true}, nil
		}
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, err
		}
		if eventMsg != "" {
			shared.EmitNormalEvent(r.Recorder, it, shared.EventReasonStepSucceeded, eventMsg)
		}
		return ctrl.Result{Requeue: true}, nil
	}
}

// checkStepReadyCondition 检查步骤级 ReadyCondition。
func (r *IntegrationTestReconciler) checkStepReadyCondition(ctx context.Context, it *infrav1alpha1.IntegrationTest, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifest *resource.ExpandedManifest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	ready := step.ReadyCondition
	if ready == nil {
		return ctrl.Result{}, nil
	}

	// 初始化 ReadyConditionStatus
	if stepStatus.ReadyConditionStatus == nil {
		now := metav1.Now()
		dl := metav1.NewTime(now.Add(stepTimeout(step)))
		stepStatus.ReadyConditionStatus = &infrav1alpha1.ReadyConditionStatus{
			State:     shared.StateRunning,
			StartedAt: &now,
			Deadline:  &dl,
		}
	}

	selectors := selectorsFromStep(step)
	allExpectations := expectationsFromStepCondition(ready)

	state, waiting, err := r.buildStepState(ctx, it, selectors, allExpectations, manifest)
	if err != nil {
		stepStatus.ReadyConditionStatus.State = shared.StateFailed
		stepStatus.ReadyConditionStatus.Results = nil
		setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("readyCondition gather state failed: %v", err))
		// 先 patch，成功后再发 Event
		if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
			return ctrl.Result{}, patchErr
		}
		shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %s readyCondition 错误: %v", it.Status.CurrentRound, step.Name, err))
		return r.handleStepFailure(ctx, it)
	}

	if waiting {
		if r.stepTimedOut(stepStatus) {
			stepStatus.ReadyConditionStatus.State = shared.StateFailed
			now := metav1.Now()
			stepStatus.ReadyConditionStatus.FinishedAt = &now
			setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonTimeout, "readyCondition timeout")
			// 先 patch，成功后再发 Event
			if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
				return ctrl.Result{}, patchErr
			}
			shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonIntegrationTestTimeout, fmt.Sprintf("[Round %d] 步骤 %s readyCondition 超时", it.Status.CurrentRound, step.Name))
			return r.handleStepFailure(ctx, it)
		}
		stepStatus.ReadyConditionStatus.State = shared.StateRunning
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	results, err := r.runExpectations(ready, state)
	stepStatus.ReadyConditionStatus.Results = results.All()
	if err != nil {
		stepStatus.ReadyConditionStatus.State = shared.StateFailed
		setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonFailed, fmt.Sprintf("readyCondition error: %v", err))
		// 先 patch，成功后再发 Event
		if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
			return ctrl.Result{}, patchErr
		}
		shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %s readyCondition 错误: %v", it.Status.CurrentRound, step.Name, err))
		return r.handleStepFailure(ctx, it)
	}

	if !results.Passed() {
		if r.stepTimedOut(stepStatus) {
			stepStatus.ReadyConditionStatus.State = shared.StateFailed
			now := metav1.Now()
			stepStatus.ReadyConditionStatus.FinishedAt = &now
			setStepFailed(&it.Status, stepStatus, step.Name, shared.ReasonTimeout, "readyCondition not satisfied before timeout")
			// 先 patch，成功后再发 Event
			if patchErr := r.patchStatus(ctx, it, it.Status); patchErr != nil {
				return ctrl.Result{}, patchErr
			}
			shared.EmitWarningEvent(r.Recorder, it, shared.EventReasonIntegrationTestTimeout, fmt.Sprintf("[Round %d] 步骤 %s readyCondition 超时", it.Status.CurrentRound, step.Name))
			return r.handleStepFailure(ctx, it)
		}
		stepStatus.ReadyConditionStatus.State = shared.StateRunning
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	now := metav1.Now()
	stepStatus.ReadyConditionStatus.State = shared.StatePassed
	stepStatus.ReadyConditionStatus.FinishedAt = &now
	logging.ReadyConditionPassed(log)
	return ctrl.Result{}, nil
}

// buildStepState 收集模板资源与选择器资源的状态。
func (r *IntegrationTestReconciler) buildStepState(ctx context.Context, it *infrav1alpha1.IntegrationTest, selectors []infrav1alpha1.ResourceSelector, expectations []infrav1alpha1.Expectation, manifest *resource.ExpandedManifest) (map[string]interface{}, bool, error) {
	state := make(map[string]interface{})

	if manifest != nil {
		resourceState, err := r.gatherResourceState(ctx, manifest)
		if err != nil {
			if stderrors.Is(err, ErrResourceNotReady) {
				return nil, true, nil
			}
			return nil, false, err
		}
		for k, v := range resourceState {
			state[k] = v
		}
	}

	if len(selectors) == 0 {
		return state, false, nil
	}

	selectorResults, err := r.gatherSelectorStates(ctx, it, selectors, expectations)
	if err != nil {
		return nil, false, err
	}
	selectorState := selectorResultsToState(selectorResults)
	if len(selectorState) == 0 {
		return nil, true, nil
	}
	for k, v := range selectorState {
		state[k] = v
	}

	return state, false, nil
}

// selectorsFromStep 从步骤中提取选择器。
func selectorsFromStep(step infrav1alpha1.TestStep) []infrav1alpha1.ResourceSelector {
	if step.Resource == nil || step.Resource.Selector == nil {
		return nil
	}
	return []infrav1alpha1.ResourceSelector{*step.Resource.Selector}
}

// expectationsFromStepCondition 从 StepCondition 中提取所有期望。
func expectationsFromStepCondition(condition *infrav1alpha1.StepCondition) []infrav1alpha1.Expectation {
	if condition == nil {
		return nil
	}
	return append(condition.AllOf, condition.AnyOf...)
}
