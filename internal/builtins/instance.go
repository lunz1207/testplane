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

// InstanceReady 检查实例是否就绪（phase=running 且无 transitionStatus）。
func InstanceReady(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "running" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("instance not ready").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceStopped 检查实例是否已停止（phase=stopped 且无 transitionStatus）。
func InstanceStopped(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "stopped" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("instance not stopped").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceSecurityGroupExists 检查实例安全组是否存在。
// params: id (string, 可选), expected (bool, 默认 true)
func InstanceSecurityGroupExists(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	sgs := plugin.GetSlice(status, "securityGroups")
	expectedID := plugin.GetString(params, "id")
	expectExists := plugin.GetBoolOr(params, "expected", true)

	// 遍历安全组列表，检查特定 ID 是否存在
	found := false
	attachedCount := len(sgs)

	if expectedID != "" {
		// 检查特定 ID 是否在列表中
		for _, sg := range sgs {
			if sgID, ok := sg.(string); ok && sgID == expectedID {
				found = true
				break
			}
		}

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

	// 未指定 ID，检查是否有任何安全组
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

// InstanceSecurityGroupNotExists 检查实例安全组是否不存在。
func InstanceSecurityGroupNotExists(resource, params map[string]interface{}) plugin.Result {
	newParams := map[string]interface{}{
		"id":       plugin.GetString(params, "id"),
		"expected": false,
	}
	return InstanceSecurityGroupExists(resource, newParams)
}

// InstancePhaseEquals 通用的 phase 检查函数。
// params: phase (string, 必填), ignoreTransition (bool, 默认 false)
func InstancePhaseEquals(resource, params map[string]interface{}) plugin.Result {
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

// InstancePending 检查实例是否处于 pending 状态（phase=pending 且无 transitionStatus）。
func InstancePending(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "pending" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("instance not pending").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceSuspended 检查实例是否已暂停（phase=suspended 且无 transitionStatus）。
func InstanceSuspended(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "suspended" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("instance not suspended").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceTerminated 检查实例是否已终止（phase=terminated 且无 transitionStatus）。
func InstanceTerminated(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "terminated" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("instance not terminated").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceCeased 检查实例是否已销毁（phase=ceased 且无 transitionStatus）。
func InstanceCeased(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	transition := plugin.GetString(status, "transitionStatus")

	if phase == "ceased" && transition == "" {
		return plugin.Pass()
	}
	return plugin.Fail("instance not ceased").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}
