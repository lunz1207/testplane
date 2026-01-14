# 断言系统设计文档

## 概述

断言系统是 TestPlane 的核心验证机制，用于验证测试目标的状态是否符合预期。

**统一断言条件 WaitCondition**：
- **超时模式**：用于 IntegrationTest 的步骤断言和最终断言，通过 `timeoutSeconds` 配置
- **周期模式**：用于 LoadTest 运行期周期性断言，通过 `intervalSeconds` + `failureThreshold` 配置

---

## 系统架构

```
┌─────────────────────────────────────────────────────────────────────────────────────────┐
│                              TestPlane 断言系统架构                                       │
├─────────────────────────────────────────────────────────────────────────────────────────┤
│                                                                                         │
│  ┌─────────────────────┐                                                                │
│  │    CRD 定义层        │                                                                │
│  │ (api/v1alpha1/)     │                                                                │
│  │                     │                                                                │
│  │ • Expectation       │ ←── 单个断言定义 (Function/Webhook/Params)                     │
│  │ • WaitCondition     │ ←── 统一断言条件 (超时模式 / 周期模式)                           │
│  │ • ExpectationResult │ ←── 断言结果记录                                                │
│  └──────────┬──────────┘                                                                │
│             │                                                                           │
│             ▼                                                                           │
│  ┌─────────────────────────────────────────────────────────────────┐                    │
│  │                     Framework 层                                │                    │
│  │          (internal/controller/framework/)                       │                    │
│  │                                                                 │                    │
│  │  ┌───────────────────────┐    ┌──────────────────────────────┐ │                    │
│  │  │   ExpectationRunner   │    │      Resource Manager        │ │                    │
│  │  │                       │    │                              │ │                    │
│  │  │ • RunWaitCondition()  │    │ • GatherResourceStates()     │ │                    │
│  │  │ • runExpectation()    │    │ • WaitResourcesConverge()    │ │                    │
│  │  │ • runWebhook()        │    │ • ApplyResources()           │ │                    │
│  │  │ • runFunction()       │    │ • APIReader (绕过缓存)         │ │                    │
│  │  └───────────┬───────────┘    └──────────────┬───────────────┘ │                    │
│  │              │                               │                 │                    │
│  │              │     ┌─────────────────────────┘                 │                    │
│  │              ▼     ▼                                           │                    │
│  │  ┌───────────────────────────────────────────────────────────┐ │                    │
│  │  │                    Plugin Registry                        │ │                    │
│  │  │               (plugin/registry.go)                        │ │                    │
│  │  │                                                           │ │                    │
│  │  │  functions: map[string]Function                           │ │                    │
│  │  │                                                           │ │                    │
│  │  │  • Register(name, fn)  • Call(name, resource, params)     │ │                    │
│  │  └───────────────────────────────────────────────────────────┘ │                    │
│  └─────────────────────────────────────────────────────────────────┘                    │
│                                                                                         │
│  ┌──────────────────────────────────────────────────────────────────────────┐           │
│  │                         Controller 层                                    │           │
│  │                                                                          │           │
│  │  ┌─────────────────────────────┐  ┌──────────────────────────────────┐  │           │
│  │  │   IntegrationTest Controller │  │      LoadTest Controller        │  │           │
│  │  │                             │  │                                  │  │           │
│  │  │ • Step ReadyCondition       │  │ • Target ReadyCondition          │  │           │
│  │  │ • Step Expectations         │  │ • Periodic Expectations          │  │           │
│  │  │ • Final Expectations        │  │ • EnvInjection (Function)        │  │           │
│  │  └─────────────────────────────┘  └──────────────────────────────────┘  │           │
│  └──────────────────────────────────────────────────────────────────────────┘           │
└─────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 数据结构

### Expectation

单个断言定义，支持两种模式：
1. **内置函数**：Function + Params（可选）
2. **Webhook**：Function + Webhook + Params（可选）

```go
type Expectation struct {
    // Function 函数名（必填）
    // - 无 Webhook 时：调用内置函数
    // - 有 Webhook 时：传给 Webhook 表示执行哪个检查
    Function string `json:"function"`

    // Webhook 外部服务地址（可选）
    // 有值时调用 Webhook，无值时调用内置函数
    Webhook string `json:"webhook,omitempty"`

    // Params 函数参数（可选）
    Params runtime.RawExtension `json:"params,omitempty"`
}
```

**资源选择**：断言的目标资源由上下文自动确定：
- IntegrationTest: 使用当前 Step 的资源（manifest 或 selector）
- LoadTest: 使用 Target 资源

### 条件类型

TestPlane 使用三种不同的条件类型：

#### StepCondition

用于 IntegrationTest 步骤的 `readyCondition` 和 `expectations`。

```go
type StepCondition struct {
    // TimeoutSeconds 单次检查超时（秒）。
    // +kubebuilder:default=10
    TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
    // AllOf 所有期望都必须满足。
    AllOf []Expectation `json:"allOf,omitempty"`
    // AnyOf 任一期望满足即可。
    AnyOf []Expectation `json:"anyOf,omitempty"`
}
```

#### ReadyCondition

用于 LoadTest 的 `target.readyCondition`。

```go
type ReadyCondition struct {
    // TimeoutSeconds 总超时时间（秒）。
    // +kubebuilder:default=300
    TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
    // AllOf 所有期望都必须满足。
    AllOf []Expectation `json:"allOf,omitempty"`
    // AnyOf 任一期望满足即可。
    AnyOf []Expectation `json:"anyOf,omitempty"`
}
```

#### HealthCheck

用于 LoadTest 运行期健康检查（周期模式）。

```go
type HealthCheck struct {
    // IntervalSeconds 检查间隔（秒）。
    // +kubebuilder:default=10
    IntervalSeconds int32 `json:"intervalSeconds,omitempty"`
    // TimeoutSeconds 单次检查超时（秒）。
    // +kubebuilder:default=10
    TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
    // FailureThreshold 连续失败阈值。
    // +kubebuilder:default=3
    FailureThreshold int32 `json:"failureThreshold,omitempty"`
    // AllOf 所有期望都必须满足。
    AllOf []Expectation `json:"allOf,omitempty"`
    // AnyOf 任一期望满足即可。
    AnyOf []Expectation `json:"anyOf,omitempty"`
}
```

**语义**：
- **StepCondition / ReadyCondition**：在超时时间内持续检查，直到 allOf 全部通过且 anyOf 至少一个通过
- **HealthCheck**：按间隔周期检查，连续失败达阈值则失败

---

## 使用场景

### 场景对比

| 场景 | CRD | 字段路径 | 类型 | 主要字段 |
|------|-----|---------|------|----------|
| 步骤就绪条件 | IntegrationTest | `steps[].readyCondition` | `StepCondition` | `timeoutSeconds` |
| 步骤断言 | IntegrationTest | `steps[].expectations` | `StepCondition` | `timeoutSeconds` |
| Target 就绪条件 | LoadTest | `target.readyCondition` | `ReadyCondition` | `timeoutSeconds` |
| 健康检查 | LoadTest | `healthCheck` | `HealthCheck` | `intervalSeconds`, `failureThreshold` |

### 字段使用矩阵

| 字段 | StepCondition | ReadyCondition | HealthCheck |
|------|---------------|----------------|-------------|
| `timeoutSeconds` | ✅ | ✅ | ✅（单次检查）|
| `intervalSeconds` | - | - | ✅ |
| `failureThreshold` | - | - | ✅ |
| `allOf` | ✅ | ✅ | ✅ |
| `anyOf` | ✅ | ✅ | ✅ |

---

## 断言函数注册与实现

### 函数签名

```go
// internal/plugin/registry.go

