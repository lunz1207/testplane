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

// Package logging 提供统一的日志工具函数。
//
// 日志级别规范:
//   - Error: 需要人工介入的错误（资源创建失败、无法恢复的状态）
//   - Info (V0): 重要状态变更、操作完成（阶段转换、步骤完成、资源应用）
//   - V(1): 常规操作细节（期望检查结果、资源收敛等待）
//   - V(2): 调试级别信息（函数入口、中间状态、循环迭代）
//
// 消息格式规范:
//   - 使用现在进行时描述操作开始: "applying resource"
//   - 使用过去时描述操作完成: "resource applied"
//   - 使用名词短语描述状态: "step timeout"
//   - 全部小写，无句号
package logging

import (
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// 日志级别常量。
const (
	LevelInfo    = 0 // 重要操作
	LevelVerbose = 1 // 常规细节
	LevelDebug   = 2 // 调试信息
)

// WithResource 添加资源标识到 logger。
// 用于在整个 reconcile 过程中携带资源上下文。
func WithResource(log logr.Logger, obj client.Object) logr.Logger {
	gvk := obj.GetObjectKind().GroupVersionKind()
	kind := gvk.Kind
	if kind == "" {
		// 如果 GVK 为空，尝试从类型名获取
		kind = "Unknown"
	}
	return log.WithValues(
		"kind", kind,
		"namespace", obj.GetNamespace(),
		"name", obj.GetName(),
	)
}

// WithKindName 添加 kind 和 name 到 logger。
// 用于处理非 client.Object 的资源（如 unstructured）。
func WithKindName(log logr.Logger, kind, namespace, name string) logr.Logger {
	return log.WithValues(
		"kind", kind,
		"namespace", namespace,
		"name", name,
	)
}

// WithStep 添加步骤信息到 logger（IntegrationTest 专用）。
func WithStep(log logr.Logger, stepName string, stepIndex int) logr.Logger {
	return log.WithValues("step", stepName, "stepIndex", stepIndex)
}

// WithRound 添加轮次信息到 logger（IntegrationTest 专用）。
func WithRound(log logr.Logger, round int) logr.Logger {
	return log.WithValues("round", round)
}

// Reconciling 记录开始 reconcile。
func Reconciling(log logr.Logger, phase string) {
	log.Info("reconciling", "phase", phase)
}

// PhaseChanged 记录阶段变更。
func PhaseChanged(log logr.Logger, from, to string) {
	log.Info("phase changed", "from", from, "to", to)
}

// ResourceApplying 记录资源应用开始（V1）。
func ResourceApplying(log logr.Logger, kind, name string) {
	log.V(LevelVerbose).Info("applying resource", "targetKind", kind, "targetName", name)
}

// ResourceApplied 记录资源应用完成。
func ResourceApplied(log logr.Logger, kind, name string) {
	log.Info("resource applied", "targetKind", kind, "targetName", name)
}

// ResourceDeleting 记录资源删除开始（V1）。
func ResourceDeleting(log logr.Logger, kind, name string) {
	log.V(LevelVerbose).Info("deleting resource", "targetKind", kind, "targetName", name)
}

// ResourceDeleted 记录资源删除完成。
func ResourceDeleted(log logr.Logger, kind, name string) {
	log.Info("resource deleted", "targetKind", kind, "targetName", name)
}

// WaitingFor 记录等待状态（V1）。
func WaitingFor(log logr.Logger, what string, kvs ...interface{}) {
	log.V(LevelVerbose).Info("waiting for "+what, kvs...)
}

// StepStarted 记录步骤开始。
func StepStarted(log logr.Logger) {
	log.Info("step started")
}

// StepCompleted 记录步骤完成。
func StepCompleted(log logr.Logger) {
	log.Info("step completed")
}

// StepFailed 记录步骤失败。
func StepFailed(log logr.Logger, reason, message string) {
	log.Info("step failed", "reason", reason, "message", message)
}

// ExpectationChecking 记录期望检查开始（V2）。
func ExpectationChecking(log logr.Logger, expectation string) {
	log.V(LevelDebug).Info("checking expectation", "expectation", expectation)
}

// ExpectationPassed 记录期望检查通过（V1）。
func ExpectationPassed(log logr.Logger, expectation string) {
	log.V(LevelVerbose).Info("expectation passed", "expectation", expectation)
}

// ExpectationFailed 记录期望检查失败（V1）。
func ExpectationFailed(log logr.Logger, expectation string, actual interface{}) {
	log.V(LevelVerbose).Info("expectation failed", "expectation", expectation, "actual", actual)
}

// ReadyConditionChecking 记录就绪条件检查（V1）。
func ReadyConditionChecking(log logr.Logger, condition string) {
	log.V(LevelVerbose).Info("checking ready condition", "condition", condition)
}

// ReadyConditionPassed 记录就绪条件通过。
func ReadyConditionPassed(log logr.Logger) {
	log.Info("ready condition passed")
}

// ReadyConditionFailed 记录就绪条件失败。
func ReadyConditionFailed(log logr.Logger, reason string) {
	log.Info("ready condition failed", "reason", reason)
}

// HealthCheckPassed 记录健康检查通过（LoadTest 专用）。
func HealthCheckPassed(log logr.Logger, checkCount int) {
	log.Info("health check passed", "checkCount", checkCount)
}

// HealthCheckFailed 记录健康检查失败（LoadTest 专用）。
func HealthCheckFailed(log logr.Logger, failures, threshold int) {
	log.Info("health check failed", "consecutiveFailures", failures, "threshold", threshold)
}

// RoundStarted 记录轮次开始（IntegrationTest 专用）。
func RoundStarted(log logr.Logger, round int) {
	log.Info("round started", "round", round)
}

// RoundCompleted 记录轮次完成（IntegrationTest 专用）。
func RoundCompleted(log logr.Logger, round int) {
	log.Info("round completed", "round", round)
}

// SpecChanged 记录 spec 变更。
func SpecChanged(log logr.Logger, generation, observedGeneration int64) {
	log.Info("spec changed", "generation", generation, "observedGeneration", observedGeneration)
}

// SpecChangeIgnored 记录 spec 变更被忽略（V1）。
func SpecChangeIgnored(log logr.Logger, generation, observedGeneration int64) {
	log.V(LevelVerbose).Info("spec change ignored (test running)", "generation", generation, "observedGeneration", observedGeneration)
}

// Converged 记录资源收敛完成（V1）。
func Converged(log logr.Logger, kind, name string) {
	log.V(LevelVerbose).Info("resource converged", "targetKind", kind, "targetName", name)
}

// DebugEnter 记录函数入口（V2）。
func DebugEnter(log logr.Logger, funcName string) {
	log.V(LevelDebug).Info("entering", "func", funcName)
}
