package integrationtest

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
)

// executeTest 执行测试逻辑，根据模式选择顺序或并行执行。
func (r *IntegrationTestReconciler) executeTest(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) (ctrl.Result, error) {
	if isTerminalPhase(status.Phase) {
		return ctrl.Result{}, nil
	}

	// Pending → Running：初始化并开始测试
	if status.Phase == infrav1alpha1.IntegrationTestPhasePending {
		status.Phase = infrav1alpha1.IntegrationTestPhaseRunning
		r.initRepeatStatus(status)
		framework.EmitNormalEvent(r.Recorder, tc, EventReasonIntegrationTestStarted, fmt.Sprintf("开始执行测试用例，模式: %s, 轮数: %s", tc.Spec.Mode, formatTotalRounds(tc)))
	}

	// 检查是否达到停止条件
	if r.shouldStopRepeat(tc, status) {
		return r.finishTest(ctx, tc, status)
	}

	var result ctrl.Result
	var err error

	// 从 spec 获取 mode
	mode := tc.Spec.Mode
	if mode == "" {
		mode = infrav1alpha1.IntegrationTestModeSequential
	}

	if mode == infrav1alpha1.IntegrationTestModeSequential {
		result, err = r.executeSequential(ctx, tc, status)
	} else {
		result, err = r.executeParallel(ctx, tc, status)
	}

	// 注意：status 持久化已移至顶层 reconcileNormal() 统一处理
	return result, err
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
func (r *IntegrationTestReconciler) startNextRound(ctx context.Context, tc *infrav1alpha1.IntegrationTest, status *infrav1alpha1.IntegrationTestStatus) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 避免重复增加 CompletedRounds（轮间延迟返回后会再次进入此函数）
	if len(status.Steps) > 0 {
		// 保存当前轮次摘要到历史
		saveRoundSummary(status)

		status.CompletedRounds++
		log.Info("round completed", "round", status.CurrentRound, "completedRounds", status.CompletedRounds)

		// 重置步骤索引（准备下一轮或结束）
		zero := 0
		status.CurrentStepIndex = &zero
	}

	// 检查是否应该停止
	if r.shouldStopRepeat(tc, status) {
		return r.finishTest(ctx, tc, status)
	}

	// 继续下一轮，递增轮数并重置 Steps 状态
	status.CurrentRound++
	status.Steps = nil

	// 轮间延迟
	if tc.Spec.Repeat != nil && tc.Spec.Repeat.DelayBetweenRounds > 0 {
		log.Info("delay between rounds", "seconds", tc.Spec.Repeat.DelayBetweenRounds)
		return ctrl.Result{RequeueAfter: time.Duration(tc.Spec.Repeat.DelayBetweenRounds) * time.Second}, nil
	}

	log.Info("starting next round", "round", status.CurrentRound)
	return ctrl.Result{Requeue: true}, nil
}

// saveRoundSummary 保存当前轮次摘要到历史（保留最近 10 轮）并更新聚合统计。
func saveRoundSummary(status *infrav1alpha1.IntegrationTestStatus) {
	if len(status.Steps) == 0 {
		return
	}

	// 计算摘要信息
	var succeededSteps int
	var failedStep string
	var startedAt, finishedAt *time.Time

	for _, step := range status.Steps {
		if step.State == framework.StateSucceeded {
			succeededSteps++
		} else if step.State == framework.StateFailed && failedStep == "" {
			failedStep = step.Name
		}
		if step.StartedAt != nil && (startedAt == nil || step.StartedAt.Time.Before(*startedAt)) {
			t := step.StartedAt.Time
			startedAt = &t
		}
		if step.FinishedAt != nil && (finishedAt == nil || step.FinishedAt.After(*finishedAt)) {
			t := step.FinishedAt.Time
			finishedAt = &t
		}
	}

	// 计算执行时长
	var durationSeconds int32
	if startedAt != nil && finishedAt != nil {
		durationSeconds = int32(finishedAt.Sub(*startedAt).Seconds())
	}

	summary := infrav1alpha1.RoundSummary{
		Round:           status.CurrentRound,
		Succeeded:       succeededSteps == len(status.Steps),
		FailedStep:      failedStep,
		StepCount:       len(status.Steps),
		SucceededSteps:  succeededSteps,
		DurationSeconds: durationSeconds,
	}
	if startedAt != nil {
		t := metav1.NewTime(*startedAt)
		summary.StartedAt = &t
	}
	if finishedAt != nil {
		t := metav1.NewTime(*finishedAt)
		summary.FinishedAt = &t
	}

	// 更新聚合统计
	updateAggregateStats(status, &summary)

	// 保留最近 10 轮
	const maxRoundHistory = 10
	status.RoundHistory = append(status.RoundHistory, summary)
	if len(status.RoundHistory) > maxRoundHistory {
		status.RoundHistory = status.RoundHistory[len(status.RoundHistory)-maxRoundHistory:]
	}
}

// updateAggregateStats 更新聚合统计信息。
func updateAggregateStats(status *infrav1alpha1.IntegrationTestStatus, summary *infrav1alpha1.RoundSummary) {
	if status.AggregateStats == nil {
		status.AggregateStats = &infrav1alpha1.AggregateStats{}
	}

	stats := status.AggregateStats

	// 更新步骤计数
	stats.TotalSteps += summary.StepCount
	stats.TotalSucceededSteps += summary.SucceededSteps
	stats.TotalFailedSteps += summary.StepCount - summary.SucceededSteps

	// 更新轮次计数
	if summary.Succeeded {
		stats.SucceededRounds++
	} else {
		stats.FailedRounds++
	}

	// 更新时长统计
	if summary.DurationSeconds > 0 {
		// 累计总时长（用于外部计算平均值）
		stats.TotalDurationSeconds += int64(summary.DurationSeconds)

		// 更新最小/最大时长
		if stats.MinDurationSeconds == 0 || summary.DurationSeconds < stats.MinDurationSeconds {
			stats.MinDurationSeconds = summary.DurationSeconds
		}
		if summary.DurationSeconds > stats.MaxDurationSeconds {
			stats.MaxDurationSeconds = summary.DurationSeconds
		}
	}

	// 更新时间戳
	now := metav1.Now()
	stats.LastUpdated = &now
}
