# TestPlane API 设计文档

## 概述

TestPlane 提供两种测试 CRD：
- **IntegrationTest**：集成测试，支持多步骤操作和断言
- **LoadTest**：负载测试，支持持续运行和周期性检查

两者共用统一的基础类型，保持设计一致性。

---

## 核心概念

### 断言系统

#### Expectation

单个断言定义，支持内置函数和 Webhook 两种模式。

```go
type Expectation struct {
    // Function 函数名（必填）
    // - 无 Webhook 时：调用内置函数
    // - 有 Webhook 时：传给 Webhook 表示执行哪个检查
    Function string `json:"function"`

    // Webhook 外部服务地址（可选）
    Webhook string `json:"webhook,omitempty"`

    // Params 函数参数（可选）
    Params runtime.RawExtension `json:"params,omitempty"`

    // Resource K8s 资源数据源（可选，仅内置函数使用）
    Resource *ExpectationResource `json:"resource,omitempty"`
}
```

**执行模式**：
- **内置函数**：`Function` + `Resource`（可选）+ `Params`（可选）
- **Webhook**：`Function` + `Webhook` + `Params`（可选）

#### ExpectationResource

指定断言的目标资源。

```go
type ExpectationResource struct {
    APIVersion string `json:"apiVersion"`
    Kind       string `json:"kind"`
    Name       string `json:"name"`
    Namespace  string `json:"namespace,omitempty"`
}
```

### 条件配置

#### WaitCondition

统一的断言条件，支持两种模式：
- **超时模式**：用于 IntegrationTest 的 readyCondition、step expectations、final expectations
- **周期模式**：用于 LoadTest 运行期断言

```go
type WaitCondition struct {
    // TimeoutSeconds 超时时间（秒），默认 300（超时模式使用）
    TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`

    // IntervalSeconds 检查间隔（秒），默认 10（周期模式使用）
    IntervalSeconds int32 `json:"intervalSeconds,omitempty"`

    // FailureThreshold 连续失败阈值，默认 3（周期模式使用）
    FailureThreshold int32 `json:"failureThreshold,omitempty"`

    // AllOf 所有期望都必须满足
    AllOf []Expectation `json:"allOf,omitempty"`

    // AnyOf 任一期望满足即可
    AnyOf []Expectation `json:"anyOf,omitempty"`
}
```

**语义**：
- **超时模式**：在超时时间内持续检查，直到所有 allOf 通过且（如有 anyOf）至少一个 anyOf 通过
- **周期模式**：按间隔周期检查，连续失败达阈值则失败

### 资源管理

#### ResourceSelector

资源选择器（只读），用于引用已有资源。

```go
type ResourceSelector struct {
    APIVersion         string            `json:"apiVersion"`
    Kind               string            `json:"kind"`
    Namespace          string            `json:"namespace,omitempty"`
    Name               string            `json:"name,omitempty"`
    LabelSelector      map[string]string `json:"labelSelector,omitempty"`
    AnnotationSelector map[string]string `json:"annotationSelector,omitempty"`
}
```

**选择方式（互斥）**：
- `Name`：按名称精确选择
- `LabelSelector`：按标签选择
- `AnnotationSelector`：按注解选择

#### ManifestAction

资源清单与操作（用于 IntegrationTest steps）。

```go
type ManifestAction struct {
    // Manifest K8s 资源清单（支持单个对象、List 或数组）
    Manifest runtime.RawExtension `json:"manifest"`

    // Action 操作类型（默认 Apply）
    Action TemplateAction `json:"action,omitempty"`
}

type TemplateAction string

