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

package builtins

import (
	"fmt"

	"github.com/lunz1207/testplane/internal/plugin"
)

// ResourceExists 检查资源是否存在。
func ResourceExists(resource, params map[string]interface{}) plugin.Result {
	exists := len(resource) > 0
	if exists {
		return plugin.Pass()
	}
	return plugin.Fail("resource not exists")
}

// ResourceNotExists 检查资源是否不存在。
func ResourceNotExists(resource, params map[string]interface{}) plugin.Result {
	exists := len(resource) > 0
	if !exists {
		return plugin.Pass()
	}
	return plugin.Fail("resource still exists")
}

// DeploymentAvailable 检查 Deployment 是否有可用副本。
// 这比 ResourceExists 更有意义，因为它确保 Pod 已就绪。
func DeploymentAvailable(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("deployment not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	availableReplicas := plugin.GetInt(status, "availableReplicas")
	readyReplicas := plugin.GetInt(status, "readyReplicas")

	// 获取期望的副本数（默认为 1）
	spec := plugin.GetMap(resource, "spec")
	desiredReplicas := 1
	if spec != nil {
		if r := plugin.GetInt(spec, "replicas"); r > 0 {
			desiredReplicas = r
		}
	}

	if availableReplicas >= desiredReplicas && readyReplicas >= desiredReplicas {
		return plugin.Pass()
	}

	return plugin.Fail(fmt.Sprintf("deployment not available: %d/%d replicas ready", readyReplicas, desiredReplicas)).
		WithActual(fmt.Sprintf("available=%d, ready=%d, desired=%d", availableReplicas, readyReplicas, desiredReplicas))
}
