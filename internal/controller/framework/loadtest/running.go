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
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
)

// transitionToRunning 进入 Running 阶段并应用 workload。
// emitTargetReadyEvent 参数表示是否在 patch 后发送 TargetReady 事件（readyCondition 通过时使用）。
func (r *LoadTestReconciler) transitionToRunning(ctx context.Context, lt *infrav1alpha1.LoadTest, emitTargetReadyEvent ...bool) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 解析环境变量注入
	if err := r.resolveAndUpdateEnvInjection(ctx, lt, "resolved values"); err != nil {
		return ctrl.Result{}, err
	}

	// 应用 workload
	if err := r.applyWorkload(ctx, lt); err != nil {
		log.Error(err, "failed to apply workload")
		return r.setFailed(ctx, lt, "WorkloadApplyFailed", err.Error())
	}

	// 初始化断言检查状态
	if lt.Spec.Expectations != nil {
		lt.Status.ExpectationsStatus = &infrav1alpha1.ExpectationsStatus{}
	}

	lt.Status.Phase = infrav1alpha1.LoadTestRunning

	// 设置 Conditions
	setCondition(&lt.Status, ConditionTypeTargetReady, metav1.ConditionTrue, "TargetReady", "Target is ready", lt.Generation)
	setCondition(&lt.Status, ConditionTypeReady, metav1.ConditionTrue, "Running", "LoadTest is running", lt.Generation)

	if err := framework.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, err
	}

	// patch 成功后发送 Event
	if len(emitTargetReadyEvent) > 0 && emitTargetReadyEvent[0] {
		framework.EmitNormalEvent(r.Recorder, lt, EventReasonTargetReady, "Target is ready")
	}
	framework.EmitNormalEvent(r.Recorder, lt, EventReasonLoadTestRunning, "LoadTest is now running")

	return ctrl.Result{RequeueAfter: defaultRequeue}, nil
}

// resolveAndUpdateEnvInjection 解析并更新环境变量注入。
func (r *LoadTestReconciler) resolveAndUpdateEnvInjection(ctx context.Context, lt *infrav1alpha1.LoadTest, logMsg string) error {
	log := logf.FromContext(ctx)

	if len(lt.Spec.Workload.EnvInjection) == 0 {
		lt.Status.InjectedValues = nil
		return nil
	}

	target, err := r.getTargetResource(ctx, lt)
	if err != nil {
		log.Error(err, "failed to get target for env injection")
		_, _ = r.setFailed(ctx, lt, "TargetGetFailed", err.Error())
		return err
	}

	values, err := r.resolveEnvInjection(target, lt.Spec.Workload.EnvInjection)
	if err != nil {
		log.Error(err, "failed to resolve env injection")
		_, _ = r.setFailed(ctx, lt, "EnvInjectionFailed", err.Error())
		return err
	}

	lt.Status.InjectedValues = values
	log.Info(logMsg, "values", values)
	return nil
}

// reconcileRunning 处理 Running 阶段。
// 根据 expectations 的 failureThreshold 判断是否失败。
func (r *LoadTestReconciler) reconcileRunning(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	// 执行断言检查
	if lt.Spec.Expectations != nil {
		return r.runExpectationsChecks(ctx, lt)
	}

	// 无断言检查，继续等待
	return ctrl.Result{RequeueAfter: defaultRequeue}, nil
}

// runExpectationsChecks 执行断言检查。
func (r *LoadTestReconciler) runExpectationsChecks(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	interval, status := r.getCheckIntervalAndStatus(lt)

	// 检查是否需要等待
	if remaining := r.shouldWaitForNextCheck(status, interval); remaining > 0 {
		return ctrl.Result{RequeueAfter: remaining}, nil
	}

	return r.executeAndRecordExpectations(ctx, lt, status, interval)
}

// getCheckIntervalAndStatus 获取检查间隔并确保状态存在。
func (r *LoadTestReconciler) getCheckIntervalAndStatus(lt *infrav1alpha1.LoadTest) (time.Duration, *infrav1alpha1.ExpectationsStatus) {
	interval := getDurationOrDefault(lt.Spec.Expectations.IntervalSeconds, 10*time.Second)

	if lt.Status.ExpectationsStatus == nil {
		lt.Status.ExpectationsStatus = &infrav1alpha1.ExpectationsStatus{}
	}

	return interval, lt.Status.ExpectationsStatus
}

// shouldWaitForNextCheck 检查是否需要等待下一次检查。
func (r *LoadTestReconciler) shouldWaitForNextCheck(status *infrav1alpha1.ExpectationsStatus, interval time.Duration) time.Duration {
	if status.LastCheckTime != nil && time.Since(status.LastCheckTime.Time) < interval {
		return interval - time.Since(status.LastCheckTime.Time)
	}
	return 0
}