const (
    TemplateActionApply  TemplateAction = "Apply"
    TemplateActionDelete TemplateAction = "Delete"
)
```

**支持格式**：
- 单个 K8s 对象
- List 对象（`kind: List`）
- JSON 数组

#### ResourceTemplate

资源模板（用于 LoadTest 和 ResourcesSpec）。

```go
type ResourceTemplate struct {
    // Name 模板名称（可选）
    Name string `json:"name,omitempty"`

    // Template 完整的 K8s 对象或 List
    Template runtime.RawExtension `json:"template,omitempty"`

    // Action 操作类型（默认 Apply）
    Action TemplateAction `json:"action,omitempty"`
}
```

#### ResourcesSpec

资源管理规格，统一 selectors 与 templates。

```go
type ResourcesSpec struct {
    // Selectors 只读引用的资源列表
    Selectors []ResourceSelector `json:"selectors,omitempty"`

    // Templates 要创建/更新/删除的资源模板
    Templates []ResourceTemplate `json:"templates,omitempty"`
}
```

#### TargetSpec

测试目标资源（用于 LoadTest）。Template 和 Selector 二选一。

```go
type TargetSpec struct {
    // Template 资源模板（创建/更新资源）
    Template *runtime.RawExtension `json:"template,omitempty"`

    // Selector 资源选择器（引用已有资源）
    Selector *ResourceSelector `json:"selector,omitempty"`

    // ReadyCondition 就绪条件
    ReadyCondition *WaitCondition `json:"readyCondition,omitempty"`
}
```

**Template vs Selector**：
- `template`：创建/更新资源，添加 OwnerRef，测试结束时自动清理
- `selector`：只读引用已有资源，不添加 OwnerRef，不会被清理

---

## IntegrationTest

集成测试，支持多步骤操作和断言。

### Spec 结构

```go
type IntegrationTestSpec struct {
    // Mode 执行模式：Sequential 或 Parallel
    Mode IntegrationTestMode `json:"mode,omitempty"`

    // Steps 测试步骤列表
    Steps []TestStep `json:"steps,omitempty"`

    // Expectations 最终期望（所有 Steps 完成后验证）
    Expectations *WaitCondition `json:"expectations,omitempty"`

    // Repeat 重复执行配置
    Repeat *RepeatConfig `json:"repeat,omitempty"`
}
```

### TestStep 结构

```go
type TestStep struct {
    // Name 步骤名称
    Name string `json:"name"`

    // Template 资源清单（创建/更新/删除资源）
    Template *ManifestAction `json:"template,omitempty"`

    // Selector 资源选择器（只读引用已有资源）
    Selector *ResourceSelector `json:"selector,omitempty"`

    // ReadyCondition 资源就绪条件（步骤级）
    ReadyCondition *WaitCondition `json:"readyCondition,omitempty"`

    // Expectations 步骤断言（默认检查当前步骤的资源）
    Expectations *WaitCondition `json:"expectations,omitempty"`

    // TimeoutSeconds 步骤超时时间（秒）
    TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}
```

**Template vs Selector**：

| 字段 | 类型 | OwnerRef | 用途 |
|------|------|----------|------|
| `template` | `*ManifestAction` | 添加 | 创建/更新/删除资源 |
| `selector` | `*ResourceSelector` | 不添加 | 只读引用，用于断言 |

**默认资源选择**：当 `Expectation.Resource` 未指定时，断言默认检查当前步骤的资源。

### RepeatConfig

```go
type RepeatConfig struct {
    // Count 重复轮数，0 表示无限
    Count int `json:"count,omitempty"`

    // MaxDurationSeconds 最大持续时间（秒），0 表示无限
    MaxDurationSeconds int `json:"maxDurationSeconds,omitempty"`

    // UntilFailure 遇到失败停止
    UntilFailure bool `json:"untilFailure,omitempty"`

    // DelayBetweenRounds 轮次间延迟（秒）
    DelayBetweenRounds int `json:"delayBetweenRounds,omitempty"`
}
```

### 执行模式

| 模式 | Apply | 收敛 | 期望检查 | 失败处理 |
|------|-------|------|----------|----------|
| Sequential | 逐步执行 | 单步等待 | 逐步进行 | 立即停止 |
| Parallel | 全部同时 | 全部等待 | 逐步检查 | 继续其他 |

### YAML 示例

```yaml
apiVersion: infra.testplane.io/v1alpha1
kind: IntegrationTest
metadata:
  name: deployment-lifecycle-test
spec:
  mode: Sequential

  steps:
    - name: create-deployment
      template:
        manifest:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: test-nginx
          spec:
            replicas: 2
            selector:
              matchLabels:
                app: test-nginx
            template:
              metadata:
                labels:
                  app: test-nginx
              spec:
                containers:
                  - name: nginx
                    image: nginx:latest
      expectations:
        timeoutSeconds: 300
        allOf:
          - function: ResourceExists  # 默认检查当前步骤的资源

    - name: scale-deployment
      template:
        manifest:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: test-nginx
          spec:
            replicas: 3
      expectations:
        timeoutSeconds: 300
        allOf:
          - function: ResourceExists

    - name: delete-deployment
      template:
        manifest:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: test-nginx
        action: Delete
      expectations:
        timeoutSeconds: 120
        allOf:
          - function: ResourceNotExists

  repeat:
    count: 3
    delayBetweenRounds: 30
