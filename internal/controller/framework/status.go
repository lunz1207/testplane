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

package framework

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
)

// 默认 FieldOwner 常量
const (
	FieldOwnerIntegrationTest = "integrationtest-controller"
	FieldOwnerLoadTest        = "loadtest-controller"
)

// PatchIntegrationTestStatus 使用纯正 SSA 更新 IntegrationTest 状态。
// 构造干净的 Apply Configuration，只包含标识字段和 status，不依赖 GET 到的对象。
func PatchIntegrationTestStatus(ctx context.Context, c client.Client, name, namespace string, status infrav1alpha1.IntegrationTestStatus) error {
	patch := &infrav1alpha1.IntegrationTest{}
	patch.SetName(name)
	patch.SetNamespace(namespace)
	patch.SetGroupVersionKind(infrav1alpha1.GroupVersion.WithKind("IntegrationTest"))
	patch.Status = status

	return c.Status().Patch(ctx, patch, client.Apply,
		client.FieldOwner(FieldOwnerIntegrationTest),
		client.ForceOwnership,
	)
}

// PatchIntegrationTestStatusFromObject 便捷函数，直接从对象更新状态。
func PatchIntegrationTestStatusFromObject(ctx context.Context, c client.Client, it *infrav1alpha1.IntegrationTest) error {
	return PatchIntegrationTestStatus(ctx, c, it.Name, it.Namespace, it.Status)
}

// PatchLoadTestStatus 使用纯正 SSA 更新 LoadTest 状态。
// 构造干净的 Apply Configuration，只包含标识字段和 status，不依赖 GET 到的对象。
func PatchLoadTestStatus(ctx context.Context, c client.Client, name, namespace string, status infrav1alpha1.LoadTestStatus) error {
	patch := &infrav1alpha1.LoadTest{}
	patch.SetName(name)
	patch.SetNamespace(namespace)
	patch.SetGroupVersionKind(infrav1alpha1.GroupVersion.WithKind("LoadTest"))
	patch.Status = status

	return c.Status().Patch(ctx, patch, client.Apply,
		client.FieldOwner(FieldOwnerLoadTest),
		client.ForceOwnership,
	)
}

// PatchLoadTestStatusFromObject 便捷函数，直接从对象更新状态。
func PatchLoadTestStatusFromObject(ctx context.Context, c client.Client, lt *infrav1alpha1.LoadTest) error {
	return PatchLoadTestStatus(ctx, c, lt.Name, lt.Namespace, lt.Status)
}

// PatchStatusSSA 使用 Server-Side Apply 更新 status（通用版本，保留向后兼容）。
// 推荐使用类型安全的 PatchIntegrationTestStatus 或 PatchLoadTestStatus。
func PatchStatusSSA(ctx context.Context, c client.Client, obj client.Object, fieldOwner string) error {
	// 构造干净的 Apply Configuration，只包含必要的标识字段和 status
	switch o := obj.(type) {
	case *infrav1alpha1.IntegrationTest:
		return PatchIntegrationTestStatus(ctx, c, o.Name, o.Namespace, o.Status)
	case *infrav1alpha1.LoadTest:
		return PatchLoadTestStatus(ctx, c, o.Name, o.Namespace, o.Status)
	default:
		return fmt.Errorf("unsupported type for SSA status patch: %T", obj)
	}
}

// PatchStatusMerge 使用 SSA 更新 status 的便利函数（保留向后兼容）。
// 推荐使用类型安全的 PatchIntegrationTestStatus 或 PatchLoadTestStatus。
func PatchStatusMerge(ctx context.Context, c client.Client, obj client.Object) error {
	return PatchStatusSSA(ctx, c, obj, "")
}
