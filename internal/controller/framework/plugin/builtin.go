package plugin

// DefaultRegistry 默认注册表。
var DefaultRegistry = NewRegistry()

func init() {
	RegisterBuiltins(DefaultRegistry)
}

// RegisterBuiltins 注册所有内置函数。
// 所有函数统一使用 Function 签名，返回 Result。
// - 断言函数：使用 Pass()/Fail() 返回结果
// - 提取函数：使用 Extract() 返回提取值
func RegisterBuiltins(r *Registry) {
	// ===== 断言函数 =====

	// Cluster
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

	// Instance
	r.Register("InstanceReady", InstanceReady)
	r.Register("InstanceStopped", InstanceStopped)
	r.Register("InstancePending", InstancePending)
	r.Register("InstanceSuspended", InstanceSuspended)
	r.Register("InstanceTerminated", InstanceTerminated)
	r.Register("InstanceCeased", InstanceCeased)
	r.Register("InstancePhaseEquals", InstancePhaseEquals)
	r.Register("InstanceSecurityGroupExists", InstanceSecurityGroupExists)
	r.Register("InstanceSecurityGroupNotExists", InstanceSecurityGroupNotExists)

	// 通用
	r.Register("ResourceExists", ResourceExists)
	r.Register("ResourceNotExists", ResourceNotExists)
	r.Register("DeploymentAvailable", DeploymentAvailable)

	// Kubernetes 资源就绪检查
	r.Register("DeploymentReady", DeploymentReady)
	r.Register("StatefulSetReady", StatefulSetReady)
	r.Register("DaemonSetReady", DaemonSetReady)
	r.Register("PodReady", PodReady)
	r.Register("PodComplete", PodComplete)
	r.Register("JobComplete", JobComplete)
	r.Register("ServiceReady", ServiceReady)
	r.Register("PVCBound", PVCBound)

	// ===== 提取函数（用于 EnvInjection）=====

	r.Register("ClusterNodeURL", ClusterNodeURL)
	r.Register("ClusterNodeIP", ClusterNodeIP)
	r.Register("ClusterID", ClusterID)
	r.Register("ClusterVIP", ClusterVIP)
	r.Register("ClusterClientPort", ClusterClientPort)
	r.Register("FieldPath", FieldPath)
}
