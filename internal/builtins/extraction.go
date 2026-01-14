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

// ClusterNodeURL 获取指定角色节点的 IP 地址。
// params: role (string), index (int, 默认 0)
func ClusterNodeURL(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Extract("")
	}

	role := plugin.GetString(params, "role")
	index := plugin.GetInt(params, "index")

	// 构建 node-id -> privateIP 映射
	nodeIPMap := make(map[string]string)
	nodes := plugin.GetSlice(status, "nodes")
	for _, node := range nodes {
		nodeMap, ok := node.(map[string]interface{})
		if !ok {
			continue
		}
		nodeID := plugin.GetString(nodeMap, "nodeID")
		privateIP := plugin.GetString(nodeMap, "privateIP")
		if nodeID != "" && privateIP != "" {
			nodeIPMap[nodeID] = privateIP
		}
	}

	// 从 displayTabs.nodeDetails 获取 node-id 和 node-role
	displayTabs := plugin.GetMap(status, "displayTabs")
	if displayTabs == nil {
		return plugin.Extract("")
	}

	nodeDetails := plugin.GetSlice(displayTabs, "nodeDetails")
	matchedIndex := 0

	for _, detail := range nodeDetails {
		detailMap, ok := detail.(map[string]interface{})
		if !ok {
			continue
		}

		nodeID := plugin.GetString(detailMap, "node-id")
		nodeRole := plugin.GetString(detailMap, "node-role")

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
			return plugin.Extract(privateIP)
		}
		matchedIndex++
	}

	return plugin.Extract("")
}

// ClusterNodeIP 返回指定节点的私有 IP。
// params: role (string, 可选), index (int, 默认 0)
func ClusterNodeIP(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Extract("")
	}

	role := plugin.GetString(params, "role")
	index := plugin.GetInt(params, "index")

	nodes := plugin.GetSlice(status, "nodes")
	matchedIndex := 0

	for _, node := range nodes {
		nodeMap, ok := node.(map[string]interface{})
		if !ok {
			continue
		}

		nodeRole := plugin.GetString(nodeMap, "role")
		if role != "" && !strings.EqualFold(nodeRole, role) {
			continue
		}

		if matchedIndex == index {
			return plugin.Extract(plugin.GetString(nodeMap, "privateIP"))
		}
		matchedIndex++
	}

	return plugin.Extract("")
}

// ClusterID 返回集群 ID。
func ClusterID(resource, params map[string]interface{}) plugin.Result {
	status := plugin.GetMap(resource, "status")
	if status == nil {
		return plugin.Extract("")
	}
	return plugin.Extract(plugin.GetString(status, "clusterID"))
}

// ClusterVIP 获取指定名称的 VIP。
// params: name (string)
func ClusterVIP(resource, params map[string]interface{}) plugin.Result {
	spec := plugin.GetMap(resource, "spec")
	if spec == nil {
		return plugin.Extract("")
	}

	endpoints := plugin.GetMap(spec, "endpoints")
	if endpoints == nil {
		return plugin.Extract("")
	}

	vips := plugin.GetMap(endpoints, "reservedVIPs")
	if vips == nil {
		return plugin.Extract("")
	}

	name := plugin.GetString(params, "name")
	return plugin.Extract(plugin.GetString(vips, name))
}

// ClusterClientPort 返回客户端端口。
func ClusterClientPort(resource, params map[string]interface{}) plugin.Result {
	spec := plugin.GetMap(resource, "spec")
	if spec == nil {
		return plugin.Extract("")
	}

	endpoints := plugin.GetMap(spec, "endpoints")
	if endpoints == nil {
		return plugin.Extract("")
	}

	port := plugin.GetInt(endpoints, "clientPort")
	if port == 0 {
		return plugin.Extract("")
	}
	return plugin.Extract(fmt.Sprintf("%d", port))
}

// FieldPath 通用字段路径提取器。
// params: path (string, 如 "status.phase" 或 ".data.url")
func FieldPath(resource, params map[string]interface{}) plugin.Result {
	path := plugin.GetString(params, "path")
	if path == "" {
		return plugin.Extract("")
	}

	// 去掉开头的点（支持 JSONPath 风格如 ".data.url"）
	path = strings.TrimPrefix(path, ".")

	parts := strings.Split(path, ".")
	current := resource

	for _, part := range parts {
		if current == nil {
			return plugin.Extract("")
		}
		next := plugin.GetMap(current, part)
		if next != nil {
			current = next
		} else {
			// 尝试获取字符串值
			if val := plugin.GetString(current, part); val != "" {
				return plugin.Extract(val)
			}
			return plugin.Extract("")
		}
	}

	return plugin.Extract("")
}
