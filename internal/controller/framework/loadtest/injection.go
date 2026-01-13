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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	infrav1alpha1 "github.com/lunz1207/testplane/api/v1alpha1"
	"github.com/lunz1207/testplane/internal/controller/framework/plugin"
	"github.com/lunz1207/testplane/internal/controller/framework/resource"
)

// resolveEnvInjection 解析环境变量注入配置。
// 使用统一的 Function 从目标资源提取值（通过 Result.Value）。
func (r *LoadTestReconciler) resolveEnvInjection(target *unstructured.Unstructured, injections []infrav1alpha1.EnvInjection) (map[string]string, error) {
	values := make(map[string]string)

	for _, inj := range injections {
		// 检查函数是否存在
		if !r.PluginRegistry.Has(inj.Extract.Function) {
			return nil, fmt.Errorf("unknown function: %s", inj.Extract.Function)
		}

		// 执行函数并获取提取值
		result, err := r.PluginRegistry.Call(inj.Extract.Function, target.Object, inj.Extract.Params.Raw)
		if err != nil {
			return nil, fmt.Errorf("run function %s for %s: %w", inj.Extract.Function, inj.Name, err)
		}

		values[inj.Name] = result.Value
	}

	return values, nil
}

// ensurePluginRegistry 确保提取器注册表已初始化。
func (r *LoadTestReconciler) ensurePluginRegistry() {
	if r.PluginRegistry == nil {
		r.PluginRegistry = plugin.DefaultRegistry
	}
}

// ensureResourceManager 确保资源管理器已初始化。
func (r *LoadTestReconciler) ensureResourceManager() {
	if r.ResourceManager == nil {
		r.ResourceManager = resource.NewManager(r.Client, r.Scheme, loadTestFieldOwner, r.APIReader)
	}
}