// executeAndRecordExpectations 执行期望检查并记录结果。
// 采用分散 patch 模式：先 patch 状态，成功后再发送 Event。
func (r *LoadTestReconciler) executeAndRecordExpectations(
	ctx context.Context,
	lt *infrav1alpha1.LoadTest,
	status *infrav1alpha1.ExpectationsStatus,
	interval time.Duration,
) (ctrl.Result, error) {
	// 构建 state map，使用 target 资源
	state := r.buildStateForExpectations(ctx, lt)

	// 执行检查
	results, allPassed := r.runExpectationsWithState(state, *lt.Spec.Expectations)

	// 更新基础状态
	now := metav1.Now()
	status.LastCheckTime = &now
	status.CheckCount++
	status.LastResults = framework.ToExpectationResultSummaries(results)

	// 处理检查结果（只更新状态，不发送 Event）
	var eventMsg string
	var eventType string
	if allPassed {
		eventMsg = r.handleExpectationPass(lt, status)
		eventType = "pass"
	} else {
		var shouldFail bool
		eventMsg, shouldFail = r.handleExpectationFail(ctx, lt, status)
		eventType = "fail"
		if shouldFail {
			return ctrl.Result{}, nil
		}
	}

	// 先 patch 状态
	if err := framework.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, err
	}

	// patch 成功后再发送 Event
	if eventType == "pass" {
		framework.EmitNormalEvent(r.Recorder, lt, framework.EventReasonExpectationPassed, eventMsg)
	} else {
		framework.EmitWarningEvent(r.Recorder, lt, framework.EventReasonExpectationFailed, eventMsg)
	}

	return ctrl.Result{RequeueAfter: interval}, nil
}

// handleExpectationPass 处理期望检查通过的情况。
// 只更新状态，返回 Event 消息（调用方负责 patch 后发送 Event）。
func (r *LoadTestReconciler) handleExpectationPass(lt *infrav1alpha1.LoadTest, status *infrav1alpha1.ExpectationsStatus) string {
	log := logf.FromContext(context.Background())

	status.PassCount++
	status.ConsecutiveFailures = 0
	log.Info("expectations check passed", "checkCount", status.CheckCount)

	msg := fmt.Sprintf("Expectations check passed (pass: %d, fail: %d)", status.PassCount, status.FailCount)

	// 设置 ExpectationsMet Condition
	setCondition(&lt.Status, ConditionTypeExpectationsMet, metav1.ConditionTrue, "ExpectationsPassed", msg, lt.Generation)

	return msg
}

// handleExpectationFail 处理期望检查失败的情况。
// 只更新状态，返回 Event 消息和是否应该终止测试（调用方负责 patch 后发送 Event）。
func (r *LoadTestReconciler) handleExpectationFail(ctx context.Context, lt *infrav1alpha1.LoadTest, status *infrav1alpha1.ExpectationsStatus) (string, bool) {
	log := logf.FromContext(ctx)

	status.FailCount++
	status.ConsecutiveFailures++
	log.Info("expectations check failed", "consecutiveFailures", status.ConsecutiveFailures)

	msg := fmt.Sprintf("Expectations check failed (consecutive failures: %d)", status.ConsecutiveFailures)

	// 设置 ExpectationsMet Condition
	setCondition(&lt.Status, ConditionTypeExpectationsMet, metav1.ConditionFalse, "ExpectationsFailed", msg, lt.Generation)

	// 检查是否达到失败阈值
	threshold := getOrDefaultInt32(lt.Spec.Expectations.FailureThreshold, 3)

	if status.ConsecutiveFailures >= threshold {
		_, _ = r.setFailed(ctx, lt, "ExpectationsFailed",
			fmt.Sprintf("consecutive failures reached threshold: %d", threshold))
		return msg, true
	}

	return msg, false
}

// buildStateForExpectations 为 expectations 构建 state map。
// LoadTest 的断言对象固定为 Target 资源。
func (r *LoadTestReconciler) buildStateForExpectations(
	ctx context.Context,
	lt *infrav1alpha1.LoadTest,
) map[string]interface{} {
	target, err := r.getTargetResource(ctx, lt)
	if err != nil || target == nil {
		return map[string]interface{}{}
	}
	return buildStateFromTarget(target)
}

// runExpectationsWithState 使用预构建的 state 执行断言检查。
func (r *LoadTestReconciler) runExpectationsWithState(state map[string]interface{}, expectations infrav1alpha1.WaitCondition) ([]infrav1alpha1.ExpectationResult, bool) {
	runner := framework.NewExpectationRunner(r.PluginRegistry)
	results, err := runner.RunWaitCondition(&expectations, state)

	// LoadTest 不中断执行，即使出错也继续
	if err != nil {
		return results.All(), false
	}

	return results.All(), results.Passed()
}

// runWaitCondition 执行等待条件检查（用于 readyCondition）。
func (r *LoadTestReconciler) runWaitCondition(target *unstructured.Unstructured, condition infrav1alpha1.WaitCondition) ([]infrav1alpha1.ExpectationResult, bool) {
	// 构建 state map，key 格式: apiVersion/kind/name
	// 这样 SelectStateByResource 可以正确匹配 expectation.resource
	state := buildStateFromTarget(target)

	runner := framework.NewExpectationRunner(r.PluginRegistry)
	results, err := runner.RunWaitCondition(&condition, state)

	if err != nil {
		return results.All(), false
	}

	return results.All(), results.Passed()
}

// buildStateFromTarget 将 target 资源转换为 state map。
// key 格式: apiVersion/kind/name，与 SelectStateByResource 期望的格式一致。
func buildStateFromTarget(target *unstructured.Unstructured) map[string]interface{} {
	keyStr := fmt.Sprintf("%s/%s/%s", target.GetAPIVersion(), target.GetKind(), target.GetName())
	return map[string]interface{}{
		keyStr: target.Object,
	}
}

// summarizeResults 汇总期望结果。
func summarizeResults(results []infrav1alpha1.ExpectationResult) string {
	passed := 0
	failed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	data, _ := json.Marshal(map[string]int{
		"passed": passed,
		"failed": failed,
	})
	return string(data)
}
