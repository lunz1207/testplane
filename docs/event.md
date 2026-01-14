# 事件机制设计

## 1. 概述

TestPlane 通过 Kubernetes Event 记录 IntegrationTest 与 LoadTest 的关键节点，方便追踪进度与排障。

- IntegrationTest：关注测试生命周期与步骤执行
- LoadTest：关注目标就绪、负载部署、健康检查

---

## 2. 事件常量

### 2.1 IntegrationTest

**文件**：`internal/controller/shared/events.go`

```go
const (
    EventReasonIntegrationTestStarted   = "IntegrationTestStarted"
    EventReasonIntegrationTestSucceeded = "IntegrationTestSucceeded"
    EventReasonIntegrationTestFailed    = "IntegrationTestFailed"
    EventReasonIntegrationTestTimeout   = "IntegrationTestTimeout"

    EventReasonStepStarted   = "StepStarted"
    EventReasonStepSucceeded = "StepSucceeded"
    EventReasonStepFailed    = "StepFailed"
)
```

### 2.2 LoadTest

**文件**：`internal/controller/shared/events.go`

```go
const (
    EventReasonLoadTestStarted   = "LoadTestStarted"
    EventReasonLoadTestRunning   = "LoadTestRunning"
    EventReasonLoadTestSucceeded = "LoadTestSucceeded"
    EventReasonLoadTestFailed    = "LoadTestFailed"

    EventReasonTargetApplied      = "TargetApplied"
    EventReasonTargetReady        = "TargetReady"
    EventReasonTargetApplyFailed  = "TargetApplyFailed"
    EventReasonReadyConditionWait = "ReadyConditionWait"

    EventReasonWorkloadApplied     = "WorkloadApplied"
    EventReasonWorkloadApplyFailed = "WorkloadApplyFailed"
)
```

### 2.3 共享断言事件

**文件**：`internal/controller/shared/events.go`

```go
const (
    EventReasonExpectationPassed = "ExpectationPassed"
    EventReasonExpectationFailed = "ExpectationFailed"
)
```

---

## 3. 触发点与示例

### 3.1 IntegrationTest

| 事件 Reason | 类型 | 触发时机 | 示例消息 |
|-------------|------|----------|----------|
| `IntegrationTestStarted` | Normal | 进入 Running | "开始执行测试用例，模式: Sequential, 轮数: 3" |
| `StepStarted` | Normal | 步骤开始 | "[Round 1] 开始执行步骤 1: create-instance" |
| `StepSucceeded` | Normal | 步骤成功 | "[Round 1] 步骤 create-instance 执行成功" |
| `StepFailed` | Warning | 步骤失败 | "[Round 1] 步骤 1 执行失败: create-instance - apply failed" |
| `IntegrationTestTimeout` | Warning | 步骤或最终断言超时 | "[Round 1] 步骤 create-instance 期望检查超时" |
| `IntegrationTestFailed` | Warning | 测试失败 | "测试用例执行失败: step create-instance failed" |
| `IntegrationTestSucceeded` | Normal | 测试成功 | "测试用例执行成功" |

### 3.2 LoadTest

| 事件 Reason | 类型 | 触发时机 | 示例消息 |
|-------------|------|----------|----------|
| `LoadTestStarted` | Normal | 初始化完成 | "LoadTest started" |
| `ReadyConditionWait` | Normal | 等待目标就绪 | "Waiting for target to be ready (timeout: 5m0s)" |
| `TargetApplied` | Normal | Target apply 成功 | "Target Cluster/cluster-test applied successfully" |
| `TargetReady` | Normal | ReadyCondition 通过 | "Target is ready" |
| `WorkloadApplied` | Normal | Workload apply 成功 | "Workload Deployment/load-generator applied successfully" |
| `LoadTestRunning` | Normal | 进入 Running | "LoadTest is now running" |
| `ExpectationPassed` | Normal | 健康检查通过 | "HealthCheck passed (pass: 3, fail: 0)" |
| `ExpectationFailed` | Warning | 健康检查失败 | "HealthCheck failed (consecutive failures: 2)" |
| `LoadTestFailed` | Warning | 失败终态 | "consecutive failures reached threshold: 3" |
| `LoadTestSucceeded` | Normal | 成功终态 | "LoadTest completed successfully" |

---

## 4. 查看事件

```bash
kubectl describe integrationtest <name>
kubectl describe loadtest <name>
```

```bash
# 查看指定对象事件
kubectl get events --field-selector involvedObject.name=<name>

# 只看 Warning 事件
kubectl get events --field-selector type=Warning
```

---

## 5. 设计考量

- **关键节点优先**：只记录生命周期与断言结果，避免噪音
- **语义清晰**：Reason 与阶段一致，消息包含步骤/轮次
- **幂等友好**：Event 聚合由 Kubernetes 处理
