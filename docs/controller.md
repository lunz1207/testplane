# 控制器设计文档

## 概述

TestPlane 提供两个核心控制器：

- **IntegrationTestReconciler**：处理集成测试的生命周期
- **LoadTestReconciler**：处理负载测试的生命周期

两者都遵循 Kubernetes Operator 模式，使用 controller-runtime 框架实现。

---

## 设计原则

### 非阻塞调和

控制器严格遵循非阻塞设计：

```go
// 正确：使用 RequeueAfter
if !ready {
    return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}

// 错误：在 Reconcile 中阻塞等待
for !ready {
    time.Sleep(5 * time.Second)  // 禁止！
}
```

### 幂等操作

每次调和都假设可能从任意状态开始，所有操作都是幂等的：

- 资源应用使用 Server-Side Apply（SSA）
- 状态更新使用 Server-Side Apply（SSA）
- 通过 `Applied` 标志避免重复操作

### 状态更新策略

使用 SSA 替代 `Status().Update()`：

```go
// 推荐：使用 SSA
lt.Status.Phase = "Running"
err := framework.PatchStatusSSA(ctx, r.Client, lt, "loadtest-controller")

// 便利函数（自动选择 fieldOwner）
original := lt.DeepCopy()  // 兼容性保留，实际不使用
lt.Status.Phase = "Running"
err := framework.PatchStatusMerge(ctx, r.Client, lt, original)

// 不推荐：使用 Update（容易冲突）
lt.Status.Phase = "Running"
err := r.Status().Update(ctx, lt)
```

**SSA vs Update 对比**：

| 方面 | `Status().Update()` | SSA `Status().Patch()` |
|------|---------------------|------------------------|
| 操作方式 | 替换整个 status | 基于字段所有权更新 |
| 冲突风险 | 高（需要最新 resourceVersion） | 低（自动解决） |
| 多 controller | 互相覆盖 | 各自管理自己的字段 |
| 最佳实践 | 旧方式 | Kubernetes 推荐 |

### 绕过缓存读取

断言检查时直接从 API Server 读取最新状态：

```go
func (r *Reconciler) getAPIReader() client.Reader {
    if r.APIReader != nil {
        return r.APIReader
    }
    return r.Client
}

// 使用 APIReader 读取资源
err := r.getAPIReader().Get(ctx, key, obj)
```

---

## IntegrationTest 控制器

### 执行流程

```
┌─────────────────────────────────────────────────────────────────────┐
│                     IntegrationTest Reconcile                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. 检查终态 ──► 终态直接返回                                         │
│         │                                                            │
│         ▼                                                            │
│  2. 初始化 Running ──► 设置 startTime、初始化 steps                   │
│         │                                                            │
│         ▼                                                            │
│  3. 执行当前轮次                                                      │
│         │                                                            │
│         ├─► Sequential：逐步执行                                      │
│         │     ├─ applyResources()                                    │
│         │     ├─ waitConverge()                                      │
│         │     ├─ checkReadyCondition()                               │
│         │     └─ checkExpectations()                                 │
│         │                                                            │
│         └─► Parallel：并行执行                                        │
│               ├─ 所有步骤同时 apply                                   │
│               └─ 逐步检查各自期望                                     │
│         │                                                            │
│         ▼                                                            │
│  4. 检查最终断言（FinalExpectations）                                 │
│         │                                                            │
│         ▼                                                            │
│  5. 轮次完成处理                                                      │
│         ├─ 记录 RoundSummary                                         │
│         ├─ 检查停止条件（count/maxDuration/untilFailure）             │
│         └─ 准备下一轮或标记完成                                       │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 步骤执行

#### 四阶段执行（Sequential 模式）

```
阶段 1: Apply 资源
    ↓ stepStatus.Applied = true
阶段 2: 等待收敛
    ↓ observedGeneration >= generation
阶段 3: ReadyCondition（可选）
    ↓ allOf/anyOf 通过
阶段 4: 期望检查
    ↓ expectations 通过
