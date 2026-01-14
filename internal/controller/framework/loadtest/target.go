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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework"
	"github.com/lunz1207/testplane/internal/controller/framework/resource"
)

const (
	// defaultReadyConditionTimeout 默认就绪条件超时时间。
	defaultReadyConditionTimeout = 5 * time.Minute
)

// reconcileInitializing 处理 Initializing 阶段（应用 Target + 解析注入 + 等待就绪条件）。
func (r *LoadTestReconciler) reconcileInitializing(ctx context.Context, lt *infrav1alpha1.LoadTest) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. 应用并解析 Target
	target, err := r.applyAndResolveTarget(ctx, lt)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 2. 检查 ReadyCondition
	readyCondition := lt.Spec.Target.ReadyCondition
	if readyCondition == nil || (len(readyCondition.AllOf) == 0 && len(readyCondition.AnyOf) == 0) {
		log.Info("no readyCondition defined, transitioning to Running")
		return r.transitionToRunning(ctx, lt)
	}

	// 3. 初始化或检查 ReadyCondition
	if lt.Status.ReadyConditionStatus == nil {
		return r.initializeReadyConditionStatus(ctx, lt, readyCondition)
	}

	return r.checkReadyCondition(ctx, lt, target, readyCondition)
}

// initializeReadyConditionStatus 初始化就绪条件状态。
func (r *LoadTestReconciler) initializeReadyConditionStatus(
	ctx context.Context,
	lt *infrav1alpha1.LoadTest,
	readyCondition *infrav1alpha1.ReadyCondition,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	now := metav1.Now()
	timeout := getDurationOrDefault(readyCondition.TimeoutSeconds, defaultReadyConditionTimeout)
	deadline := metav1.NewTime(now.Add(timeout))

	lt.Status.ReadyConditionStatus = &infrav1alpha1.ReadyConditionStatus{
		State:     "Pending",
		StartedAt: &now,
		Deadline:  &deadline,
	}

	if err := framework.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, fmt.Errorf("update readyConditionStatus: %w", err)
	}

	log.Info("initialized readyCondition check", "timeout", timeout, "deadline", deadline.Time)
	framework.EmitNormalEvent(r.Recorder, lt, EventReasonReadyConditionWait,
		fmt.Sprintf("Waiting for target to be ready (timeout: %v)", timeout))

	return ctrl.Result{Requeue: true}, nil
}

// checkReadyCondition 检查就绪条件。
func (r *LoadTestReconciler) checkReadyCondition(
	ctx context.Context,
	lt *infrav1alpha1.LoadTest,
	target *unstructured.Unstructured,
	readyCondition *infrav1alpha1.ReadyCondition,
) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 检查超时
	if lt.Status.ReadyConditionStatus.Deadline != nil &&
		time.Now().After(lt.Status.ReadyConditionStatus.Deadline.Time) {
		// 设置 TargetReady Condition 为 False
		setCondition(&lt.Status, ConditionTypeTargetReady, metav1.ConditionFalse, "ReadyConditionTimeout", "readyCondition timeout exceeded", lt.Generation)
		return r.setFailed(ctx, lt, "ReadyConditionTimeout", "readyCondition timeout exceeded")
	}

	// 执行 ReadyCondition 检查
	results, allPassed := r.runReadyCondition(target, *readyCondition)
	lt.Status.ReadyConditionStatus.Results = results

	if allPassed {
		log.Info("readyCondition passed, transitioning to Running")
		now := metav1.Now()
		lt.Status.ReadyConditionStatus.State = framework.StatePassed
		lt.Status.ReadyConditionStatus.FinishedAt = &now

		// Event 移至 transitionToRunning 中 patch 之后发送，避免重复
		return r.transitionToRunning(ctx, lt, true)
	}

	// 设置 TargetReady Condition 为等待中
	setCondition(&lt.Status, ConditionTypeTargetReady, metav1.ConditionFalse, "WaitingForReadyCondition", "Waiting for target to become ready", lt.Generation)

	// 继续等待
	log.Info("readyCondition not passed, retrying", "results", summarizeResults(results))
	if err := framework.PatchStatusMerge(ctx, r.Client, lt); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: defaultRequeue}, nil
}

