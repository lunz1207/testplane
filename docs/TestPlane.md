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
}
```

**执行模式**：
- **内置函数**：`Function` + `Params`（可选）
- **Webhook**：`Function` + `Webhook` + `Params`（可选）

**资源选择**：
断言的目标资源由上下文自动确定：
- IntegrationTest: 使用当前 Step 的资源（manifest 或 selector）
- LoadTest: 使用 Target 资源

### 条件配置

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

用于 LoadTest 的 `target.readyCondition`，等待模式：持续检查直到通过或超时。

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
- **StepCondition / ReadyCondition**：在超时时间内持续检查，直到所有 allOf 通过且（如有 anyOf）至少一个 anyOf 通过
- **HealthCheck**：按间隔周期检查，连续失败达阈值则失败

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

#### ResourceRef

单资源引用（扁平化），Manifest 和 Selector 互斥。

```go
type ResourceRef struct {
    // Manifest K8s 资源清单（与 Selector 互斥）。
    // +kubebuilder:pruning:PreserveUnknownFields
    Manifest runtime.RawExtension `json:"manifest,omitempty"`
    // Selector 资源选择器（与 Manifest 互斥）。
    Selector *ResourceSelector `json:"selector,omitempty"`
    // Action 操作类型（仅 Manifest 有效，默认 Apply）。
    // +kubebuilder:default=Apply
    Action TemplateAction `json:"action,omitempty"`
}

type TemplateAction string

const (
    TemplateActionApply  TemplateAction = "Apply"
    TemplateActionDelete TemplateAction = "Delete"
)
```

**Manifest 支持格式**：
- 单个 K8s 对象
- List 对象（`kind: List`）
- JSON 数组

#### TargetSpec

测试目标资源（用于 LoadTest）。

```go
type TargetSpec struct {
    // Resource 目标资源（单资源）。
    Resource ResourceRef `json:"resource"`
    // ReadyCondition 就绪条件（可选）。
    ReadyCondition *ReadyCondition `json:"readyCondition,omitempty"`
}
```

**Manifest vs Selector**：
- `manifest`：创建/更新资源，添加 OwnerRef，测试结束时自动清理
- `selector`：只读引用已有资源，不添加 OwnerRef，不会被清理

---

## IntegrationTest

集成测试，支持多步骤操作和断言。

### Spec 结构

```go
type IntegrationTestSpec struct {
    // Mode 测试执行模式：Sequential（顺序）或 Parallel（并行）。
    Mode IntegrationTestMode `json:"mode,omitempty"`
    // Steps 测试步骤列表。
    Steps []TestStep `json:"steps,omitempty"`
    // Repeat 重复执行配置，不设置则只执行一轮。
    Repeat *RepeatConfig `json:"repeat,omitempty"`
}
```

### TestStep 结构

```go
type TestStep struct {
    // Name 步骤名称。
    Name string `json:"name"`
    // Resource 步骤资源（单资源）。
    // Resource.Manifest 和 Resource.Selector 互斥，只能指定其中一个：
    // - Manifest：创建/更新/删除资源
    // - Selector：引用已有资源（只读）
    Resource *ResourceRef `json:"resource,omitempty"`
    // ReadyCondition 创建/更新资源后的就绪条件（步骤级）。
    ReadyCondition *StepCondition `json:"readyCondition,omitempty"`
    // Expectations 步骤执行后的业务预期。
    Expectations *StepCondition `json:"expectations,omitempty"`
    // TimeoutSeconds 步骤超时时间（秒），控制整个步骤的超时。
    TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}