// Function 统一函数签名（断言和提取）。
// resource: CR 完整数据（由框架获取）
// params: 用户定义的参数
// 返回 Result，业务方按需使用：
//   - 断言模式：使用 Passed、Actual、Message
//   - 提取模式：使用 Value 返回提取的值
type Function func(resource, params map[string]interface{}) Result
```

### Result 结构

```go
// internal/plugin/result.go

type Result struct {
    Passed  bool
    Value   string  // 提取模式使用
    Actual  string  // 断言模式使用
    Message string
}

// Pass 创建成功结果
func Pass() Result {
    return Result{Passed: true}
}

// Fail 创建失败结果
func Fail(msg string) Result {
    return Result{Passed: false, Message: msg}
}

// WithActual 设置实际值（用于调试）
func (r Result) WithActual(actual interface{}) Result {
    r.Actual = fmt.Sprintf("%v", actual)
    return r
}

// Extract 创建提取结果（用于 EnvInjection）
func Extract(value string) Result {
    return Result{Passed: true, Value: value}
}
```

### Registry 注册表

```go
// internal/plugin/registry.go

type Registry struct {
    functions map[string]Function
}

// NewRegistry 创建注册表。
func NewRegistry() *Registry {
    return &Registry{
        functions: make(map[string]Function),
    }
}

