package integrationtest

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/shared"
	"github.com/lunz1207/testplane/internal/controller/shared/logging"
)

// 注意：发送 Event 前先用 APIReader 检查 API Server 最新状态，避免缓存延迟导致重复事件

// executeTest 执行测试逻辑，根据模式选择顺序或并行执行。
func (r *IntegrationTestReconciler) executeTest(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	if isTerminalPhase(it.Status.Phase) {
		return ctrl.Result{}, nil
	}

	// Pending → Running：初始化并开始测试
	if it.Status.Phase == infrav1alpha1.IntegrationTestPhasePending {
		it.Status.Phase = infrav1alpha1.IntegrationTestPhaseRunning
		r.initRepeatStatus(&it.Status)
		// 先 patch，成功后再发 Event
		if err := r.patchStatus(ctx, it, it.Status); err != nil {
			return ctrl.Result{}, err
		}
		shared.EmitNormalEvent(r.Recorder, it, shared.EventReasonIntegrationTestStarted, fmt.Sprintf("开始执行测试用例，模式: %s, 轮数: %s", it.Spec.Mode, formatTotalRounds(it)))
	}

	// 检查是否达到停止条件
	if r.shouldStopRepeat(it, &it.Status) {
		return r.finishTest(ctx, it)
	}

	// 从 spec 获取 mode
	mode := it.Spec.Mode
	if mode == "" {
		mode = infrav1alpha1.IntegrationTestModeSequential
	}

	if mode == infrav1alpha1.IntegrationTestModeSequential {
		return r.executeSequential(ctx, it)
	}
	return r.executeParallel(ctx, it)
}

// initRepeatStatus 初始化重复执行状态。
func (r *IntegrationTestReconciler) initRepeatStatus(status *infrav1alpha1.IntegrationTestStatus) {
	status.CurrentRound = 1
	status.CompletedRounds = 0
}

// formatTotalRounds 格式化总轮数显示。
func formatTotalRounds(tc *infrav1alpha1.IntegrationTest) string {
	if tc.Spec.Repeat == nil {
		return "1"
	}
	if tc.Spec.Repeat.Count == 0 {
		return "无限"
	}
	return fmt.Sprintf("%d", tc.Spec.Repeat.Count)
}

// shouldStopRepeat 检查是否应该停止重复执行。
func (r *IntegrationTestReconciler) shouldStopRepeat(tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) bool {
	// 没有配置 repeat，且当前轮次完成
	if tc.Spec.Repeat == nil {
		return status.CompletedRounds >= 1
	}

	repeat := tc.Spec.Repeat

	// 检查轮数限制
	if repeat.Count > 0 && status.CompletedRounds >= repeat.Count {
		return true
	}

	// 检查时间限制
	if repeat.MaxDurationSeconds > 0 && status.StartTime != nil {
		elapsed := time.Since(status.StartTime.Time)
		if elapsed >= time.Duration(repeat.MaxDurationSeconds)*time.Second {
			return true
		}
	}

	return false
}

// startNextRound 开始下一轮执行。
func (r *IntegrationTestReconciler) startNextRound(ctx context.Context, it *infrav1alpha1.IntegrationTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 避免重复增加 CompletedRounds（轮间延迟返回后会再次进入此函数）
	if len(it.Status.Steps) > 0 {
		it.Status.CompletedRounds++
		logging.RoundCompleted(log, it.Status.CurrentRound)

		// 重置步骤索引（准备下一轮或结束）
		zero := 0
		it.Status.CurrentStepIndex = &zero
	}

	// 检查是否应该停止
	if r.shouldStopRepeat(it, &it.Status) {
		return r.finishTest(ctx, it)
	}

	// 继续下一轮，递增轮数并重置 Steps 状态
	it.Status.CurrentRound++
	it.Status.Steps = nil

	// patch 状态
	if err := r.patchStatus(ctx, it, it.Status); err != nil {
		return ctrl.Result{}, err
	}

	// 轮间延迟
	if it.Spec.Repeat != nil && it.Spec.Repeat.DelayBetweenRounds > 0 {
		log.V(logging.LevelVerbose).Info("delay between rounds", "seconds", it.Spec.Repeat.DelayBetweenRounds)
		return ctrl.Result{RequeueAfter: time.Duration(it.Spec.Repeat.DelayBetweenRounds) * time.Second}, nil
	}

	logging.RoundStarted(log, it.Status.CurrentRound)
	return ctrl.Result{Requeue: true}, nil
}