```

**Manifest vs Selector**：

| 字段 | OwnerRef | 用途 |
|------|----------|------|
| `resource.manifest` | 添加 | 创建/更新/删除资源 |
| `resource.selector` | 不添加 | 只读引用，用于断言 |

**默认资源选择**：断言默认检查当前步骤的资源（manifest 或 selector 指定的资源）。

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
      timeoutSeconds: 300
      resource:
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
        allOf:
          - function: ResourceExists  # 默认检查当前步骤的资源

    - name: scale-deployment
      timeoutSeconds: 300
      resource:
        manifest:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: test-nginx
          spec:
            replicas: 3
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
        allOf:
          - function: ResourceExists

    - name: delete-deployment
      timeoutSeconds: 120
      resource:
        manifest:
          apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: test-nginx
        action: Delete
      expectations:
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
    // Target 被测目标资源。
    // 使用 Target.ReadyCondition 定义就绪条件，通过后才部署 Workload。
    Target TargetSpec `json:"target"`
    // Workload 负载资源定义。
    Workload WorkloadSpec `json:"workload"`
    // HealthCheck 运行期健康检查（周期性执行）。
    // 使用 IntervalSeconds（检查间隔）和 FailureThreshold（连续失败阈值）。
    HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
}
```

### WorkloadSpec

```go
type WorkloadSpec struct {
    // EnvInjection 环境变量注入列表（函数式）。
    EnvInjection []EnvInjection `json:"envInjection,omitempty"`
    // Resources 负载资源（多资源）。
    Resources []ResourceRef `json:"resources"`
}

type EnvInjection struct {
    // Name 环境变量名。
    Name string `json:"name"`
    // Extract 值提取器。
    Extract Extractor `json:"extract"`
}

type Extractor struct {
    // Function 提取函数名。
    Function string `json:"function"`
    // Params 函数参数。
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
    readyCondition:
      timeoutSeconds: 300
      allOf:
        - function: DeploymentAvailable

  workload:
    # 从 Target 提取值，注入到 Pod annotations
    envInjection:
      - name: TARGET_HOST
        extract:
          function: FieldPath
          params:
            path: status.loadBalancer.ingress[0].ip

    resources:
      - manifest:
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
                      # 通过 Downward API 引用注入的 annotation
                      - name: TARGET_HOST
                        valueFrom:
                          fieldRef:
                            fieldPath: metadata.annotations['testplane.io/inject-target-host']

  # 运行期健康检查（默认检查 target 资源）
  healthCheck:
    intervalSeconds: 30
    failureThreshold: 3
    allOf:
      - function: DeploymentAvailable  # 默认检查 target 资源
```

---

## 类型层次结构

```
Expectation                    ← 单个断言定义
    │
    ▼
StepCondition / ReadyCondition / HealthCheck   ← 断言条件（超时/周期模式）
    │
    ▼
ResourceSelector               ← 只读资源选择器
    │
    ▼
ResourceRef                    ← 单资源引用（Manifest | Selector）
    │
    ▼
TargetSpec / TestStep          ← 使用 ResourceRef 的高级类型
```

---

## 类型定义位置

| 类型 | 文件 | 用途 |
|------|------|------|
| `Expectation` | expectation_types.go | 单个断言 |
| `Extractor` | expectation_types.go | 值提取器（用于 EnvInjection）|
| `ExpectationResult` | expectation_types.go | 断言结果 |
| `ExpectationResultSummary` | expectation_types.go | 断言结果摘要（状态存储优化）|
| `ResourceSelector` | resource_types.go | 资源选择器 |
| `ResourceRef` | resource_types.go | 单资源引用（Manifest \| Selector）|
| `TemplateAction` | resource_types.go | 资源操作类型（Apply/Delete）|
| `ReadyConditionStatus` | status_types.go | 就绪条件状态 |
| `StepCondition` | integrationtest_types.go | IntegrationTest 步骤断言条件 |
| `ReadyCondition` | loadtest_types.go | LoadTest 就绪条件 |
| `HealthCheck` | loadtest_types.go | LoadTest 健康检查（周期模式）|
| `TargetSpec` | loadtest_types.go | 测试目标资源 |
| `WorkloadSpec` | loadtest_types.go | 负载资源定义 |

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
