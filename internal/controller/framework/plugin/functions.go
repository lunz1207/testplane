package plugin

import (
	"fmt"
	"strings"
)

// ============ 辅助函数 ============

// getMap 获取 map 字段。
func getMap(data map[string]interface{}, key string) map[string]interface{} {
	if v, ok := data[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

// getString 获取字符串字段。
func getString(data map[string]interface{}, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

// getInt 获取整数字段。
func getInt(data map[string]interface{}, key string) int {
	switch v := data[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

// getBoolOr 获取布尔字段，不存在时返回默认值。
func getBoolOr(data map[string]interface{}, key string, def bool) bool {
	if data == nil {
		return def
	}
	if _, ok := data[key]; !ok {
		return def
	}
	if v, ok := data[key].(bool); ok {
		return v
	}
	return def
}

// getSlice 获取数组字段。
func getSlice(data map[string]interface{}, key string) []interface{} {
	if v, ok := data[key].([]interface{}); ok {
		return v
	}
	return nil
}

// ============ 通用断言 ============

// ResourceExists 检查资源是否存在。
func ResourceExists(resource, params map[string]interface{}) Result {
	exists := len(resource) > 0
	if exists {
		return Pass()
	}
	return Fail("resource not exists")
}

// ResourceNotExists 检查资源是否不存在。
func ResourceNotExists(resource, params map[string]interface{}) Result {
	exists := len(resource) > 0
	if !exists {
		return Pass()
	}
	return Fail("resource still exists")
}

// DeploymentAvailable 检查 Deployment 是否有可用副本。
// 这比 ResourceExists 更有意义，因为它确保 Pod 已就绪。
func DeploymentAvailable(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("deployment not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	availableReplicas := getInt(status, "availableReplicas")
	readyReplicas := getInt(status, "readyReplicas")

	// 获取期望的副本数（默认为 1）
	spec := getMap(resource, "spec")
	desiredReplicas := 1
	if spec != nil {
		if r := getInt(spec, "replicas"); r > 0 {
			desiredReplicas = r
		}
	}

	if availableReplicas >= desiredReplicas && readyReplicas >= desiredReplicas {
		return Pass()
	}

	return Fail(fmt.Sprintf("deployment not available: %d/%d replicas ready", readyReplicas, desiredReplicas)).
		WithActual(fmt.Sprintf("available=%d, ready=%d, desired=%d", availableReplicas, readyReplicas, desiredReplicas))
}

// ============ Cluster 断言 ============

// ClusterReady 检查集群是否就绪（phase=active 且无 transitionStatus）。
func ClusterReady(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "active" && transition == "" {
		return Pass()
	}
	return Fail("cluster not ready").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterHealthy 检查集群是否健康（phase=active、health=healthy、transitionStatus 为空）。
func ClusterHealthy(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := strings.TrimSpace(getString(status, "phase"))
	health := strings.TrimSpace(getString(status, "health"))
	transition := strings.TrimSpace(getString(status, "transitionStatus"))

	if phase == "active" && strings.EqualFold(health, "healthy") && transition == "" {
		return Pass()
	}
	return Fail("cluster not healthy").WithActual(fmt.Sprintf("phase=%s, health=%s, transition=%s", phase, health, transition))
}

// ClusterNodeCount 检查集群节点数量。
// params: expected (int)
func ClusterNodeCount(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	nodes := getSlice(status, "nodes")
	actual := len(nodes)
	expected := getInt(params, "expected")

	if actual == expected {
		return Pass()
	}
	return Fail(fmt.Sprintf("expected %d nodes, got %d", expected, actual)).WithActual(actual)
}

// ClusterSecurityGroupExists 检查集群安全组是否存在。
// params: id (string, 可选), expected (bool, 默认 true)
func ClusterSecurityGroupExists(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	sgs := getSlice(status, "securityGroups")
	expectedID := getString(params, "id")
	expectExists := getBoolOr(params, "expected", true)

	// 统计已绑定的安全组
	attachedCount := 0
	found := false
	for _, sg := range sgs {
		sgMap, ok := sg.(map[string]interface{})
		if !ok {
			continue
		}
		state := getString(sgMap, "state")
		if state == "Attached" || state == "" {
			attachedCount++
			if expectedID != "" && getString(sgMap, "id") == expectedID {
				found = true
			}
		}
	}

	// 检查特定 ID
	if expectedID != "" {
		if expectExists && found {
			return Pass()
		}
		if !expectExists && !found {
			return Pass()
		}
		if expectExists {
			return Fail(fmt.Sprintf("security group %s not attached", expectedID))
		}
		return Fail(fmt.Sprintf("security group %s still attached", expectedID))
	}

	// 检查是否有任何安全组
	hasAny := attachedCount > 0
	if expectExists && hasAny {
		return Pass()
	}
	if !expectExists && !hasAny {
		return Pass()
	}
	if expectExists {
		return Fail("no security groups attached")
	}
	return Fail(fmt.Sprintf("%d security groups still attached", attachedCount))
}

// ClusterSecurityGroupNotExists 检查集群安全组是否不存在。
// params: id (string, 可选)
func ClusterSecurityGroupNotExists(resource, params map[string]interface{}) Result {
	newParams := map[string]interface{}{
		"id":       getString(params, "id"),
		"expected": false,
	}
	return ClusterSecurityGroupExists(resource, newParams)
}

// ClusterPhaseEquals 通用的集群 phase 检查函数。
// params: phase (string, 必填), ignoreTransition (bool, 默认 false)
// 集群 phase 可选值：pending, active, stopped, deleted, ceased
func ClusterPhaseEquals(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	actualPhase := getString(status, "phase")
	expectedPhase := getString(params, "phase")
	transition := getString(status, "transitionStatus")
	ignoreTransition := getBoolOr(params, "ignoreTransition", false)

	if expectedPhase == "" {
		return Fail("missing required param: phase")
	}

	if actualPhase != expectedPhase {
		return Fail(fmt.Sprintf("expected phase=%s", expectedPhase)).WithActual(fmt.Sprintf("phase=%s, transition=%s", actualPhase, transition))
	}

	if !ignoreTransition && transition != "" {
		return Fail(fmt.Sprintf("phase=%s but has transition status", expectedPhase)).WithActual(fmt.Sprintf("phase=%s, transition=%s", actualPhase, transition))
	}

	return Pass()
}

// ClusterPending 检查集群是否处于 pending 状态（phase=pending 且无 transitionStatus）。
func ClusterPending(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "pending" && transition == "" {
		return Pass()
	}
	return Fail("cluster not pending").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterStopped 检查集群是否已停止（phase=stopped 且无 transitionStatus）。
func ClusterStopped(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "stopped" && transition == "" {
		return Pass()
	}
	return Fail("cluster not stopped").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterDeleted 检查集群是否已删除（phase=deleted 且无 transitionStatus）。
func ClusterDeleted(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "deleted" && transition == "" {
		return Pass()
	}
	return Fail("cluster not deleted").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ClusterCeased 检查集群是否已销毁（phase=ceased 且无 transitionStatus）。
func ClusterCeased(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "ceased" && transition == "" {
		return Pass()
	}
	return Fail("cluster not ceased").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ============ Instance 断言 ============

// InstanceReady 检查实例是否就绪（phase=running 且无 transitionStatus）。
func InstanceReady(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "running" && transition == "" {
		return Pass()
	}
	return Fail("instance not ready").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceStopped 检查实例是否已停止（phase=stopped 且无 transitionStatus）。
func InstanceStopped(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "stopped" && transition == "" {
		return Pass()
	}
	return Fail("instance not stopped").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceSecurityGroupExists 检查实例安全组是否存在。
// params: id (string, 可选), expected (bool, 默认 true)
func InstanceSecurityGroupExists(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	sgs := getSlice(status, "securityGroups")
	expectedID := getString(params, "id")
	expectExists := getBoolOr(params, "expected", true)

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
			return Pass()
		}
		if !expectExists && !found {
			return Pass()
		}
		if expectExists {
			return Fail(fmt.Sprintf("security group %s not attached", expectedID))
		}
		return Fail(fmt.Sprintf("security group %s still attached", expectedID))
	}

	// 未指定 ID，检查是否有任何安全组
	hasAny := attachedCount > 0
	if expectExists && hasAny {
		return Pass()
	}
	if !expectExists && !hasAny {
		return Pass()
	}
	if expectExists {
		return Fail("no security groups attached")
	}
	return Fail(fmt.Sprintf("%d security groups still attached", attachedCount))
}

// InstanceSecurityGroupNotExists 检查实例安全组是否不存在。
func InstanceSecurityGroupNotExists(resource, params map[string]interface{}) Result {
	newParams := map[string]interface{}{
		"id":       getString(params, "id"),
		"expected": false,
	}
	return InstanceSecurityGroupExists(resource, newParams)
}

// InstancePhaseEquals 通用的 phase 检查函数。
// params: phase (string, 必填), ignoreTransition (bool, 默认 false)
func InstancePhaseEquals(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	actualPhase := getString(status, "phase")
	expectedPhase := getString(params, "phase")
	transition := getString(status, "transitionStatus")
	ignoreTransition := getBoolOr(params, "ignoreTransition", false)

	if expectedPhase == "" {
		return Fail("missing required param: phase")
	}

	if actualPhase != expectedPhase {
		return Fail(fmt.Sprintf("expected phase=%s", expectedPhase)).WithActual(fmt.Sprintf("phase=%s, transition=%s", actualPhase, transition))
	}

	if !ignoreTransition && transition != "" {
		return Fail(fmt.Sprintf("phase=%s but has transition status", expectedPhase)).WithActual(fmt.Sprintf("phase=%s, transition=%s", actualPhase, transition))
	}

	return Pass()
}

// InstancePending 检查实例是否处于 pending 状态（phase=pending 且无 transitionStatus）。
func InstancePending(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "pending" && transition == "" {
		return Pass()
	}
	return Fail("instance not pending").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceSuspended 检查实例是否已暂停（phase=suspended 且无 transitionStatus）。
func InstanceSuspended(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "suspended" && transition == "" {
		return Pass()
	}
	return Fail("instance not suspended").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceTerminated 检查实例是否已终止（phase=terminated 且无 transitionStatus）。
func InstanceTerminated(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "terminated" && transition == "" {
		return Pass()
	}
	return Fail("instance not terminated").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// InstanceCeased 检查实例是否已销毁（phase=ceased 且无 transitionStatus）。
func InstanceCeased(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	transition := getString(status, "transitionStatus")

	if phase == "ceased" && transition == "" {
		return Pass()
	}
	return Fail("instance not ceased").WithActual(fmt.Sprintf("phase=%s, transition=%s", phase, transition))
}

// ============ Kubernetes 资源就绪检查 ============

// DeploymentReady 检查 Deployment 是否就绪。
// 就绪条件：availableReplicas >= replicas 且 updatedReplicas >= replicas
func DeploymentReady(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("deployment not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	spec := getMap(resource, "spec")
	desiredReplicas := 1
	if spec != nil {
		if r := getInt(spec, "replicas"); r > 0 {
			desiredReplicas = r
		}
	}

	availableReplicas := getInt(status, "availableReplicas")
	updatedReplicas := getInt(status, "updatedReplicas")

	if availableReplicas >= desiredReplicas && updatedReplicas >= desiredReplicas {
		return Pass()
	}

	return Fail(fmt.Sprintf("deployment not ready: available=%d, updated=%d, desired=%d",
		availableReplicas, updatedReplicas, desiredReplicas)).
		WithActual(fmt.Sprintf("available=%d, updated=%d", availableReplicas, updatedReplicas))
}

// StatefulSetReady 检查 StatefulSet 是否就绪。
// 就绪条件：readyReplicas >= replicas 且 currentRevision == updateRevision
func StatefulSetReady(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("statefulset not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	spec := getMap(resource, "spec")
	desiredReplicas := 1
	if spec != nil {
		if r := getInt(spec, "replicas"); r > 0 {
			desiredReplicas = r
		}
	}

	readyReplicas := getInt(status, "readyReplicas")
	currentRevision := getString(status, "currentRevision")
	updateRevision := getString(status, "updateRevision")

	if readyReplicas >= desiredReplicas && currentRevision == updateRevision {
		return Pass()
	}

	return Fail(fmt.Sprintf("statefulset not ready: ready=%d/%d, currentRevision=%s, updateRevision=%s",
		readyReplicas, desiredReplicas, currentRevision, updateRevision)).
		WithActual(fmt.Sprintf("ready=%d, currentRev=%s, updateRev=%s", readyReplicas, currentRevision, updateRevision))
}

// DaemonSetReady 检查 DaemonSet 是否就绪。
// 就绪条件：numberReady >= desiredNumberScheduled
func DaemonSetReady(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("daemonset not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	desiredNumberScheduled := getInt(status, "desiredNumberScheduled")
	numberReady := getInt(status, "numberReady")

	if numberReady >= desiredNumberScheduled && desiredNumberScheduled > 0 {
		return Pass()
	}

	return Fail(fmt.Sprintf("daemonset not ready: ready=%d/%d", numberReady, desiredNumberScheduled)).
		WithActual(fmt.Sprintf("ready=%d, desired=%d", numberReady, desiredNumberScheduled))
}

// PodReady 检查 Pod 是否就绪。
// 就绪条件：phase=Running 且所有容器 Ready
func PodReady(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("pod not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	if phase != "Running" {
		return Fail(fmt.Sprintf("pod not running: phase=%s", phase)).WithActual(phase)
	}

	containerStatuses := getSlice(status, "containerStatuses")
	if len(containerStatuses) == 0 {
		return Fail("no container statuses")
	}

	for _, cs := range containerStatuses {
		csMap, ok := cs.(map[string]interface{})
		if !ok {
			continue
		}
		ready := getBoolOr(csMap, "ready", false)
		if !ready {
			name := getString(csMap, "name")
			return Fail(fmt.Sprintf("container %s not ready", name)).WithActual("container not ready")
		}
	}

	return Pass()
}

// PodComplete 检查 Pod 是否已完成（一次性任务）。
// 就绪条件：phase=Succeeded
func PodComplete(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("pod not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	if phase == "Succeeded" {
		return Pass()
	}

	if phase == "Failed" {
		return Fail("pod failed").WithActual(phase)
	}

	return Fail(fmt.Sprintf("pod not complete: phase=%s", phase)).WithActual(phase)
}

// JobComplete 检查 Job 是否已完成。
// 就绪条件：succeeded >= completions 或 condition=Complete
func JobComplete(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("job not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	// 检查 conditions
	conditions := getSlice(status, "conditions")
	for _, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		condType := getString(condMap, "type")
		condStatus := getString(condMap, "status")
		if condType == "Complete" && condStatus == "True" {
			return Pass()
		}
		if condType == "Failed" && condStatus == "True" {
			reason := getString(condMap, "reason")
			return Fail(fmt.Sprintf("job failed: %s", reason)).WithActual("Failed")
		}
	}

	// 检查 succeeded >= completions
	spec := getMap(resource, "spec")
	completions := 1
	if spec != nil {
		if c := getInt(spec, "completions"); c > 0 {
			completions = c
		}
	}

	succeeded := getInt(status, "succeeded")
	if succeeded >= completions {
		return Pass()
	}

	active := getInt(status, "active")
	failed := getInt(status, "failed")
	return Fail(fmt.Sprintf("job not complete: succeeded=%d/%d", succeeded, completions)).
		WithActual(fmt.Sprintf("succeeded=%d, active=%d, failed=%d", succeeded, active, failed))
}

// ServiceReady 检查 Service 是否就绪。
// 就绪条件：有对应 Endpoints 且至少有一个 ready address
// 注意：此函数检查 Service 的 spec 和 status，实际 Endpoints 需要额外查询
// params: minReadyAddresses (int, 默认 1)
func ServiceReady(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("service not found")
	}

	spec := getMap(resource, "spec")
	if spec == nil {
		return Fail("no spec")
	}

	// 对于 ExternalName 类型，不需要 endpoints
	svcType := getString(spec, "type")
	if svcType == "ExternalName" {
		externalName := getString(spec, "externalName")
		if externalName != "" {
			return Pass()
		}
		return Fail("externalName service without externalName")
	}

	// 对于其他类型，检查是否有 clusterIP（除了 Headless Service）
	clusterIP := getString(spec, "clusterIP")
	if clusterIP == "" || clusterIP == "None" {
		// Headless service，需要检查 selector
		selector := getMap(spec, "selector")
		if len(selector) == 0 {
			return Fail("headless service without selector")
		}
		// Headless service 有 selector，认为就绪
		return Pass()
	}

	// 有 ClusterIP，认为 Service 基础配置已就绪
	return Pass()
}

// PVCBound 检查 PVC 是否已绑定。
// 就绪条件：phase=Bound
func PVCBound(resource, params map[string]interface{}) Result {
	if len(resource) == 0 {
		return Fail("pvc not found")
	}

	status := getMap(resource, "status")
	if status == nil {
		return Fail("no status")
	}

	phase := getString(status, "phase")
	if phase == "Bound" {
		return Pass()
	}

	return Fail(fmt.Sprintf("pvc not bound: phase=%s", phase)).WithActual(phase)
}

// ============ 提取函数（用于 EnvInjection）============
// 提取函数返回 Result，使用 Extract() 创建结果，Value 字段存储提取值。

// ClusterNodeURL 获取指定角色节点的 IP 地址。
// params: role (string), index (int, 默认 0)
func ClusterNodeURL(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Extract("")
	}

	role := getString(params, "role")
	index := getInt(params, "index")

	// 构建 node-id -> privateIP 映射
	nodeIPMap := make(map[string]string)
	nodes := getSlice(status, "nodes")
	for _, node := range nodes {
		nodeMap, ok := node.(map[string]interface{})
		if !ok {
			continue
		}
		nodeID := getString(nodeMap, "nodeID")
		privateIP := getString(nodeMap, "privateIP")
		if nodeID != "" && privateIP != "" {
			nodeIPMap[nodeID] = privateIP
		}
	}

	// 从 displayTabs.nodeDetails 获取 node-id 和 node-role
	displayTabs := getMap(status, "displayTabs")
	if displayTabs == nil {
		return Extract("")
	}

	nodeDetails := getSlice(displayTabs, "nodeDetails")
	matchedIndex := 0

	for _, detail := range nodeDetails {
		detailMap, ok := detail.(map[string]interface{})
		if !ok {
			continue
		}

		nodeID := getString(detailMap, "node-id")
		nodeRole := getString(detailMap, "node-role")

		if nodeID == "" || nodeRole == "" {
			continue
		}

		// 如果指定了 role，则过滤
		if role != "" && !strings.EqualFold(nodeRole, role) {
			continue
		}

		privateIP := nodeIPMap[nodeID]
		if privateIP == "" {
			continue
		}

		if matchedIndex == index {
			return Extract(privateIP)
		}
		matchedIndex++
	}

	return Extract("")
}

// ClusterNodeIP 返回指定节点的私有 IP。
// params: role (string, 可选), index (int, 默认 0)
func ClusterNodeIP(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Extract("")
	}

	role := getString(params, "role")
	index := getInt(params, "index")

	nodes := getSlice(status, "nodes")
	matchedIndex := 0

	for _, node := range nodes {
		nodeMap, ok := node.(map[string]interface{})
		if !ok {
			continue
		}

		nodeRole := getString(nodeMap, "role")
		if role != "" && !strings.EqualFold(nodeRole, role) {
			continue
		}

		if matchedIndex == index {
			return Extract(getString(nodeMap, "privateIP"))
		}
		matchedIndex++
	}

	return Extract("")
}

// ClusterID 返回集群 ID。
func ClusterID(resource, params map[string]interface{}) Result {
	status := getMap(resource, "status")
	if status == nil {
		return Extract("")
	}
	return Extract(getString(status, "clusterID"))
}

// ClusterVIP 获取指定名称的 VIP。
// params: name (string)
func ClusterVIP(resource, params map[string]interface{}) Result {
	spec := getMap(resource, "spec")
	if spec == nil {
		return Extract("")
	}

	endpoints := getMap(spec, "endpoints")
	if endpoints == nil {
		return Extract("")
	}

	vips := getMap(endpoints, "reservedVIPs")
	if vips == nil {
		return Extract("")
	}

	name := getString(params, "name")
	return Extract(getString(vips, name))
}

// ClusterClientPort 返回客户端端口。
func ClusterClientPort(resource, params map[string]interface{}) Result {
	spec := getMap(resource, "spec")
	if spec == nil {
		return Extract("")
	}

	endpoints := getMap(spec, "endpoints")
	if endpoints == nil {
		return Extract("")
	}

	port := getInt(endpoints, "clientPort")
	if port == 0 {
		return Extract("")
	}
	return Extract(fmt.Sprintf("%d", port))
}

// FieldPath 通用字段路径提取器。
// params: path (string, 如 "status.phase" 或 ".data.url")
func FieldPath(resource, params map[string]interface{}) Result {
	path := getString(params, "path")
	if path == "" {
		return Extract("")
	}

	// 去掉开头的点（支持 JSONPath 风格如 ".data.url"）
	path = strings.TrimPrefix(path, ".")

	parts := strings.Split(path, ".")
	current := resource

	for _, part := range parts {
		if current == nil {
			return Extract("")
		}
		next := getMap(current, part)
		if next != nil {
			current = next
		} else {
			// 尝试获取字符串值
			if val := getString(current, part); val != "" {
				return Extract(val)
			}
			return Extract("")
		}
	}

	return Extract("")
}
