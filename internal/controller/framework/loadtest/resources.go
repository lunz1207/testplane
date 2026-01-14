package loadtest

import (
	"context"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework/resource"
)

// expandResources 将 []ResourceRef 的模板展开为 ExpandedManifest 列表（支持 List/数组）。
// 需要时可对模板内容进行占位符替换（用于 workload 注入）。
func (r *LoadTestReconciler) expandResources(lt *infrav1alpha1.LoadTest, resources []infrav1alpha1.ResourceRef, replacements map[string]string) ([]resource.ExpandedManifest, error) {
	return resource.ExpandResourceRefs(resources, lt.Namespace, replacements)
}

// applyResources 批量应用资源。
// 资源通过 ownerRef 关联到 LoadTest，删除时 GC 自动清理。
func (r *LoadTestReconciler) applyResources(ctx context.Context, lt *infrav1alpha1.LoadTest, manifests []resource.ExpandedManifest) error {
	return r.ResourceManager.ExecuteManifests(ctx, lt, manifests)
}