步骤成功 → 进入下一步
```

#### 资源收敛判定

```go
func isConverged(obj *unstructured.Unstructured) bool {
    generation := obj.GetGeneration()
    if generation == 0 {
        return true  // 无 generation 的资源直接视为收敛
    }

    observedGen, found, _ := unstructured.NestedInt64(
        obj.Object, "status", "observedGeneration")
    if !found {
        return true  // 无 observedGeneration 直接视为收敛
    }

    return observedGen >= generation
}
```

**Action 类型**：

| Action | 收敛条件 |
|--------|---------|
| Apply | `observedGeneration >= generation` |
| Delete | 资源不存在（NotFound） |

### 超时机制

```
step.TimeoutSeconds (默认 600s)
  └─ 控制整个步骤：Apply → 收敛 → ReadyCondition → 期望检查
```

### 关键代码位置

| 功能 | 文件路径 |
|------|----------|
| 主控制器 | `internal/controller/framework/integrationtest/integrationtest_controller.go` |
| 执行逻辑 | `internal/controller/framework/integrationtest/execution.go` |
| 步骤执行 | `internal/controller/framework/integrationtest/step_runner.go` |
| 生命周期 | `internal/controller/framework/integrationtest/lifecycle.go` |
| 资源管理 | `internal/controller/framework/resource/manager.go` |

---

## LoadTest 控制器

### 执行流程

```
┌─────────────────────────────────────────────────────────────────────┐
│                       LoadTest Reconcile                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  Pending                                                             │
│    │                                                                 │
│    ▼                                                                 │
│  Initializing                                                        │
│    ├─ 1. Apply Target 资源（SSA）                                    │
│    ├─ 2. 解析 Target（name/label/annotation）                        │
│    ├─ 3. 等待 ReadyCondition                                         │
│    │     ├─ 检查 allOf/anyOf 断言                                    │
│    │     └─ 超时则失败                                               │
│    │                                                                 │
│    ▼                                                                 │
│  进入 Running                                                        │
│    ├─ 4. 解析 EnvInjection（从 Target 提取值）                       │
│    ├─ 5. Apply Workload（注入环境变量）                              │
│    │                                                                 │
│    ▼                                                                 │
│  Running（周期循环）                                                  │
│    ├─ 6. 执行 Expectations 检查                                      │
│    │     ├─ 通过：重置连续失败计数                                   │
│    │     └─ 失败：累加连续失败计数                                   │
│    │                                                                 │
│    ├─ 7. 检查失败阈值                                                │
│    │     └─ consecutiveFailures >= failureThreshold → Failed         │
│    │                                                                 │
│    └─ 8. RequeueAfter(intervalSeconds)                               │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### 值提取与 Annotation 注入

Controller 从 Target 资源提取值，并注入到 Workload Pod template 的 annotations 中。
用户通过 Kubernetes Downward API 引用这些 annotations 作为环境变量。

```yaml
workload:
  envInjection:
    - name: TARGET_HOST        # 生成 annotation: testplane.io/inject-target-host
      extract:
        function: FieldPath
        params:
          path: status.endpoint
  resources:
    - manifest:
        # ... Deployment 模板
        spec:
          template:
            spec:
              containers:
                - env:
                    - name: TARGET_HOST
                      valueFrom:
                        fieldRef:
                          fieldPath: metadata.annotations['testplane.io/inject-target-host']
```

**执行流程**：

```go
// 1. 从 Target 提取值
func (r *Reconciler) resolveEnvInjection(target, injections) (map[string]string, error) {
    values := make(map[string]string)
    for _, inj := range injections {
        result, _ := r.PluginRegistry.Call(inj.Extract.Function, target.Object, inj.Extract.Params.Raw)
        values[inj.Name] = result.Value
    }
    return values, nil
}

// 2. 注入到 Pod template annotations
func injectAnnotationsToWorkload(obj *unstructured.Unstructured, values map[string]string) error {
    // 将 TARGET_HOST → testplane.io/inject-target-host
    // 根据资源类型设置到正确的 annotation 路径
    // Deployment: spec.template.metadata.annotations
    // Pod: metadata.annotations
}
```