```

---

## LoadTest

负载测试，支持持续运行和周期性检查。

### Spec 结构

```go
type LoadTestSpec struct {
    // Target 被测目标资源
    Target TargetSpec `json:"target"`

    // Workload 负载资源定义
    Workload WorkloadSpec `json:"workload"`

    // Expectations 运行期断言（周期性执行，默认检查 target 资源）
    Expectations *WaitCondition `json:"expectations,omitempty"`
}
```

### WorkloadSpec

```go
type WorkloadSpec struct {
    // EnvInjection 环境变量注入列表
    EnvInjection []EnvInjection `json:"envInjection,omitempty"`

    // Resources 负载资源模板
    Resources ResourcesSpec `json:"resources"`
}

type EnvInjection struct {
    // Name 环境变量名
    Name string `json:"name"`

    // Extract 值提取器
    Extract Extractor `json:"extract"`
}

type Extractor struct {
    // Function 提取函数名
    Function string `json:"function"`

    // Params 函数参数
    Params runtime.RawExtension `json:"params,omitempty"`
}
```

### YAML 示例

```yaml
apiVersion: infra.testplane.io/v1alpha1
kind: LoadTest
metadata:
  name: app-load-test
spec:
  target:
    # 方式1：使用 template 创建资源
    template:
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
    # selector:
    #   apiVersion: apps/v1
    #   kind: Deployment
    #   name: existing-app
    readyCondition:
      timeoutSeconds: 300
      allOf:
        - function: ResourceExists

  workload:
    envInjection:
      - name: TARGET_HOST
        extract:
          function: FieldPath
          params:
            path: status.loadBalancer.ingress[0].ip

    resources:
      templates:
        - template:
            apiVersion: apps/v1
            kind: Deployment
            metadata:
              name: load-generator
            spec:
              replicas: 1
              selector:
                matchLabels:
                  app: load-generator
              template:
                metadata:
                  labels:
                    app: load-generator
                spec:
                  containers:
                    - name: load
                      image: load-test:latest
                      env:
                        - name: TARGET_URL
                          value: "${TARGET_HOST}"

  # 运行期断言（默认检查 target 资源）
  expectations:
    intervalSeconds: 30
    failureThreshold: 3
    allOf:
      - function: ResourceExists  # 默认检查 target 资源
```

---

## 类型层次结构

```
Expectation                    ← 单个断言定义
    │
    ▼
WaitCondition                  ← 统一断言条件（超时模式 / 周期模式）
    │
    ▼
ResourceSelector               ← 只读资源选择器
    │
    ▼
ManifestAction / ResourceTemplate ← 资源模板
    │
    ▼
TargetSpec / TestStep          ← 简化的单资源模式（Template | Selector）
```

---

## 类型定义位置

| 类型 | 文件 | 用途 |
|------|------|------|
| `Expectation` | common_types.go | 单个断言 |
| `ExpectationResource` | common_types.go | 断言目标资源 |
| `WaitCondition` | common_types.go | 统一断言条件（超时/周期模式）|
| `ResourceSelector` | common_types.go | 资源选择器 |
| `ManifestAction` | common_types.go | 资源清单操作 |
| `ResourceTemplate` | common_types.go | 资源模板 |
| `ResourcesSpec` | common_types.go | 资源管理规格 |
| `TargetSpec` | common_types.go | 测试目标（Template \| Selector）|
| `Function` | common_types.go | 统一函数定义（断言/提取）|
| `ExpectationResult` | common_types.go | 断言结果 |
| `ReadyConditionStatus` | common_types.go | 就绪条件状态 |

---

## 资源操作规范

### Server-Side Apply (SSA)

所有资源操作统一使用 SSA：

| 场景 | ForceOwnership | 说明 |
|------|----------------|------|
| IntegrationTest | 不使用 | 避免夺取其他控制器字段 |
| LoadTest target/workload | 使用 | 确保字段归属 |

### OwnerReference 规则

| 场景 | OwnerRef | 说明 |
|------|----------|------|
| 同命名空间 | 添加 | 自动 GC |
| 跨命名空间 | 不添加 | 需手动清理 |
| selectors | 不添加 | 只读引用 |

---

## 设计原则

1. **类型分层**：ResourceSelector → ManifestAction → ResourcesSpec → TargetSpec
2. **语义明确**：Selector 只读，Template 可写
3. **统一更新**：资源操作统一使用 SSA
4. **非阻塞**：使用 Requeue 机制，不在 Reconcile 中阻塞
5. **纯函数断言**：Expectation 只读取 Snapshot，不修改资源
