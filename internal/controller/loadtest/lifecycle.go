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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/shared"
	"github.com/lunz1207/testplane/internal/controller/shared/logging"
)

// initializeLoadTest 初始化 LoadTest 状态。
func (r *LoadTestReconciler) initializeLoadTest(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("initializing")

	now := metav1.Now()
	lt.Status.Phase = infrav1alpha1.LoadTestPending
	lt.Status.StartTime = &now
	lt.Status.ObservedGeneration = lt.Generation

	// 设置初始 Conditions
	shared.SetCondition(&lt.Status.Conditions, ConditionTypeReady, metav1.ConditionFalse, "Initializing", "LoadTest is initializing", lt.Generation)
	shared.SetCondition(&lt.Status.Conditions, ConditionTypeTargetReady, metav1.ConditionUnknown, "Pending", "Target readiness not yet checked", lt.Generation)

	if err := shared.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, err
	}

	shared.EmitNormalEvent(r.Recorder, lt, shared.EventReasonLoadTestStarted, "LoadTest started")
	return ctrl.Result{Requeue: true}, nil
}

// reconcilePending 处理 Pending 阶段。
func (r *LoadTestReconciler) reconcilePending(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	logging.PhaseChanged(log, string(infrav1alpha1.LoadTestPending), string(infrav1alpha1.LoadTestInitializing))

	lt.Status.Phase = infrav1alpha1.LoadTestInitializing

	if err := shared.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// reconcileTerminal 处理终态。
// workload 通过 OwnerReference 由 K8s 自动清理。
func (r *LoadTestReconciler) reconcileTerminal(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	// 设置完成时间
	if lt.Status.CompletionTime == nil {
		now := metav1.Now()
		lt.Status.CompletionTime = &now

		// 只在 Succeeded 状态下设置 Ready Condition 为 True
		if lt.Status.Phase == infrav1alpha1.LoadTestSucceeded {
			shared.SetCondition(&lt.Status.Conditions, ConditionTypeReady, metav1.ConditionTrue, "Succeeded", "LoadTest completed successfully", lt.Generation)
		}

		if err := shared.PatchStatusMerge(ctx, r.Client, lt); err != nil {
			return ctrl.Result{}, err
		}

		// 只在 Succeeded 状态下发送成功事件
		if lt.Status.Phase == infrav1alpha1.LoadTestSucceeded {
			shared.EmitNormalEvent(r.Recorder, lt, shared.EventReasonLoadTestSucceeded, "LoadTest completed successfully")
		}
	}

	return ctrl.Result{}, nil
}

// setFailed 设置失败状态。
func (r *LoadTestReconciler) setFailed(ctx context.Context, lt *infrav1alpha1.LoadTest, reason, message string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	logging.StepFailed(log, reason, message)

	lt.Status.Phase = infrav1alpha1.LoadTestFailed
	lt.Status.Reason = reason
	lt.Status.Message = message

	// 设置 Ready Condition 为 False
	shared.SetCondition(&lt.Status.Conditions, ConditionTypeReady, metav1.ConditionFalse, reason, message, lt.Generation)

	if err := shared.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, err
	}

	shared.EmitWarningEvent(r.Recorder, lt, shared.EventReasonLoadTestFailed, message)
	// Failed 是终态，无需 Requeue
	return ctrl.Result{}, nil
}