### 周期性断言

```go
func (r *Reconciler) runExpectationsCheck(ctx, lt) (ctrl.Result, error) {
    expectations := lt.Spec.Expectations

    // 检查间隔
    interval := time.Duration(expectations.IntervalSeconds) * time.Second
    if interval == 0 {
        interval = 10 * time.Second
    }

    // 执行断言
    results, allPassed := r.runExpectations(ctx, target, *expectations)

    // 更新状态
    status := &lt.Status.ExpectationsStatus
    status.CheckCount++
    status.LastCheckTime = &metav1.Time{Time: time.Now()}
    status.LastResults = results

    if allPassed {
        status.PassCount++
        status.ConsecutiveFailures = 0
    } else {
        status.FailCount++
        status.ConsecutiveFailures++

        // 检查阈值
        threshold := expectations.FailureThreshold
        if threshold == 0 {
            threshold = 3
        }
        if status.ConsecutiveFailures >= threshold {
            return r.setFailed(ctx, lt, "ExpectationsFailed",
                fmt.Sprintf("连续失败达阈值: %d", threshold))
        }
    }

    return ctrl.Result{RequeueAfter: interval}, nil
}
```

### 关键代码位置

| 功能 | 文件路径 |
|------|----------|
| 主控制器 | `internal/controller/framework/loadtest/loadtest_controller.go` |
| 生命周期 | `internal/controller/framework/loadtest/lifecycle.go` |
| Target 处理 | `internal/controller/framework/loadtest/target.go` |
| Workload 应用 | `internal/controller/framework/loadtest/workload.go` |
| 环境注入 | `internal/controller/framework/loadtest/injection.go` |
| 运行期断言 | `internal/controller/framework/loadtest/running.go` |

---

## 资源管理器

### Server-Side Apply

```go
func (m *Manager) ApplyResource(ctx, obj, owner) error {
    // 构建要应用的对象
    applyObj := &unstructured.Unstructured{}
    applyObj.SetAPIVersion(obj.APIVersion)
    applyObj.SetKind(obj.Kind)
    applyObj.SetName(obj.Name)
    applyObj.SetNamespace(namespace)

    // 设置 spec
    unstructured.SetNestedField(applyObj.Object, specMap, "spec")

    // 同命名空间时添加 OwnerReference
    if namespace == owner.GetNamespace() {
        controllerutil.SetOwnerReference(owner, applyObj, m.Scheme)
    }

    // SSA 应用
    return m.Client.Patch(ctx, applyObj, client.Apply,
        client.FieldOwner(m.FieldOwner))
}
```

### 资源模板展开

支持三种格式：

```go
func (m *Manager) ExpandTemplates(templates []ManifestAction) ([]ResourceSpec, error) {
    var result []ResourceSpec

    for _, t := range templates {
        // 解析 manifest
        raw := t.Manifest.Raw

        // 1. 尝试解析为单个对象
        // 2. 尝试解析为 List
        // 3. 尝试解析为数组

        result = append(result, expanded...)
    }

    return result, nil
}
```

### 等待收敛

```go
func (m *Manager) WaitForConvergence(ctx, resources) error {
    for _, res := range resources {
        obj := &unstructured.Unstructured{}
        obj.SetAPIVersion(res.APIVersion)
        obj.SetKind(res.Kind)

        err := m.Client.Get(ctx, key, obj)
        if err != nil {
            if res.Action == Delete && apierrors.IsNotFound(err) {
                continue  // 删除资源不存在视为收敛
            }
            return err
        }

        if !isConverged(obj) {
            return fmt.Errorf("资源 %s 尚未收敛", key)
        }
    }
    return nil
}
```

---

## 期望执行引擎

### 执行模式