// Register 注册函数（用于断言和提取）。
func (r *Registry) Register(name string, fn Function) {
    r.functions[name] = fn
}

// Call 调用函数。
func (r *Registry) Call(name string, resource map[string]interface{}, paramsJSON []byte) (Result, error) {
    fn, ok := r.functions[name]
    if !ok {
        return Fail(fmt.Sprintf("unknown function: %s", name)), fmt.Errorf("unknown function: %s", name)
    }

    params, err := parseParams(paramsJSON)
    if err != nil {
        return Fail(fmt.Sprintf("invalid params: %v", err)), err
    }

    return fn(resource, params), nil
}

// Has 检查函数是否存在。
func (r *Registry) Has(name string) bool {
    _, ok := r.functions[name]
    return ok
}
```

---

## 内置断言函数

### 函数注册（internal/builtins/register.go）

```go
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
```

### 函数列表

#### 通用断言

| 函数名 | 说明 | 参数 |
|--------|------|------|
| `ResourceExists` | 资源存在 | 无 |
| `ResourceNotExists` | 资源不存在 | 无 |
| `DeploymentAvailable` | Deployment 可用副本数满足 | 无 |

#### Kubernetes 资源就绪检查

| 函数名 | 说明 | 参数 |
|--------|------|------|
| `DeploymentReady` | Deployment 就绪（available >= replicas 且 updated >= replicas） | 无 |
| `StatefulSetReady` | StatefulSet 就绪（ready >= replicas 且版本一致） | 无 |
| `DaemonSetReady` | DaemonSet 就绪（ready >= desired） | 无 |
| `PodReady` | Pod 就绪（Running 且所有容器 Ready） | 无 |
| `PodComplete` | Pod 已完成（phase=Succeeded） | 无 |
| `JobComplete` | Job 已完成（succeeded >= completions） | 无 |
| `ServiceReady` | Service 已就绪（有 ClusterIP 或 ExternalName） | 无 |
| `PVCBound` | PVC 已绑定（phase=Bound） | 无 |

#### Cluster 断言

| 函数名 | 说明 | 参数 |
|--------|------|------|
| `ClusterReady` | 集群就绪（phase=active, 无 transition） | 无 |
| `ClusterHealthy` | 集群健康（phase=active, health=healthy） | 无 |
| `ClusterPending` | 集群 pending 状态 | 无 |
| `ClusterStopped` | 集群已停止（phase=stopped） | 无 |
| `ClusterDeleted` | 集群已删除（phase=deleted） | 无 |
| `ClusterCeased` | 集群已销毁（phase=ceased） | 无 |
| `ClusterPhaseEquals` | 通用 phase 检查 | `phase: string`, `ignoreTransition: bool` |
| `ClusterNodeCount` | 集群节点数量 | `expected: int` |
| `ClusterSecurityGroupExists` | 集群安全组存在 | `id: string`（可选）, `expected: bool` |
| `ClusterSecurityGroupNotExists` | 集群安全组不存在 | `id: string`（可选） |

#### Instance 断言

| 函数名 | 说明 | 参数 |
|--------|------|------|
| `InstanceReady` | 实例就绪（phase=running） | 无 |
| `InstanceStopped` | 实例已停止（phase=stopped） | 无 |
| `InstancePending` | 实例 pending 状态 | 无 |
| `InstanceSuspended` | 实例已暂停（phase=suspended） | 无 |
| `InstanceTerminated` | 实例已终止（phase=terminated） | 无 |
| `InstanceCeased` | 实例已销毁（phase=ceased） | 无 |
| `InstancePhaseEquals` | 通用 phase 检查 | `phase: string`, `ignoreTransition: bool` |
| `InstanceSecurityGroupExists` | 实例安全组存在 | `id: string`（可选）, `expected: bool` |
| `InstanceSecurityGroupNotExists` | 实例安全组不存在 | `id: string`（可选） |

#### 提取函数（用于 EnvInjection）

| 函数名 | 说明 | 参数 |
|--------|------|------|
| `FieldPath` | 通用字段路径提取 | `path: string`（如 "status.phase"） |
| `ClusterNodeURL` | 获取指定角色节点 IP | `role: string`, `index: int`（默认 0） |
| `ClusterNodeIP` | 获取节点私有 IP | `role: string`（可选）, `index: int` |
| `ClusterID` | 获取集群 ID | 无 |
| `ClusterVIP` | 获取指定名称的 VIP | `name: string` |
| `ClusterClientPort` | 获取客户端端口 | 无 |

### 示例实现

```go
// internal/builtins/cluster.go

