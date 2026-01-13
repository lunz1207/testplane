package integrationtest

import (
	"context"
	stderrors "errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
	"github.com/lunz1207/testplane/internal/controller/framework/resource"
)

// stepExpectationOutcome 步骤期望检查结果。
type stepExpectationOutcome int

const (
	outcomeWaiting   stepExpectationOutcome = iota // 等待资源就绪或期望满足
	outcomeFailed                                  // 步骤失败（超时或错误）
	outcomeSucceeded                               // 步骤成功
)

// checkStepExpectationsCore 核心期望检查逻辑，被 checkStepExpectations 和 checkParallelStepExpectations 共用。
func (r *IntegrationTestReconciler) checkStepExpectationsCore(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifests []resource.ExpandedManifest) stepExpectationOutcome {
	log := logf.FromContext(ctx)

	selectors := selectorsFromStep(step)
	allExpectations := expectationsFromWaitCondition(step.Expectations)

	state, waiting, err := r.buildStepState(ctx, tc, selectors, allExpectations, manifests)
	if err != nil {
		setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("gather state failed: %v", err))
		return outcomeFailed
	}

	if waiting {
		if r.stepTimedOut(stepStatus) {
			setStepFailed(status, stepStatus, step.Name, framework.ReasonTimeout, "resources/selectors not ready before timeout")
			return outcomeFailed
		}
		stepStatus.State = framework.StateRunning
		return outcomeWaiting
	}

	// 执行期望检查
	results, err := r.runExpectations(step.Expectations, state)
	if err != nil {
		setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("expectations error: %v", err))
		framework.EmitWarningEvent(r.Recorder, tc, EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %s 期望检查错误: %v", status.CurrentRound, step.Name, err))
		return outcomeFailed
	}

	allResults := results.All()
	stepStatus.ExpectationResults = framework.ToExpectationResultSummaries(allResults)

	for _, result := range allResults {
		log.Info("expectation result", "step", step.Name, "expect", result.Expect, "passed", result.Passed)
	}

	if !results.Passed() {
		if r.stepTimedOut(stepStatus) {
			setStepFailed(status, stepStatus, step.Name, framework.ReasonTimeout, "expectations not satisfied before timeout")
			framework.EmitWarningEvent(r.Recorder, tc, EventReasonIntegrationTestTimeout, fmt.Sprintf("[Round %d] 步骤 %s 期望检查超时", status.CurrentRound, step.Name))
			return outcomeFailed
		}
		stepStatus.State = framework.StateRunning
		return outcomeWaiting
	}

	// 步骤成功
	setStepSucceeded(stepStatus)
	framework.EmitNormalEvent(r.Recorder, tc, EventReasonStepSucceeded, fmt.Sprintf("[Round %d] 步骤 %s 执行成功", status.CurrentRound, step.Name))
	log.Info("step completed", "step", step.Name)
	return outcomeSucceeded
}

// checkParallelStepExpectations 检查并行步骤的期望，返回是否通过。
// 注意：此函数只修改 status，不负责持久化（由顶层 reconcileNormal() 统一处理）。
func (r *IntegrationTestReconciler) checkParallelStepExpectations(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifests []resource.ExpandedManifest) (ctrl.Result, bool) {
	_ = logf.FromContext(ctx)

	// ReadyCondition（可选，仅并行步骤需要）
	if step.ReadyCondition != nil {
		result, err := r.checkStepReadyCondition(ctx, tc, status, stepStatus, step, manifests)
		if err != nil || result.RequeueAfter > 0 {
			return result, false
		}
	}

	outcome := r.checkStepExpectationsCore(ctx, tc, status, stepStatus, step, manifests)
	switch outcome {
	case outcomeWaiting:
		return ctrl.Result{RequeueAfter: defaultRequeue}, false
	case outcomeFailed:
		return ctrl.Result{}, false
	default: // outcomeSucceeded
		return ctrl.Result{}, true
	}
}

// checkStepExpectations 检查步骤的期望。
// 注意：此函数只修改 status，不负责持久化（由顶层 reconcileNormal() 统一处理）。
func (r *IntegrationTestReconciler) checkStepExpectations(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifests []resource.ExpandedManifest) (ctrl.Result, error) {
	outcome := r.checkStepExpectationsCore(ctx, tc, status, stepStatus, step, manifests)
	switch outcome {
	case outcomeWaiting:
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	case outcomeFailed:
		return r.handleStepFailure(ctx, tc, status)
	default: // outcomeSucceeded
		return ctrl.Result{Requeue: true}, nil
	}
}

