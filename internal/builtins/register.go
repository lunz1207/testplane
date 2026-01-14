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

// Package builtins 提供 TestPlane 内置的期望函数实现。
// 用户可以选择注册全部、部分或不注册这些函数，也可以添加自定义函数。
package builtins

import "github.com/lunz1207/testplane/internal/plugin"

// RegisterAll 注册所有内置函数到指定的 Registry。
func RegisterAll(r *plugin.Registry) {
	RegisterCluster(r)
	RegisterInstance(r)
	RegisterK8s(r)
	RegisterCommon(r)
	RegisterExtraction(r)
}

// RegisterCluster 注册 Cluster 相关的断言函数。
func RegisterCluster(r *plugin.Registry) {
	r.Register("ClusterReady", ClusterReady)
	r.Register("ClusterHealthy", ClusterHealthy)
	r.Register("ClusterPending", ClusterPending)
	r.Register("ClusterStopped", ClusterStopped)
	r.Register("ClusterDeleted", ClusterDeleted)
	r.Register("ClusterCeased", ClusterCeased)
	r.Register("ClusterPhaseEquals", ClusterPhaseEquals)
	r.Register("ClusterNodeCount", ClusterNodeCount)
	r.Register("ClusterSecurityGroupExists", ClusterSecurityGroupExists)
	r.Register("ClusterSecurityGroupNotExists", ClusterSecurityGroupNotExists)
}

// RegisterInstance 注册 Instance 相关的断言函数。
func RegisterInstance(r *plugin.Registry) {
	r.Register("InstanceReady", InstanceReady)
	r.Register("InstanceStopped", InstanceStopped)
	r.Register("InstancePending", InstancePending)
	r.Register("InstanceSuspended", InstanceSuspended)
	r.Register("InstanceTerminated", InstanceTerminated)
	r.Register("InstanceCeased", InstanceCeased)
	r.Register("InstancePhaseEquals", InstancePhaseEquals)
	r.Register("InstanceSecurityGroupExists", InstanceSecurityGroupExists)
	r.Register("InstanceSecurityGroupNotExists", InstanceSecurityGroupNotExists)
}

// RegisterK8s 注册 Kubernetes 资源就绪检查函数。
func RegisterK8s(r *plugin.Registry) {
	r.Register("DeploymentReady", DeploymentReady)
	r.Register("StatefulSetReady", StatefulSetReady)
	r.Register("DaemonSetReady", DaemonSetReady)
	r.Register("PodReady", PodReady)
	r.Register("PodComplete", PodComplete)
	r.Register("JobComplete", JobComplete)
	r.Register("ServiceReady", ServiceReady)
	r.Register("PVCBound", PVCBound)
}

// RegisterCommon 注册通用断言函数。
func RegisterCommon(r *plugin.Registry) {
	r.Register("ResourceExists", ResourceExists)
	r.Register("ResourceNotExists", ResourceNotExists)
	r.Register("DeploymentAvailable", DeploymentAvailable)
}

// RegisterExtraction 注册提取函数（用于 EnvInjection）。
func RegisterExtraction(r *plugin.Registry) {
	r.Register("ClusterNodeURL", ClusterNodeURL)
	r.Register("ClusterNodeIP", ClusterNodeIP)
	r.Register("ClusterID", ClusterID)
	r.Register("ClusterVIP", ClusterVIP)
	r.Register("ClusterClientPort", ClusterClientPort)
	r.Register("FieldPath", FieldPath)
}
