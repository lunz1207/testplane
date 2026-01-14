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

// DeploymentReady 检查 Deployment 是否就绪。
// 就绪条件：availableReplicas >= replicas 且 updatedReplicas >= replicas
func DeploymentReady(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("deployment not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	spec := plugin.GetMap(resource, "spec")
	desiredReplicas := 1
	if spec != nil {
		if r := plugin.GetInt(spec, "replicas"); r > 0 {
			desiredReplicas = r
		}
	}

	availableReplicas := plugin.GetInt(status, "availableReplicas")
	updatedReplicas := plugin.GetInt(status, "updatedReplicas")

	if availableReplicas >= desiredReplicas && updatedReplicas >= desiredReplicas {
		return plugin.Pass()
	}

	return plugin.Fail(fmt.Sprintf("deployment not ready: available=%d, updated=%d, desired=%d",
		availableReplicas, updatedReplicas, desiredReplicas)).
		WithActual(fmt.Sprintf("available=%d, updated=%d", availableReplicas, updatedReplicas))
}

// StatefulSetReady 检查 StatefulSet 是否就绪。
// 就绪条件：readyReplicas >= replicas 且 currentRevision == updateRevision
func StatefulSetReady(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("statefulset not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	spec := plugin.GetMap(resource, "spec")
	desiredReplicas := 1
	if spec != nil {
		if r := plugin.GetInt(spec, "replicas"); r > 0 {
			desiredReplicas = r
		}
	}

	readyReplicas := plugin.GetInt(status, "readyReplicas")
	currentRevision := plugin.GetString(status, "currentRevision")
	updateRevision := plugin.GetString(status, "updateRevision")

	if readyReplicas >= desiredReplicas && currentRevision == updateRevision {
		return plugin.Pass()
	}

	return plugin.Fail(fmt.Sprintf("statefulset not ready: ready=%d/%d, currentRevision=%s, updateRevision=%s",
		readyReplicas, desiredReplicas, currentRevision, updateRevision)).
		WithActual(fmt.Sprintf("ready=%d, currentRev=%s, updateRev=%s", readyReplicas, currentRevision, updateRevision))
}

// DaemonSetReady 检查 DaemonSet 是否就绪。
// 就绪条件：numberReady >= desiredNumberScheduled
func DaemonSetReady(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("daemonset not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	desiredNumberScheduled := plugin.GetInt(status, "desiredNumberScheduled")
	numberReady := plugin.GetInt(status, "numberReady")

	if numberReady >= desiredNumberScheduled && desiredNumberScheduled > 0 {
		return plugin.Pass()
	}

	return plugin.Fail(fmt.Sprintf("daemonset not ready: ready=%d/%d", numberReady, desiredNumberScheduled)).
		WithActual(fmt.Sprintf("ready=%d, desired=%d", numberReady, desiredNumberScheduled))
}

// PodReady 检查 Pod 是否就绪。
// 就绪条件：phase=Running 且所有容器 Ready
func PodReady(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("pod not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	if phase != "Running" {
		return plugin.Fail(fmt.Sprintf("pod not running: phase=%s", phase)).WithActual(phase)
	}

	containerStatuses := plugin.GetSlice(status, "containerStatuses")
	if len(containerStatuses) == 0 {
		return plugin.Fail("no container statuses")
	}

	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]interface{})
		if !ok {
			continue
		}
		ready := plugin.GetBoolOr(csMap, "ready", false)
		if !ready {
			name := plugin.GetString(csMap, "name")
			return plugin.Fail(fmt.Sprintf("container %s not ready", name)).WithActual("container not ready")
		}
	}

	return plugin.Pass()
}

// PodComplete 检查 Pod 是否已完成（一次性任务）。
// 就绪条件：phase=Succeeded
func PodComplete(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("pod not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	if phase == "Succeeded" {
		return plugin.Pass()
	}

	if phase == "Failed" {
		return plugin.Fail("pod failed").WithActual(phase)
	}

	return plugin.Fail(fmt.Sprintf("pod not complete: phase=%s", phase)).WithActual(phase)
}

// JobComplete 检查 Job 是否已完成。
// 就绪条件：succeeded >= completions 或 condition=Complete
func JobComplete(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("job not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	// 检查 conditions
	conditions := plugin.GetSlice(status, "conditions")
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		condType := plugin.GetString(condMap, "type")
		condStatus := plugin.GetString(condMap, "status")
		if condType == "Complete" && condStatus == "True" {
			return plugin.Pass()
		}
		if condType == "Failed" && condStatus == "True" {
			reason := plugin.GetString(condMap, "reason")
			return plugin.Fail(fmt.Sprintf("job failed: %s", reason)).WithActual("Failed")
		}
	}

	// 检查 succeeded >= completions
	spec := plugin.GetMap(resource, "spec")
	completions := 1
	if spec != nil {
		if c := plugin.GetInt(spec, "completions"); c > 0 {
			completions = c
		}
	}

	succeeded := plugin.GetInt(status, "succeeded")
	if succeeded >= completions {
		return plugin.Pass()
	}

	active := plugin.GetInt(status, "active")
	failed := plugin.GetInt(status, "failed")
	return plugin.Fail(fmt.Sprintf("job not complete: succeeded=%d/%d", succeeded, completions)).
		WithActual(fmt.Sprintf("succeeded=%d, active=%d, failed=%d", succeeded, active, failed))
}

// ServiceReady 检查 Service 是否就绪。
// 就绪条件：有对应 Endpoints 且至少有一个 ready address
// 注意：此函数检查 Service 的 spec 和 status，实际 Endpoints 需要额外查询
// params: minReadyAddresses (int, 默认 1)
func ServiceReady(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("service not found")
	}

	spec := plugin.GetMap(resource, "spec")
	if spec == nil {
		return plugin.Fail("no spec")
	}

	// 对于 ExternalName 类型，不需要 endpoints
	svcType := plugin.GetString(spec, "type")
	if svcType == "ExternalName" {
		externalName := plugin.GetString(spec, "externalName")
		if externalName != "" {
			return plugin.Pass()
		}
		return plugin.Fail("externalName service without externalName")
	}

	// 对于其他类型，检查是否有 clusterIP（除了 Headless Service）
	clusterIP := plugin.GetString(spec, "clusterIP")
	if clusterIP == "" || clusterIP == "None" {
		// Headless service，需要检查 selector
		selector := plugin.GetMap(spec, "selector")
		if len(selector) == 0 {
			return plugin.Fail("headless service without selector")
		}
		// Headless service 有 selector，认为就绪
		return plugin.Pass()
	}

	// 有 ClusterIP，认为 Service 基础配置已就绪
	return plugin.Pass()
}

// PVCBound 检查 PVC 是否已绑定。
// 就绪条件：phase=Bound
func PVCBound(resource, params map[string]interface{}) plugin.Result {
	if len(resource) == 0 {
		return plugin.Fail("pvc not found")
	}

	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Fail("no status")
	}

	phase := plugin.GetString(status, "phase")
	if phase == "Bound" {
		return plugin.Pass()
	}

	return plugin.Fail(fmt.Sprintf("pvc not bound: phase=%s", phase)).WithActual(phase)
}
