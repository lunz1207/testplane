package integrationtest

import (
	"context"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/shared/resource"
)

// ErrResourceNotReady 表示资源的 Controller 尚未处理完最新的 spec。
// 调用方应该 requeue 等待，而不是将此视为失败。
var ErrResourceNotReady = resource.ErrResourceNotReady

// expandStepResource 展开步骤的单个 ResourceRef 为 ExpandedManifest。
// 如果 step.Resource 为空或没有 Manifest，返回 nil。
func (r *IntegrationTestReconciler) expandStepResource(tc *infrav1alpha1.IntegrationTest, step infrav1alpha1.TestStep) (*resource.ExpandedManifest, error) {
	if step.Resource == nil || len(step.Resource.Manifest.Raw) == 0 {
		return nil, nil
	}
	return resource.ExpandSingleResourceRef(*step.Resource, tc.Namespace)
}

// applyResource 应用单个资源。
// 资源通过 ownerRef 关联到 IntegrationTest，删除时 GC 自动清理。
func (r *IntegrationTestReconciler) applyResource(ctx context.Context, tc *infrav1alpha1.IntegrationTest, manifest *resource.ExpandedManifest) error {
	return r.ResourceManager.ExecuteManifest(ctx, tc, manifest)
}

// waitResourceConverge 等待单个资源收敛。
func (r *IntegrationTestReconciler) waitResourceConverge(ctx context.Context, manifest *resource.ExpandedManifest) error {
	return r.ResourceManager.WaitForManifest(ctx, manifest)
}

// gatherResourceState 获取单个资源的当前状态，用于期望检查。
func (r *IntegrationTestReconciler) gatherResourceState(ctx context.Context, manifest *resource.ExpandedManifest) (map[string]interface{}, error) {
	return r.ResourceManager.GatherManifestState(ctx, manifest)
}