// checkStepReadyCondition 检查步骤级 ReadyCondition。
func (r *IntegrationTestReconciler) checkStepReadyCondition(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus, stepStatus *infrav1alpha1.StepStatus, step infrav1alpha1.TestStep, manifests []resource.ExpandedManifest) (ctrl.Result, error) {
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
			State:     framework.StateRunning,
			StartedAt: &now,
			Deadline:  &dl,
		}
	}

	selectors := selectorsFromStep(step)
	allExpectations := expectationsFromWaitCondition(ready)

	state, waiting, err := r.buildStepState(ctx, tc, selectors, allExpectations, manifests)
	if err != nil {
		stepStatus.ReadyConditionStatus.State = framework.StateFailed
		stepStatus.ReadyConditionStatus.Results = nil
		setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("readyCondition gather state failed: %v", err))
		return r.handleStepFailure(ctx, tc, status)
	}

	if waiting {
		if r.stepTimedOut(stepStatus) {
			stepStatus.ReadyConditionStatus.State = framework.StateFailed
			now := metav1.Now()
			stepStatus.ReadyConditionStatus.FinishedAt = &now
			setStepFailed(status, stepStatus, step.Name, framework.ReasonTimeout, "readyCondition timeout")
			framework.EmitWarningEvent(r.Recorder, tc, EventReasonIntegrationTestTimeout, fmt.Sprintf("[Round %d] 步骤 %s readyCondition 超时", status.CurrentRound, step.Name))
			return r.handleStepFailure(ctx, tc, status)
		}
		stepStatus.ReadyConditionStatus.State = framework.StateRunning
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	results, err := r.runExpectations(ready, state)
	stepStatus.ReadyConditionStatus.Results = results.All()
	if err != nil {
		stepStatus.ReadyConditionStatus.State = framework.StateFailed
		setStepFailed(status, stepStatus, step.Name, framework.ReasonFailed, fmt.Sprintf("readyCondition error: %v", err))
		framework.EmitWarningEvent(r.Recorder, tc, EventReasonStepFailed, fmt.Sprintf("[Round %d] 步骤 %s readyCondition 错误: %v", status.CurrentRound, step.Name, err))
		return r.handleStepFailure(ctx, tc, status)
	}

	if !results.Passed() {
		if r.stepTimedOut(stepStatus) {
			stepStatus.ReadyConditionStatus.State = framework.StateFailed
			now := metav1.Now()
			stepStatus.ReadyConditionStatus.FinishedAt = &now
			setStepFailed(status, stepStatus, step.Name, framework.ReasonTimeout, "readyCondition not satisfied before timeout")
			framework.EmitWarningEvent(r.Recorder, tc, EventReasonIntegrationTestTimeout, fmt.Sprintf("[Round %d] 步骤 %s readyCondition 超时", status.CurrentRound, step.Name))
			return r.handleStepFailure(ctx, tc, status)
		}
		stepStatus.ReadyConditionStatus.State = framework.StateRunning
		return ctrl.Result{RequeueAfter: defaultRequeue}, nil
	}

	now := metav1.Now()
	stepStatus.ReadyConditionStatus.State = framework.StatePassed
	stepStatus.ReadyConditionStatus.FinishedAt = &now
	log.Info("readyCondition passed", "step", step.Name)
	return ctrl.Result{}, nil
}

// buildStepState 收集模板资源与选择器资源的状态。
func (r *IntegrationTestReconciler) buildStepState(ctx context.Context, tc *infrav1alpha1.IntegrationTest, selectors []infrav1alpha1.ResourceSelector, expectations []infrav1alpha1.Expectation, manifests []resource.ExpandedManifest) (map[string]interface{}, bool, error) {
	state := make(map[string]interface{})

	if len(manifests) > 0 {
		resourceState, err := r.gatherResourceStates(ctx, manifests)
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

	selectorResults, err := r.gatherSelectorStates(ctx, tc, selectors, expectations)
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
	if step.Selector == nil {
		return nil
	}
	return []infrav1alpha1.ResourceSelector{*step.Selector}
}

// expectationsFromWaitCondition 从 WaitCondition 中提取所有期望。
func expectationsFromWaitCondition(condition *infrav1alpha1.WaitCondition) []infrav1alpha1.Expectation {
	if condition == nil {
		return nil
	}
	return append(condition.AllOf, condition.AnyOf...)
}