// annotationTargetSpecHash 用于存储 target spec hash 的 annotation key。
const annotationTargetSpecHash = "infra.testplane.io/target-spec-hash"

// annotationSelectorResolved 用于标记 selector target 已解析的 annotation key。
const annotationSelectorResolved = "infra.testplane.io/selector-resolved"

// applyAndResolveTarget 应用并解析 target 资源。
// 使用 annotation 存储 hash 避免重复 apply，防止与其他 controller 的 SSA 冲突。
func (r *LoadTestReconciler) applyAndResolveTarget(ctx context.Context, lt *infrav1alpha1.LoadTest) (*unstructured.Unstructured, error) {
	log := logf.FromContext(ctx)

	// 如果有 Manifest，先展开并应用
	if len(lt.Spec.Target.Resource.Manifest.Raw) > 0 {
		manifest, err := resource.ExpandRawTemplate(&lt.Spec.Target.Resource.Manifest, lt.Namespace)
		if err != nil {
			return nil, fmt.Errorf("expand target template: %w", err)
		}

		// 计算当前 target spec 的 hash
		currentHash := computeTemplateHash(&lt.Spec.Target.Resource.Manifest)
		savedHash := lt.GetAnnotations()[annotationTargetSpecHash]

		// 只在 hash 变化时 apply，避免重复 apply 导致 SSA 冲突
		needEmitEvent := false
		if currentHash != savedHash {
			log.Info("target template changed, applying resource", "oldHash", savedHash, "newHash", currentHash)
			if err := r.ResourceManager.ApplyObject(ctx, lt, manifest.Object); err != nil {
				log.Error(err, "failed to apply target resource")
				return nil, err
			}
			// 更新 annotation 中的 hash
			if err := r.updateTargetSpecHashAnnotation(ctx, lt, currentHash); err != nil {
				return nil, err
			}
			needEmitEvent = true
		} else {
			log.V(1).Info("target template unchanged, skipping apply", "hash", currentHash)
		}

		// 获取已应用的资源
		target, err := r.getResourceByManifest(ctx, manifest)
		if err != nil {
			log.Error(err, "failed to get target resource")
			return nil, err
		}

		// 只在实际 apply 时发送事件，避免重复
		if needEmitEvent {
			framework.EmitNormalEvent(r.Recorder, lt, EventReasonTargetApplied,
				fmt.Sprintf("Target %s/%s resolved", target.GetKind(), target.GetName()))
		}
		return target, nil
	}

	// 如果有 Selector，直接获取资源
	if lt.Spec.Target.Resource.Selector != nil {
		target, err := r.getResourceBySelector(ctx, lt, *lt.Spec.Target.Resource.Selector)
		if err != nil {
			log.Error(err, "failed to get target by selector")
			return nil, err
		}

		// 检查是否已解析过，避免重复发送事件
		if lt.GetAnnotations()[annotationSelectorResolved] != "true" {
			if err := r.markSelectorResolved(ctx, lt); err != nil {
				return nil, err
			}
			framework.EmitNormalEvent(r.Recorder, lt, EventReasonTargetApplied,
				fmt.Sprintf("Target %s/%s resolved", target.GetKind(), target.GetName()))
		}
		return target, nil
	}

	return nil, fmt.Errorf("target requires either template or selector")
}

// updateTargetSpecHashAnnotation 更新 LoadTest 的 target spec hash annotation。
// 使用 MergePatch 只更新 annotation，避免与其他控制器的并发冲突。
func (r *LoadTestReconciler) updateTargetSpecHashAnnotation(ctx context.Context, lt *infrav1alpha1.LoadTest, hash string) error {
	patch := client.MergeFrom(lt.DeepCopy())

	annotations := lt.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotationTargetSpecHash] = hash
	lt.SetAnnotations(annotations)

	if err := r.Patch(ctx, lt, patch); err != nil {
		return fmt.Errorf("patch target spec hash annotation: %w", err)
	}
	return nil
}

