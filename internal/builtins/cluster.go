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
	"strings"

	"github.com/lunz1207/testplane/internal/plugin"
)

// ClusterReady 检查集群是否就绪（phase=active 且无 transitionStatus）。
func ClusterReady(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "active" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("cluster not ready").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterHealthy 检查集群是否健康（phase=active、health=healthy、transitionStatus 为空）。
func ClusterHealthy(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := strings.TrimSpace(plugin.GetString(status, "phase"))
	health := strings.TrimSpace(plugin.GetString(status, "health"))
	transition := strings.TrimSpace(plugin.GetString(status, "transitionStatus"))

	if phase == "active" && strings.EqualFold(health, "healthy") && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("cluster not healthy").WithActual(fmt.Sprintf("phase=%s, health=%s, transition=%s", phase, health, transition))
}

// ClusterNodeCount 检查集群节点数量。
// params: expected (int)
func ClusterNodeCount(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	nodes := plugin.GetSlice(status, "nodes")
	actual := len(nodes)
	expected := plugin.GetInt(params, "expected")

	if actual == expected {
		return plugin.Pass()
	}
	return plugin.Fail(fmt.Sprintf("expected %d nodes, got %d", expected, actual)).WithActual(actual)
}

// ClusterSecurityGroupExists 检查集群安全组是否存在。
// params: id (string, 可选), expected (bool, 默认 true)
func ClusterSecurityGroupExists(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	sgs := plugin.GetSlice(status, "securityGroups")
	expectedID := plugin.GetString(params, "id")
	expectExists := plugin.GetBoolOr(params, "expected", true)

	// 统计已绑定的安全组
	attachedCount := 0
	found := false
	for _, sg := range sgs {
		sgMap, ok := sg.(map[string]interface{})
		if !ok {
			continue
		}
		state := plugin.GetString(sgMap, "state")
		if state == "Attached" || state == "" {
			attachedCount++
			if expectedID != "" && plugin.GetString(sgMap, "id") == expectedID {
				found = true
			}
		}
	}

	// 检查特定 ID
	if expectedID != "" {
		if expectExists && found {
			return plugin.Pass()
		}
		if !expectExists && !found {
			return plugin.Pass()
		}
		if expectExists {
			return plugin.Fail(fmt.Sprintf("security group %s not attached", expectedID))
		}
		return plugin.Fail(fmt.Sprintf("security group %s still attached", expectedID))
	}

	// 检查是否有任何安全组
	hasAny := attachedCount > 0
	if expectExists && hasAny {
		return plugin.Pass()
	}
	if !expectExists && !hasAny {
		return plugin.Pass()
	}
	if expectExists {
		return plugin.Fail("no security groups attached")
	}
	return plugin.Fail(fmt.Sprintf("%d security groups still attached", attachedCount))
}

// ClusterSecurityGroupNotExists 检查集群安全组是否不存在。
// params: id (string, 可选)
func ClusterSecurityGroupNotExists(resource, params map[string]interface{}) plugin.Result {
	newParams := map[string]interface{}{
		"id":       plugin.GetString(params, "id"),
		"expected": false,
	}
	return ClusterSecurityGroupExists(resource, newParams)
}

// ClusterPhaseEquals 通用的集群 phase 检查函数。
// params: phase (string, 必填), ignoreTransition (bool, 默认 false)
// 集群 phase 可选值：pending, active, stopped, deleted, ceased
func ClusterPhaseEquals(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	actualPhase := plugin.GetString(status, "phase")
	expectedPhase := plugin.GetString(params, "phase")
	transition := plugin.GetString(status, "transitionStatus")
	ignoreTransition := plugin.GetBoolOr(params, "ignoreTransition", false)

	if expectedPhase == "" {
		return plugin.Fail("missing required param: phase")
	}

	if actualPhase != expectedPhase {
		return plugin.Fail(fmt.Sprintf("expected phase=%s", expectedPhase)).WithActual(fmt.Sprintf("phase=%s, transition=%s", actualPhase, transition))
	}

	if !ignoreTransition && transition != "" {
		return plugin.Fail(fmt.Sprintf("phase=%s but has transition status", expectedPhase)).WithActual(fmt.Sprintf("phase=%s, transition=%s", actualPhase, transition))
	}

	return plugin.Pass()
}

// ClusterPending 检查集群是否处于 pending 状态（phase=pending 且无 transitionStatus）。
func ClusterPending(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "pending" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("cluster not pending").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterStopped 检查集群是否已停止（phase=stopped 且无 transitionStatus）。
func ClusterStopped(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "stopped" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("cluster not stopped").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterDeleted 检查集群是否已删除（phase=deleted 且无 transitionStatus）。
func ClusterDeleted(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "deleted" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("cluster not deleted").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterCeased 检查集群是否已销毁（phase=ceased 且无 transitionStatus）。
func ClusterCeased(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "ceased" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("cluster not ceased").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}