```go
type ExpectationRunner struct {
    Registry  *plugin.Registry      // 内置函数注册表
    Client    client.Client         // API 客户端
    APIReader client.Reader         // 直接读取（绕过缓存）
}

func (r *ExpectationRunner) Run(ctx, expectations, resources) ([]ExpectationResult, bool) {
    var results []ExpectationResult
    allOfPassed := true
    anyOfPassed := false

    // 收集资源状态
    state := r.gatherResourceStates(ctx, resources)

    // 检查 allOf
    for _, exp := range expectations.AllOf {
        result := r.runSingle(ctx, exp, state)
        results = append(results, result)
        if !result.Passed {
            allOfPassed = false
        }
    }

    // 检查 anyOf
    for _, exp := range expectations.AnyOf {
        result := r.runSingle(ctx, exp, state)
        results = append(results, result)
        if result.Passed {
            anyOfPassed = true
        }
    }

    // 判定最终结果
    passed := allOfPassed && (len(expectations.AnyOf) == 0 || anyOfPassed)
    return results, passed
}
```

### 内置函数执行

```go
func (r *ExpectationRunner) runBuiltin(exp Expectation, state map[string]interface{}) ExpectationResult {
    // 1. 选择目标资源状态
    targetState := r.selectState(exp.Resource, state)

    // 2. 构建 Snapshot 和 Params
    snapshot := plugin.NewSnapshot(targetState)
    params := plugin.NewParams(exp.Params)

    // 3. 执行期望函数
    result := r.Registry.Call(exp.Function, snapshot, params)

    // 4. 转换为 ExpectationResult
    return ExpectationResult{
        Expect:  exp.Function,
        Params:  exp.Params,
        Passed:  result.Passed,
        Actual:  result.Actual,
        Message: result.Message,
    }
}
```

### Webhook 执行

```go
func (r *ExpectationRunner) runWebhook(exp Expectation, state map[string]interface{}) ExpectationResult {
    // 构建请求
    req := WebhookRequest{
        Function: exp.Function,
        Params:   exp.Params,
        State:    state,
    }

    // 发送 HTTP POST
    resp, err := http.Post(exp.Webhook, "application/json", toJSON(req))

    // 解析响应
    var result WebhookResponse
    json.NewDecoder(resp.Body).Decode(&result)

    return ExpectationResult{
        Expect:  exp.Function,
        Passed:  result.Passed,
        Message: result.Message,
    }
}
```

---

## 错误处理

### 错误类型与行为

| 错误类型 | 触发条件 | 行为 |
|---------|---------|------|
| 配置错误 | Function 未注册 / Params 无效 | 立即失败 |
| 资源操作错误 | Apply/Delete 失败 | 立即失败 |
| API 错误 | API Server 不可用 / 权限不足 | 重试（RequeueAfter） |
| 收敛超时 | 超过步骤 TimeoutSeconds | 步骤失败 |
| 断言未满足 | Expectation 返回 false | 继续等待 |
| 断言超时 | 超过 expectations.TimeoutSeconds | 步骤/测试失败 |
| 连续失败 | 超过 FailureThreshold | LoadTest 失败 |

### 重试策略

```go
// 临时错误：指数退避重试
if isTransientError(err) {
    return ctrl.Result{RequeueAfter: calculateBackoff(attempt)}, nil
}

// 永久错误：标记失败
return r.setFailed(ctx, obj, reason, err.Error())
```

---

## 设计约束

### 禁止的模式

| 模式 | 问题 | 替代方案 |
|------|------|----------|
| `time.Sleep` | 阻塞调和循环 | `RequeueAfter` |
| `for` 轮询 | 占用 goroutine | 多次 Reconcile |
| 直接修改 spec | 违反声明式原则 | 只修改 status |
| 跳过错误检查 | 状态不一致 | 完整错误处理 |

### 推荐实践

| 实践 | 说明 |
|------|------|
| 使用 SSA | 统一创建/更新逻辑 |
| 绕过缓存读取 | 获取最新状态 |
| 幂等操作 | 支持重试和恢复 |
| 记录事件 | 便于调试和审计 |
| 更新 Conditions | 标准化状态表达 |