// markSelectorResolved 标记 selector target 已解析。
func (r *LoadTestReconciler) markSelectorResolved(ctx context.Context, lt *infrav1alpha1.LoadTest) error {
	patch := client.MergeFrom(lt.DeepCopy())

	annotations := lt.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[annotationSelectorResolved] = "true"
	lt.SetAnnotations(annotations)

	if err := r.Patch(ctx, lt, patch); err != nil {
		return fmt.Errorf("patch selector resolved annotation: %w", err)
	}
	return nil
}

// computeTemplateHash 计算 template 的 SHA256 hash。
func computeTemplateHash(template *runtime.RawExtension) string {
	if template == nil || len(template.Raw) == 0 {
		return ""
	}
	hash := sha256.Sum256(template.Raw)
	return hex.EncodeToString(hash[:])
}

// getResourceByManifest 根据 ExpandedManifest 获取资源。
func (r *LoadTestReconciler) getResourceByManifest(ctx context.Context, manifest *resource.ExpandedManifest) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(manifest.Object.GetAPIVersion())
	obj.SetKind(manifest.Object.GetKind())

	key := client.ObjectKey{
		Namespace: manifest.Object.GetNamespace(),
		Name:      manifest.Object.GetName(),
	}
	if err := r.Get(ctx, key, obj); err != nil {
		return nil, fmt.Errorf("get target %s/%s: %w", manifest.Object.GetKind(), manifest.Object.GetName(), err)
	}
	return obj, nil
}

func (r *LoadTestReconciler) getResourceBySelector(ctx context.Context, lt *infrav1alpha1.LoadTest, sel infrav1alpha1.ResourceSelector) (*unstructured.Unstructured, error) {
	ns := sel.Namespace
	if ns == "" {
		ns = lt.Namespace
	}

	if sel.Name != "" {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(sel.APIVersion)
		obj.SetKind(sel.Kind)
		if err := r.Get(ctx, client.ObjectKey{Namespace: ns, Name: sel.Name}, obj); err != nil {
			return nil, fmt.Errorf("get target %s/%s: %w", sel.Kind, sel.Name, err)
		}
		return obj, nil
	}

	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(sel.APIVersion)
	list.SetKind(sel.Kind)

	opts := []client.ListOption{client.InNamespace(ns)}
	if len(sel.LabelSelector) > 0 {
		selector := labels.SelectorFromSet(sel.LabelSelector)
		opts = append(opts, client.MatchingLabelsSelector{Selector: selector})
	}

	if err := r.List(ctx, list, opts...); err != nil {
		return nil, fmt.Errorf("list %s resources: %w", sel.Kind, err)
	}

	candidates := list.Items
	if len(sel.AnnotationSelector) > 0 {
		filtered := make([]unstructured.Unstructured, 0, len(list.Items))
		for _, item := range list.Items {
			if matchAnnotations(item.GetAnnotations(), sel.AnnotationSelector) {
				filtered = append(filtered, item)
			}
		}
		candidates = filtered
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no %s resource found for selector", sel.Kind)
	}

	return &candidates[0], nil
}

// getTargetResource 获取 target 资源（便利包装函数）。
func (r *LoadTestReconciler) getTargetResource(ctx context.Context, lt *infrav1alpha1.LoadTest) (*unstructured.Unstructured, error) {
	// 如果有 Manifest，先展开获取 manifest 然后查询
	if len(lt.Spec.Target.Resource.Manifest.Raw) > 0 {
		manifest, err := resource.ExpandRawTemplate(&lt.Spec.Target.Resource.Manifest, lt.Namespace)
		if err != nil {
			return nil, fmt.Errorf("expand target template: %w", err)
		}
		return r.getResourceByManifest(ctx, manifest)
	}

	// 如果有 Selector，直接查询
	if lt.Spec.Target.Resource.Selector != nil {
		return r.getResourceBySelector(ctx, lt, *lt.Spec.Target.Resource.Selector)
	}

	return nil, fmt.Errorf("target requires either manifest or selector")
}

// matchAnnotations 检查资源的注解是否匹配选择器中的所有注解。
func matchAnnotations(annotations, selector map[string]string) bool {
	for k, v := range selector {
		if annotations[k] != v {
			return false
		}
	}
	return true
}