// ClusterHealthy 检查集群是否健康
func ClusterHealthy(resource, params map[string]interface{}) plugin.Result {
    status := plugin.GetMap(resource, "status")
    if status == nil {
        return plugin.Fail("no status")
    }

    phase := plugin.GetString(status, "phase")
    health := plugin.GetString(status, "health")
    transition := plugin.GetString(status, "transitionStatus")

    if phase == "active" && health == "healthy" && transition == "" {
        return plugin.Pass()
    }
    return plugin.Fail("cluster not healthy").
           WithActual(fmt.Sprintf("phase=%s, health=%s, transition=%s", phase, health, transition))
}

// internal/builtins/common.go

// DeploymentAvailable 检查 Deployment 是否可用
func DeploymentAvailable(resource, params map[string]interface{}) plugin.Result {
    status := plugin.GetMap(resource, "status")
    if status == nil {
        return plugin.Fail("no status")
    }

    availableReplicas := plugin.GetInt(status, "availableReplicas")
    readyReplicas := plugin.GetInt(status, "readyReplicas")

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
```

---

## 获取断言对象最新状态

### 核心原则

**关键点**：断言检查必须使用 **APIReader** 直接从 API Server 读取，绕过 controller-runtime 缓存，确保获取最新状态。

```go
// internal/controller/shared/resource/manager.go

type Manager struct {
    Client     client.Client
    Scheme     *runtime.Scheme
    FieldOwner string
    APIReader  client.Reader  // 用于断言/注入时绕过缓存读取最新状态
}
```

### 收集资源状态

```go
// GatherResourceStates 获取指定资源的当前状态
// 必须配置 APIReader，直接从 API Server 读取以确保获取最新状态
func (m *Manager) GatherResourceStates(ctx context.Context, resources []ResourceSpec, defaultNamespace string) (map[string]interface{}, error) {
    if m.APIReader == nil {
        return nil, fmt.Errorf("APIReader is required for expectation checks")
    }

    state := make(map[string]interface{})

    for _, res := range resources {
        ns := res.Namespace
        if ns == "" {
            ns = defaultNamespace
        }

        obj := &unstructured.Unstructured{}
        obj.SetAPIVersion(res.APIVersion)
        obj.SetKind(res.Kind)
        key := client.ObjectKey{Namespace: ns, Name: res.Name}

        // 直接从 API Server 读取（绕过缓存）
        err := m.APIReader.Get(ctx, key, obj)
        keyStr := fmt.Sprintf("%s/%s/%s", res.APIVersion, res.Kind, res.Name)

        if errors.IsNotFound(err) {
            return nil, fmt.Errorf("%w: %s/%s not found", ErrResourceNotReady, res.Kind, res.Name)
        }
        if err != nil {
            return nil, err
        }

        // 检查 observedGeneration：确保 Controller 已处理最新 spec
        if notReady, reason := CheckResourceNotReady(obj); notReady {
            return nil, fmt.Errorf("%w: %s/%s %s", ErrResourceNotReady, res.Kind, res.Name, reason)
        }

        state[keyStr] = obj.Object
    }

    return state, nil
}
```

### 资源收敛检查

```go
// CheckResourceNotReady 检查资源是否未就绪
// 判断标准：observedGeneration < generation
func CheckResourceNotReady(obj *unstructured.Unstructured) (bool, string) {
    gen := obj.GetGeneration()
    if gen == 0 {
        return false, ""  // 某些资源不使用 generation
    }

    observed, found, _ := unstructured.NestedInt64(obj.Object, "status", "observedGeneration")
    if !found {
        return false, ""  // 没有 observedGeneration 字段
    }

    if observed < gen {
        return true, fmt.Sprintf("observedGeneration=%d < generation=%d", observed, gen)
    }

    return false, ""
}
```

### 状态选择策略

断言会按以下策略自动选择目标资源：

```go
// SelectStateForExpectation 自动选择最适合期望使用的对象
func SelectStateForExpectation(state map[string]interface{}) map[string]interface{} {
    // 单资源：直接展开
    if len(state) == 1 {
        for _, v := range state {
            if m, ok := v.(map[string]interface{}); ok {
                return m
            }
        }
    }

    // 多资源：优先选择有 status 或 spec 的
    for _, v := range state {
        if m, ok := v.(map[string]interface{}); ok {
            if _, hasStatus := m["status"]; hasStatus {
                return m
            }
            if _, hasSpec := m["spec"]; hasSpec {
                return m
            }
        }
    }

    return state
}
```

---

## 调和流程 (Reconciliation)

### ExpectationRunner - 统一执行器

```go
// internal/controller/shared/expectation_runner.go

type ExpectationRunner struct {
    Registry   *plugin.Registry
    HTTPClient *http.Client
}

// RunWaitCondition 执行 WaitCondition 期望检查
func (runner *ExpectationRunner) RunWaitCondition(
    condition *WaitCondition,
    state map[string]interface{},
) (ExpectationResults, error) {
    var results ExpectationResults

    // 执行 allOf
    for _, exp := range condition.AllOf {
        result, err := runner.runExpectation(exp, state)
        if err != nil {
            return results, err
        }
        results.AllOf = append(results.AllOf, result)
    }

    // 执行 anyOf
    for _, exp := range condition.AnyOf {
        result, err := runner.runExpectation(exp, state)
        if err != nil {
            return results, err
        }
        results.AnyOf = append(results.AnyOf, result)
    }

    return results, nil
}

// Passed 检查期望是否满足
// 规则：allOf 全部通过 && anyOf 任一通过（如果有）
func (r ExpectationResults) Passed() bool {
    // allOf: 全部必须通过
    for _, result := range r.AllOf {
        if !result.Passed {
            return false
        }
    }

    // anyOf: 任一通过即可
    if len(r.AnyOf) > 0 {
        anyPassed := false
        for _, result := range r.AnyOf {
            if result.Passed {
                anyPassed = true
                break
            }
        }
        if !anyPassed {
            return false
        }
    }

    return true
}
```

### 单个断言执行

```go
// runExpectation 执行单个期望检查
// 支持两种模式：
// 1. 内置函数：Function + Params（可选）
// 2. Webhook：Function + Webhook + Params（可选）
// 断言的资源由调用方在 state 中提供
func (runner *ExpectationRunner) runExpectation(
    exp Expectation,
    state map[string]interface{},
) (ExpectationResult, error) {
    // 有 Webhook → 调用外部服务
    if exp.Webhook != "" {
        return runner.runWebhook(exp)
    }

    // 无 Webhook → 调用内置函数
    payload := SelectStateForExpectation(state)

    return runner.runFunction(exp, payload)
}
```

---

## IntegrationTest 断言调和流程

### 步骤断言流程图

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                    IntegrationTest Step Expectation Flow                     │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. Apply Resources (SSA)                                                    │
│         │                                                                    │
│         ▼                                                                    │
│  2. Wait Resources Converge                                                  │
│     (observedGeneration >= generation)                                       │
│         │                                                                    │
│         ▼                                                                    │
│  3. Check ReadyCondition (可选)  ◄─────────────────────────────────┐        │
│         │                                                          │         │
│         │   ┌─────────────────────────────────┐                    │         │
│         │   │ 3a. Gather State (APIReader)    │                    │         │
│         │   │     - Templates → ResourceSpecs │                    │         │
│         │   │     - Selectors → SelectorState │                    │         │
│         │   └─────────────────────────────────┘                    │         │
│         │                                                          │         │
│         │   ┌─────────────────────────────────┐                    │         │
│         │   │ 3b. Run Expectations            │                    │         │
│         │   │     - AllOf: 全部必须通过         │                   │         │
│         │   │     - AnyOf: 任一通过即可        │                    │         │
│         │   └─────────────────────────────────┘                    │         │
│         │                                                          │         │
│         │   Passed? ──No──► Timeout? ──Yes──► Step Failed          │         │
│         │      │                    │                              │         │
│         │     Yes                   No ──► Requeue (3s) ───────────┘         │
│         │      │                                                             │
│         ▼      ▼                                                             │
│  4. Check Step Expectations  ◄─────────────────────────────────┐             │
│         │                                                       │            │
│         │   ┌─────────────────────────────────┐                 │            │
│         │   │ 4a. Init ExpectationDeadline    │                 │            │
│         │   │     (从第一次检查开始计时)         │                │            │
│         │   └─────────────────────────────────┘                 │            │
│         │                                                       │            │
│         │   ┌─────────────────────────────────┐                 │            │
│         │   │ 4b. Gather State (Same as 3a)   │                 │            │
│         │   └─────────────────────────────────┘                 │            │
│         │                                                       │            │
│         │   ┌─────────────────────────────────┐                 │            │
│         │   │ 4c. Run Expectations            │                 │            │
│         │   │     ExpectationRunner           │                 │            │
│         │   │       .RunWaitCondition()       │                 │            │
│         │   └─────────────────────────────────┘                 │            │
│         │                                                       │            │
│         │   Passed? ──No──► Timeout? ──Yes──► Step Failed       │            │
│         │      │                    │                           │            │
│         │     Yes                   No ──► Requeue (3s) ────────┘            │
│         ▼                                                                    │
│  5. Step Succeeded → Next Step / Final Expectations                          │
│                                                                              │
└──────────────────────────────────────────────────────────────────────────────┘
```

### 代码实现

```go
// internal/controller/integrationtest/step_expectation.go

// checkStepExpectations 检查步骤的期望
func (r *IntegrationTestReconciler) checkStepExpectations(
    ctx context.Context,
    tc *IntegrationTest,
    status *IntegrationTestStatus,
    stepStatus *StepStatus,
    step TestStep,
    resourceSpecs []ResourceSpec,
) (ctrl.Result, error) {

    // 1. 收集资源状态
    state, waiting, err := r.buildStepState(ctx, tc, selectors, expectations, resourceSpecs)
    if err != nil {
        setStepFailed(status, stepStatus, step.Name, ReasonFailed, err.Error())
        return r.handleStepFailure(ctx, tc, status)
    }

    // 2. 等待资源就绪
    if waiting {
        if r.stepTimedOut(stepStatus) {
            setStepFailed(status, stepStatus, step.Name, ReasonTimeout, "resources not ready")
            return r.handleStepFailure(ctx, tc, status)
        }
        return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
    }

    // 3. 初始化断言截止时间
    if stepStatus.ExpectationDeadline == nil {
        dl := metav1.NewTime(time.Now().Add(expectationTimeout(step)))
        stepStatus.ExpectationDeadline = &dl
    }

    // 4. 执行期望检查
    results, err := r.runExpectations(step.Expectations, state)
    stepStatus.ExpectationResults = results.All()

    // 5. 处理结果
    if !results.Passed() {
        if r.expectationTimedOut(stepStatus, step) {
            setStepFailed(status, stepStatus, step.Name, ReasonTimeout, "expectations timeout")
            return r.handleStepFailure(ctx, tc, status)
        }
        return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
    }

    // 6. 步骤成功
    setStepSucceeded(stepStatus)
    return ctrl.Result{Requeue: true}, nil
}
```

### 最终断言

```go
// internal/controller/integrationtest/expectation.go

// checkFinalExpectations 执行最终期望检查
func (r *IntegrationTestReconciler) checkFinalExpectations(
    ctx context.Context,
    tc *IntegrationTest,
    status *IntegrationTestStatus,
) (ctrl.Result, error) {

    // 1. 初始化 FinalExpectations 状态
    if status.FinalExpectations == nil {
        now := metav1.Now()
        dl := metav1.NewTime(now.Add(expectationsTimeout(tc)))
        status.FinalExpectations = &FinalExpectationsStatus{
            State:     StateRunning,
            StartedAt: &now,
            Deadline:  &dl,
        }
    }

    // 2. 收集所有步骤的资源状态（遍历所有 Steps）
    state, err := r.gatherAllResourceStates(ctx, tc)
    if err != nil {
        if errors.Is(err, ErrResourceNotReady) {
            return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
        }
        // 失败处理...
    }

    // 3. 执行期望检查
    results, err := r.runExpectations(tc.Spec.Expectations, state)
    status.FinalExpectations.ExpectationResults = results.All()

    // 4. 处理结果
    if !results.Passed() {
        if time.Now().After(status.FinalExpectations.Deadline.Time) {
            // 超时失败
            setFinalExpectationsFailed(status, ...)
            return ctrl.Result{}, nil
        }
        return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
    }

    // 5. 最终断言成功
    status.FinalExpectations.State = StateSucceeded
    setSucceeded(status)
    return ctrl.Result{}, nil
}
```

---

## LoadTest 健康检查

### HealthCheck 执行流程

```
┌──────────────────────────────────────────────────────────────────┐
│                 LoadTest HealthCheck Flow                        │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Target ReadyCondition Passed                                 │
│         │                                                        │
│         ▼                                                        │
│  2. Workload Applied                                             │
│         │                                                        │
│         ▼                                                        │
│  3. Periodic Check Loop  ◄───────────────────────┐               │
│         │                                         │              │
│         │   ┌───────────────────────────────┐    │              │
│         │   │ Check Interval Elapsed?       │    │              │
│         │   │ (default: 10s)                │    │              │
│         │   └───────────────────────────────┘    │              │
│         │           │                            │              │
│         │          No ──► Requeue ───────────────┘              │
│         │           │                                           │
│         │          Yes                                          │
│         │           │                                           │
│         │   ┌───────────────────────────────┐                   │
│         │   │ Build State for HealthCheck   │                   │
│         │   │ (Only fetch referenced)       │                   │
│         │   └───────────────────────────────┘                   │
│         │           │                                           │
│         │   ┌───────────────────────────────┐                   │
│         │   │ Run HealthCheck               │                   │
│         │   │ ExpectationRunner             │                   │
│         │   │   .RunHealthCheck()           │                   │
│         │   └───────────────────────────────┘                   │
│         │           │                                           │
│         │         ┌─┴─┐                                         │
│         │      Passed?                                          │
│         │         │   │                                         │
│         │        Yes  No                                        │
│         │         │   │                                         │
│         │         │   ▼                                         │
│         │         │  ConsecutiveFailures++                      │
│         │         │   │                                         │
│         │         │   ▼                                         │
│         │         │  >= FailureThreshold?                       │
│         │         │   │      │                                  │
│         │         │  Yes     No                                 │
│         │         │   │      │                                  │
│         │         │   ▼      └───────────────────┐              │
│         │         │  LoadTest Failed             │              │
│         │         │                              │              │
│         ▼         ▼                              │              │
│  ConsecutiveFailures = 0                         │              │
│         │                                        │              │
│         └────────────────────────────────────────┘              │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## YAML 示例

### IntegrationTest

```yaml
apiVersion: infra.testplane.io/v1alpha1
kind: IntegrationTest
metadata:
  name: deployment-test
spec:
  mode: Sequential
  steps:
    - name: create-deployment
      timeoutSeconds: 300
      resource:
        manifest:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: nginx-test
          spec:
            replicas: 2
            selector:
              matchLabels:
                app: nginx-test
            template:
              metadata:
                labels:
                  app: nginx-test
              spec:
                containers:
                  - name: nginx
                    image: nginx:latest
      # 步骤就绪条件（可选）
      readyCondition:
        timeoutSeconds: 120
        allOf:
          - function: ResourceExists
      # 步骤断言（默认检查当前步骤的资源）
      expectations:
        allOf:
          - function: DeploymentAvailable  # 默认检查当前步骤的资源
```

### LoadTest

```yaml
apiVersion: infra.testplane.io/v1alpha1
kind: LoadTest
metadata:
  name: app-load-test
spec:
  target:
    # 方式1：使用 manifest 创建资源
    resource:
      manifest:
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: target-app
        spec:
          replicas: 3
          selector:
            matchLabels:
              app: target-app
          template:
            metadata:
              labels:
                app: target-app
            spec:
              containers:
                - name: app
                  image: myapp:latest
    # 方式2：使用 selector 引用已有资源
    # resource:
    #   selector:
    #     apiVersion: apps/v1
    #     kind: Deployment
    #     name: existing-app
    # 就绪条件（超时模式）
    readyCondition:
      timeoutSeconds: 300
      allOf:
        - function: DeploymentAvailable

  workload:
    resources:
      - manifest:
          apiVersion: apps/v1
          kind: Deployment
          # ...

  # 健康检查（周期模式，默认检查 target 资源）
  healthCheck:
    intervalSeconds: 30
    failureThreshold: 3
    allOf:
      - function: DeploymentAvailable  # 默认检查 target 资源
```

### 使用 Webhook 断言

```yaml
expectations:
  allOf:
    - function: CustomHealthCheck
      webhook: http://assertion-service.default.svc:8080/check
      params:
        endpoint: "/health"
        expectedStatus: 200
```

---

## 状态记录

### ReadyConditionStatus

```go
type ReadyConditionStatus struct {
    State      string              // Pending, Passed, Failed
    StartedAt  *metav1.Time
    Deadline   *metav1.Time
    FinishedAt *metav1.Time
    Results    []ExpectationResult
}
```

### HealthCheckStatus (LoadTest)

```go
type HealthCheckStatus struct {
    LastCheckTime       *metav1.Time
    CheckCount          int32        // 总检查次数
    PassCount           int32        // 成功次数
    FailCount           int32        // 失败次数
    ConsecutiveFailures int32        // 当前连续失败次数
    LastResults         []ExpectationResultSummary
}
```

### ExpectationResult

```go
type ExpectationResult struct {
    Expect  string                 // 期望函数名
    Params  runtime.RawExtension   // 参数
    Passed  bool                   // 是否通过
    Actual  string                 // 实际值
    Message string                 // 结果消息
}
```

---

## 扩展断言函数

### 添加自定义函数

**步骤 1**: 在 `internal/builtins/` 中创建函数文件（如 `custom.go`）：

```go
// internal/builtins/custom.go
package builtins

import (
    "fmt"
    "github.com/lunz1207/testplane/internal/plugin"
)

// MyCustomExpect 检查自定义条件
// params: expected (string, 必填), threshold (int, 可选)
func MyCustomExpect(resource, params map[string]interface{}) plugin.Result {
    // 1. 获取参数
    expected := plugin.GetString(params, "expected")
    threshold := plugin.GetInt(params, "threshold")
    if threshold == 0 {
        threshold = 10 // 默认值
    }

    // 2. 获取资源状态
    status := plugin.GetMap(resource, "status")
    if status == nil {
        return plugin.Fail("no status")
    }

    // 3. 执行检查
    actual := plugin.GetString(status, "myField")
    count := plugin.GetInt(status, "count")

    if actual == expected && count >= threshold {
        return plugin.Pass()
    }

    // 4. 返回失败结果（包含实际值用于调试）
    return plugin.Fail(fmt.Sprintf("expected %s with count >= %d", expected, threshold)).
           WithActual(fmt.Sprintf("actual=%s, count=%d", actual, count))
}
```

**步骤 2**: 在 `internal/builtins/register.go` 中注册：

```go
// RegisterCustom 注册自定义断言函数。
func RegisterCustom(r *plugin.Registry) {
    r.Register("MyCustomExpect", MyCustomExpect)
}
```

**步骤 3**: 在 YAML 中使用：

```yaml
# 在 IntegrationTest step 中使用（断言自动检查当前步骤的资源）
steps:
  - name: check-my-resource
    timeoutSeconds: 60
    resource:
      selector:
        apiVersion: example.com/v1
        kind: MyResource
        name: my-instance
    expectations:
      allOf:
        - function: MyCustomExpect
          params:
            expected: "ready"
            threshold: 5
```

---

## 错误处理

| 错误类型 | 触发条件 | 行为 |
|---------|---------|------|
| 配置错误 | Function 未注册 / Params 无效 | 立即失败 |
| 资源操作错误 | Apply / Delete 失败 | 立即失败 |
| 资源获取错误 | API Server / 权限错误 | 立即失败 |
| 资源未就绪 | observedGeneration < generation | Requeue 等待 |
| 期望未满足 | Expectation 返回 false | 继续等待（WaitCondition）/ 累加失败（ExpectationPolicy） |
| 超时 | 超过 TimeoutSeconds | 标记失败（WaitCondition） |
| 连续失败 | 超过 FailureThreshold | 标记失败（ExpectationPolicy） |

---

## 设计原则

| 原则 | 说明 |
|------|------|
| **绕过缓存** | 断言检查使用 APIReader 直接从 API Server 读取，确保获取最新状态 |
| **收敛检查** | 检查 `observedGeneration >= generation`，确保 Controller 已处理最新 spec |
| **重试机制** | 断言未通过时 Requeue (默认 3s)，直到超时 |
| **超时控制** | WaitCondition 有 TimeoutSeconds，ExpectationPolicy 有 FailureThreshold |
| **状态选择** | 自动选择目标资源（单资源展开，多资源优先有 status 的）|
| **结果记录** | ExpectationResult 记录到 Status，便于调试和监控 |
| **可扩展性** | 支持 Webhook 调用外部服务，实现自定义断言 |
| **纯函数** | 期望函数只能读取 Snapshot，不能修改资源 |
| **非阻塞** | 使用 Requeue 机制，不在 Reconcile 中阻塞等待 |
