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

package resource

import (
	"context"
	stderrors "errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ErrResourceNotReady 表示资源的 Controller 尚未处理完最新的 spec。
// 调用方应该 requeue 等待，而不是将此视为失败。
var ErrResourceNotReady = stderrors.New("resource not ready: observedGeneration < generation")

// Manager 提供资源管理功能（应用、删除、等待、状态收集）。
type Manager struct {
	Client     client.Client
	Scheme     *runtime.Scheme
	FieldOwner string
	APIReader  client.Reader // 用于 waitResourcesConverge 绕过缓存检查收敛状态
}

// NewManager 创建一个新的资源管理器。
func NewManager(c client.Client, scheme *runtime.Scheme, fieldOwner string, apiReader client.Reader) *Manager {
	return &Manager{
		Client:     c,
		Scheme:     scheme,
		FieldOwner: fieldOwner,
		APIReader:  apiReader,
	}
}

// ExecuteManifest 执行单个资源清单（Apply 或 Delete）。
func (m *Manager) ExecuteManifest(ctx context.Context, owner client.Object, manifest *ExpandedManifest) error {
	if manifest == nil {
		return nil
	}
	log := logf.FromContext(ctx)
	log.Info("executing manifest",
		"kind", manifest.Object.GetKind(),
		"name", manifest.Object.GetName(),
		"action", manifest.Action)

	if manifest.IsDelete() {
		if err := m.DeleteObject(ctx, manifest.Object); err != nil {
			return fmt.Errorf("failed to delete %s/%s: %w",
				manifest.Object.GetKind(), manifest.Object.GetName(), err)
		}
	} else {
		if err := m.ApplyObject(ctx, owner, manifest.Object); err != nil {
			return fmt.Errorf("failed to apply %s/%s: %w",
				manifest.Object.GetKind(), manifest.Object.GetName(), err)
		}
	}
	return nil
}

// ExecuteManifests 批量执行资源清单（Apply 或 Delete）。
// 所有资源必须与 owner 在同一命名空间，通过 ownerRef 管理生命周期。
func (m *Manager) ExecuteManifests(ctx context.Context, owner client.Object, manifests []ExpandedManifest) error {
	log := logf.FromContext(ctx)
	for _, manifest := range manifests {
		log.Info("executing manifest",
			"kind", manifest.Object.GetKind(),
			"name", manifest.Object.GetName(),
			"action", manifest.Action)

		if manifest.IsDelete() {
			if err := m.DeleteObject(ctx, manifest.Object); err != nil {
				return fmt.Errorf("failed to delete %s/%s: %w",
					manifest.Object.GetKind(), manifest.Object.GetName(), err)
			}
		} else {
			if err := m.ApplyObject(ctx, owner, manifest.Object); err != nil {
				return fmt.Errorf("failed to apply %s/%s: %w",
					manifest.Object.GetKind(), manifest.Object.GetName(), err)
			}
		}
	}
	return nil
}

// ApplyObject 应用单个资源（创建或更新）。
// 使用 Server-Side Apply 统一处理，无需预先检查资源是否存在。
// 资源通过 OwnerReference 关联到 owner，删除时 GC 自动清理。
func (m *Manager) ApplyObject(ctx context.Context, owner client.Object, obj *unstructured.Unstructured) error {
	log := logf.FromContext(ctx)

	namespace := obj.GetNamespace()
	if namespace == "" {
		namespace = owner.GetNamespace()
		obj.SetNamespace(namespace)
	}

	// 强制要求同命名空间
	if namespace != owner.GetNamespace() {
		return fmt.Errorf("cross-namespace resource not allowed: resource %s/%s is in namespace %q, but owner is in namespace %q",
			obj.GetKind(), obj.GetName(), namespace, owner.GetNamespace())
	}

	// 设置 OwnerReference，owner 删除时 GC 自动清理资源
	if err := controllerutil.SetOwnerReference(owner, obj, m.Scheme); err != nil {
		return fmt.Errorf("set owner reference for %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}

	log.Info("applying resource via SSA with owner reference",
		"apiVersion", obj.GetAPIVersion(),
		"kind", obj.GetKind(),
		"name", obj.GetName(),
		"namespace", namespace)

	// 使用 Server-Side Apply
	if err := m.Client.Patch(ctx, obj, client.Apply,
		client.FieldOwner(m.FieldOwner)); err != nil {
		return fmt.Errorf("apply resource %s/%s via SSA: %w", obj.GetKind(), obj.GetName(), err)
	}

	log.Info("successfully applied resource",
		"apiVersion", obj.GetAPIVersion(),
		"kind", obj.GetKind(),
		"name", obj.GetName(),
		"namespace", namespace)

	return nil
}

// DeleteObject 删除单个资源。
// 如果资源不存在，视为已删除成功。
func (m *Manager) DeleteObject(ctx context.Context, obj *unstructured.Unstructured) error {
	log := logf.FromContext(ctx)

	// 先检查资源是否存在
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion(obj.GetAPIVersion())
	existing.SetKind(obj.GetKind())

	key := client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	err := m.Client.Get(ctx, key, existing)
	if errors.IsNotFound(err) {
		log.Info("resource already deleted or not exists",
			"kind", obj.GetKind(),
			"name", obj.GetName(),
			"namespace", obj.GetNamespace())
		return nil
	}
	if err != nil {
		return fmt.Errorf("get resource for deletion: %w", err)
	}

	log.Info("deleting resource",
		"kind", obj.GetKind(),
		"name", obj.GetName(),
		"namespace", obj.GetNamespace())

	if err := m.Client.Delete(ctx, existing); err != nil {
		return fmt.Errorf("delete resource %s/%s: %w", obj.GetKind(), obj.GetName(), err)
	}

	return nil
}

// WaitForManifest 等待单个资源清单收敛。
func (m *Manager) WaitForManifest(ctx context.Context, manifest *ExpandedManifest) error {
	if manifest == nil {
		return nil
	}
	log := logf.FromContext(ctx)
	if err := m.WaitForObject(ctx, manifest.Object, manifest.IsDelete()); err != nil {
		return err
	}
	log.V(1).Info("manifest converged",
		"kind", manifest.Object.GetKind(),
		"name", manifest.Object.GetName())
	return nil
}

// WaitForManifests 等待资源清单收敛。
func (m *Manager) WaitForManifests(ctx context.Context, manifests []ExpandedManifest) error {
	log := logf.FromContext(ctx)
	for _, manifest := range manifests {
		if err := m.WaitForObject(ctx, manifest.Object, manifest.IsDelete()); err != nil {
			return err
		}
	}
	log.V(1).Info("all manifests converged", "count", len(manifests))
	return nil
}

// WaitForObject 等待单个资源收敛（删除或 spec 已被处理）。
// 收敛的定义：控制器已经处理了最新的 spec（observedGeneration >= generation）。
func (m *Manager) WaitForObject(ctx context.Context, obj *unstructured.Unstructured, isDelete bool) error {
	log := logf.FromContext(ctx)

	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion(obj.GetAPIVersion())
	existing.SetKind(obj.GetKind())

	key := client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	err := m.Client.Get(ctx, key, existing)

	if isDelete {
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		log.Info("waiting for deletion", "kind", obj.GetKind(), "name", obj.GetName())
		return fmt.Errorf("resource %s/%s still exists", obj.GetKind(), obj.GetName())
	}

	// 资源尚未创建，返回 ErrResourceNotReady 让调用方 requeue
	if errors.IsNotFound(err) {
		log.Info("resource not found, waiting for creation", "kind", obj.GetKind(), "name", obj.GetName())
		return fmt.Errorf("%w: %s/%s not found", ErrResourceNotReady, obj.GetKind(), obj.GetName())
	}
	if err != nil {
		return err
	}

	// 检查 observedGeneration：确保控制器已处理最新 spec
	gen := existing.GetGeneration()
	observed, found, _ := unstructured.NestedInt64(existing.Object, "status", "observedGeneration")
	if found && observed < gen {
		log.Info("waiting for generation sync",
			"kind", obj.GetKind(),
			"name", obj.GetName(),
			"gen", gen,
			"observed", observed)
		return fmt.Errorf("%w: %s/%s observedGeneration=%d < generation=%d",
			ErrResourceNotReady, obj.GetKind(), obj.GetName(), observed, gen)
	}

	return nil
}

// GatherManifestState 获取单个资源清单的当前状态，用于期望检查。
func (m *Manager) GatherManifestState(ctx context.Context, manifest *ExpandedManifest) (map[string]interface{}, error) {
	if manifest == nil {
		return make(map[string]interface{}), nil
	}
	log := logf.FromContext(ctx)
	obj := manifest.Object
	keyStr := manifest.StateKey()

	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion(obj.GetAPIVersion())
	existing.SetKind(obj.GetKind())

	key := client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	err := m.Client.Get(ctx, key, existing)

	if manifest.IsDelete() {
		if errors.IsNotFound(err) {
			return map[string]interface{}{keyStr: map[string]interface{}{}}, nil
		}
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{keyStr: existing.Object}, nil
	}

	if errors.IsNotFound(err) {
		log.Info("resource not found for expectation check", "kind", obj.GetKind(), "name", obj.GetName())
		return nil, fmt.Errorf("%w: %s/%s not found", ErrResourceNotReady, obj.GetKind(), obj.GetName())
	}
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{keyStr: existing.Object}, nil
}

// GatherManifestStates 获取指定资源清单的当前状态，用于期望检查。
func (m *Manager) GatherManifestStates(ctx context.Context, manifests []ExpandedManifest) (map[string]interface{}, error) {
	log := logf.FromContext(ctx)
	state := make(map[string]interface{})

	for _, manifest := range manifests {
		obj := manifest.Object
		keyStr := manifest.StateKey()

		existing := &unstructured.Unstructured{}
		existing.SetAPIVersion(obj.GetAPIVersion())
		existing.SetKind(obj.GetKind())

		key := client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}

		err := m.Client.Get(ctx, key, existing)

		if manifest.IsDelete() {
			if errors.IsNotFound(err) {
				state[keyStr] = map[string]interface{}{}
				continue
			}
			if err != nil {
				return nil, err
			}
			state[keyStr] = existing.Object
			continue
		}

		if errors.IsNotFound(err) {
			log.Info("resource not found for expectation check", "kind", obj.GetKind(), "name", obj.GetName())
			return nil, fmt.Errorf("%w: %s/%s not found", ErrResourceNotReady, obj.GetKind(), obj.GetName())
		}
		if err != nil {
			return nil, err
		}

		state[keyStr] = existing.Object
	}

	return state, nil
}

// GatherObjectState 获取单个资源的当前状态。
func (m *Manager) GatherObjectState(ctx context.Context, obj *unstructured.Unstructured) (map[string]interface{}, error) {
	existing := &unstructured.Unstructured{}
	existing.SetAPIVersion(obj.GetAPIVersion())
	existing.SetKind(obj.GetKind())

	key := client.ObjectKey{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	if err := m.Client.Get(ctx, key, existing); err != nil {
		if errors.IsNotFound(err) {
			return nil, fmt.Errorf("%w: %s/%s not found", ErrResourceNotReady, obj.GetKind(), obj.GetName())
		}
		return nil, err
	}

	return existing.Object, nil
}

// CheckResourceNotReady 检查资源是否未就绪（observedGeneration < generation）。
// 返回 (notReady, reason)。
func CheckResourceNotReady(obj *unstructured.Unstructured) (bool, string) {
	gen := obj.GetGeneration()
	if gen == 0 {
		// 某些资源可能不使用 generation，跳过检查
		return false, ""
	}

	observed, found, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
	if !found {
		// 没有 observedGeneration 字段，可能是新创建的资源或不支持此字段
		return false, ""
	}

	if observed < gen {
		return true, fmt.Sprintf("observedGeneration=%d < generation=%d", observed, gen)
	}

	return false, ""
}
