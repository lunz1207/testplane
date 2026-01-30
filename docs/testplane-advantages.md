# TestPlane 独特优势

## 概述

TestPlane 是一个 Kubernetes 原生的基础设施测试框架，通过 CRD 实现声明式测试定义，通过 Controller 实现自动化测试执行。

**核心 CRD**：

| CRD | 用途 |
|-----|------|
| **IntegrationTest** | 多步骤集成测试，支持顺序/并行执行 |
| **LoadTest** | 长时间负载测试，支持持续健康检查 |

---

## 一、声明式测试定义

### 测试即 YAML

```yaml
apiVersion: infra.testplane.io/v1alpha1
kind: IntegrationTest
metadata:
  name: cluster-lifecycle
spec:
  mode: Sequential
  steps:
    - name: create-cluster
      resource:
        manifest:
          apiVersion: infra.qingcloud.test/v1alpha1
          kind: Cluster
          spec:
            nodeCount: 3
      expect:
        allOf:
          - function: ClusterHealthy

    - name: scale-up
      resource:
        manifest:
          apiVersion: infra.qingcloud.test/v1alpha1
          kind: Cluster
          spec:
            nodeCount: 5
      expect:
        allOf:
          - function: ClusterNodeCount
            params: '{"expected": 5}'
```

**优势**：
- 无需编写代码，YAML 定义即可运行
- 开发、测试、运维都能参与用例编写
- 版本管理友好，PR Review 直观

---

## 二、双执行模式

### Sequential（顺序模式）

```
Step 1 → 验证 → Step 2 → 验证 → Step 3 → 验证 → 完成
```

- 步骤按顺序执行
- 每步完成后验证期望
- 前序失败则终止
- **适用**：有依赖关系的测试流程

### Parallel（并行模式）

```
┌─ Step 1 ─┐
├─ Step 2 ─┼─→ 全部验证 → 完成
└─ Step 3 ─┘
```

- 所有步骤同时执行
- 全部完成后统一验证
- **适用**：独立资源的并发测试

---

## 三、灵活的重复执行

```yaml
spec:
  repeat:
    count: 10                    # 固定轮次
    maxDurationSeconds: 3600     # 或时间限制
    untilFailure: true           # 或直到失败
    delayBetweenRounds: 30s      # 轮次间隔
```

**三种停止条件**：

| 条件 | 说明 | 适用场景 |
|------|------|---------|
| `count` | 执行 N 轮后停止 | 回归测试 |
| `maxDurationSeconds` | 运行 N 秒后停止 | 稳定性测试 |
| `untilFailure` | 遇到失败即停止 | 问题复现 |

**状态持久化**：每轮结果记录在 Status 中，Controller 重启后从当前轮次继续。

---

## 四、统一的期望系统

### 双模式断言

```yaml
# 方式一：内置函数（本地执行）
expect:
  allOf:
    - function: ClusterHealthy
    - function: ClusterNodeCount
      params: '{"expected": 5}'

# 方式二：Webhook（远程执行）
expect:
  allOf:
    - function: CustomCheck
      params: '{"threshold": 100}'
      webhook:
        url: "http://custom-checker/validate"
```

### 内置函数 vs Webhook

| 维度 | 内置函数 | Webhook |
|------|---------|---------|
| **执行位置** | Controller 本地 | 远程服务 |
| **性能** | 快（无网络开销） | 有网络延迟 |
| **扩展方式** | 改代码 + 重新编译 | 部署服务即可 |
| **断言范围** | K8s 资源状态 | **跨系统、深度断言** |

### Webhook 的独特优势

#### 1. 跨系统断言

内置函数只能检查 K8s 资源状态，Webhook 可以访问任意外部系统：

```yaml
# 检查数据库中的数据是否正确写入
expect:
  allOf:
    - function: VerifyDatabaseRecords
      params: '{"table": "orders", "expected_count": 1000}'
      webhook:
        url: "http://db-checker/verify"

# 检查消息队列中的消息
    - function: VerifyMessageQueue
      params: '{"queue": "events", "pattern": "order.created"}'
      webhook:
        url: "http://mq-checker/verify"

# 检查外部 API 的响应
    - function: VerifyExternalAPI
      params: '{"endpoint": "/health", "expected_status": 200}'
      webhook:
        url: "http://api-checker/verify"
```

```
┌─────────────────────────────────────────────────────────────────┐
│                        断言范围对比                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  内置函数：                                                      │
│  ┌─────────────────┐                                            │
│  │   K8s 集群       │ ← 只能检查这里                              │
│  │   • CR 状态      │                                            │
│  │   • Pod 状态     │                                            │
│  │   • Deployment   │                                            │
│  └─────────────────┘                                            │
│                                                                 │
│  Webhook：                                                      │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────┐ │
│  │   K8s 集群       │    │   数据库         │    │  消息队列    │ │
│  └────────┬────────┘    └────────┬────────┘    └──────┬──────┘ │
│           │                      │                    │        │
│           └──────────────────────┼────────────────────┘        │
│                                  ↓                             │
│                         ┌─────────────────┐                    │
│                         │  Webhook 服务    │ ← 可以检查所有系统  │
│                         └─────────────────┘                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

#### 2. 深度断言

内置函数通常检查资源状态字段，Webhook 可以执行复杂的业务逻辑验证：

```yaml
# 深度断言示例：验证集群功能是否正常
expect:
  allOf:
    # 浅层断言（内置函数）：集群状态是 Ready
    - function: ClusterReady

    # 深度断言（Webhook）：实际执行 SQL 查询验证数据库可用
    - function: VerifyDatabaseConnection
      params: '{"query": "SELECT 1", "expected": "1"}'
      webhook:
        url: "http://db-checker/execute"

    # 深度断言（Webhook）：写入测试数据并验证读取一致性
    - function: VerifyReadWriteConsistency
      params: '{"write_value": "test123"}'
      webhook:
        url: "http://consistency-checker/verify"

    # 深度断言（Webhook）：验证主从复制延迟在阈值内
    - function: VerifyReplicationLag
      params: '{"max_lag_seconds": 5}'
      webhook:
        url: "http://replication-checker/verify"
```

```
浅层断言 vs 深度断言：

浅层（内置函数）：              深度（Webhook）：
┌─────────────────┐           ┌─────────────────────────────────┐
│ status:         │           │ 实际执行业务操作验证：              │
│   phase: Ready  │ ← 只看这个 │                                 │
│   healthy: true │           │ • 连接数据库执行查询               │
└─────────────────┘           │ • 写入数据验证持久化               │
                              │ • 读取数据验证一致性               │
                              │ • 检查主从复制延迟                 │
                              │ • 验证备份是否可恢复               │
                              │ • 执行压测验证性能                 │
                              └─────────────────────────────────┘
```

#### 3. 无需重新编译

```
添加新的断言逻辑：

内置函数：
  1. 修改 Go 代码
  2. 编译新镜像
  3. 重新部署 Controller
  4. 测试验证

Webhook：
  1. 部署断言服务（任意语言）
  2. 配置 Webhook URL
  3. 立即可用
```

### 组合条件

```yaml
expect:
  allOf:                    # 全部满足
    - function: ClusterHealthy
  anyOf:                    # 至少一个满足
    - function: NodeRoleIsMaster
    - function: NodeRoleIsWorker
```

### 内置函数分类

| 分类 | 函数 |
|------|------|
| **集群** | ClusterReady, ClusterHealthy, ClusterPhaseEquals, ClusterNodeCount, ... |
| **实例** | InstanceReady, InstanceStopped, InstancePhaseEquals, ... |
| **K8s** | DeploymentReady, StatefulSetReady, PodReady, JobComplete, PVCBound, ... |
| **通用** | ResourceExists, ResourceNotExists, FieldPath（JSONPath 提取） |

---

## 五、LoadTest：Target 与 Workload 的灵活管理

> LoadTest 的核心优势：**灵活管理被测对象（Target）和测试负载（Workload）的生命周期**，并通过**环境注入**实现两者的动态关联。

### 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                         LoadTest CR                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Target（被测对象）              Workload（测试负载）            │
│   ┌─────────────────┐            ┌─────────────────┐           │
│   │ Cluster         │            │ Deployment      │           │
│   │ Instance        │  ──提取──→ │ Job             │           │
│   │ 任意 K8s 资源    │   环境变量  │ Pod             │           │
│   └─────────────────┘            └─────────────────┘           │
│          │                              │                       │
│          ↓                              ↓                       │
│   ReadyCondition                  EnvInjection                 │
│   （等待就绪）                      （动态注入）                  │
│                                                                 │
│   HealthCheck（持续健康检查）                                     │
│   └─ 同时监控 Target 和 Workload                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 5.1 Target：被测对象管理

```yaml
spec:
  target:
    manifest:
      apiVersion: infra.qingcloud.test/v1alpha1
      kind: Cluster
      metadata:
        name: test-cluster
      spec:
        nodeCount: 3

    readyCondition:               # 可选：自定义就绪条件
      function: ClusterHealthy
```

**能力**：
- 声明式定义被测资源
- 自动等待就绪（可自定义就绪条件）
- 测试结束后自动清理

### 5.2 Workload：测试负载管理

```yaml
spec:
  workload:
    manifest:
      apiVersion: apps/v1
      kind: Deployment              # 可以是 Deployment/Job/Pod 等
      metadata:
        name: load-generator
      spec:
        replicas: 10                # 灵活控制负载规模
        template:
          spec:
            containers:
              - name: client
                image: load-test:v1
```

**能力**：
- 支持任意 K8s 工作负载类型
- 灵活调整副本数、资源配置
- 与 Target 生命周期解耦

### 5.3 环境注入：动态关联

```yaml
spec:
  workload:
    envInjection:
      - name: CLUSTER_VIP           # 注入到 Workload 的环境变量名
        extractor:
          function: ClusterVIP      # 从 Target 提取 VIP
      - name: CLUSTER_PORT
        extractor:
          function: ClusterClientPort
      - name: NODE_IPS
        extractor:
          function: ClusterNodeIP
          params: '{"index": 0}'    # 支持参数

    manifest:
      apiVersion: apps/v1
      kind: Deployment
      spec:
        template:
          spec:
            containers:
              - name: client
                env:
                  - name: TARGET_URL
                    value: "http://$(CLUSTER_VIP):$(CLUSTER_PORT)"
```

**流程**：

```
1. 创建 Target
      ↓
2. 等待 Target 就绪（ReadyCondition）
      ↓
3. 从 Target 状态提取值
   • ClusterVIP → "192.168.1.100"
   • ClusterClientPort → "3306"
      ↓
4. 注入到 Workload 环境变量
      ↓
5. 启动 Workload（自动携带提取的值）
      ↓
6. 持续健康检查
```

**优势**：
- 无需硬编码地址/端口
- Target 每次创建可能产生不同的 IP/端口，自动适配
- 支持自定义提取函数

### 5.4 对比传统方式

| 维度 | 传统方式 | LoadTest |
|------|---------|----------|
| **被测对象创建** | 脚本创建，手动等待 | 声明式，自动等待就绪 |
| **获取连接信息** | 硬编码或配置文件 | 动态提取 + 自动注入 |
| **负载管理** | 外部工具（JMeter/Locust） | K8s 原生工作负载 |
| **扩缩容** | 手动调整 | 修改 replicas，kubectl apply |
| **生命周期** | 分别管理，容易遗漏 | 统一管理，自动清理 |
| **监控** | 分散的日志/指标 | 统一 HealthCheck + Events |

---

## 六、持续健康检查

```yaml
spec:
  healthCheck:
    intervalSeconds: 30           # 每 30 秒检查一次
    failureThreshold: 3           # 连续 3 次失败则停止
    expectations:
      allOf:
        - function: ClusterHealthy
        - function: DeploymentReady
```

**状态追踪**：

```yaml
status:
  healthCheckStatus:
    checkCount: 100               # 总检查次数
    passCount: 98                 # 通过次数
    failCount: 2                  # 失败次数
    consecutiveFailures: 0        # 当前连续失败
    lastCheckTime: "2024-01-01T12:00:00Z"
    lastCheckResults:
      - function: ClusterHealthy
        passed: true
```

**适用场景**：长时间稳定性测试、故障注入后的恢复验证。

---

## 七、非阻塞 Reconcile 架构

### 传统测试框架

```python
def test_cluster():
    create_cluster()
    while not is_ready():       # 阻塞轮询
        time.sleep(10)
    assert is_healthy()
```

**问题**：进程阻塞，无法并行，崩溃后状态丢失。

### TestPlane

```go
func (r *Reconciler) Reconcile(ctx context.Context, req Request) (Result, error) {
    // 检查当前状态
    if !isReady(resource) {
        return ctrl.Result{RequeueAfter: 10 * time.Second}, nil  // 非阻塞
    }
    // 继续下一步
    return r.nextStep(ctx)
}
```

**优势**：
- 单 Controller 并行处理多个测试
- 状态持久化在 CR Status 中
- Controller 重启后自动恢复

---

## 八、资源自动清理

### Kubernetes 原生机制

```yaml
apiVersion: infra.qingcloud.test/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
  finalizers:
    - infra.qingcloud.test/finalizer    # 确保清理逻辑执行
  ownerReferences:
    - apiVersion: infra.testplane.io/v1alpha1
      kind: IntegrationTest
      name: my-test                      # 父资源删除时级联清理
```

**保障**：
- **Finalizer**：删除前必须执行清理逻辑
- **OwnerReference**：父资源删除时自动级联删除子资源
- **Garbage Collection**：孤儿资源自动回收

---

## 九、完整的可观测性

### 实时状态

```bash
$ kubectl get integrationtest -w

NAME              PHASE      STEP           ROUND   AGE
cluster-test      Running    create-cluster  1/10    30s
cluster-test      Running    scale-up        1/10    5m
cluster-test      Running    create-cluster  2/10    8m
```

### 详细信息

```bash
$ kubectl describe integrationtest cluster-test

Status:
  Phase: Running
  Current Round: 2
  Completed Rounds: 1
  Steps:
    - Name: create-cluster
      State: Succeeded
      Duration: 4m30s
    - Name: scale-up
      State: Running
      Started At: 2024-01-01T12:05:00Z
```

### 事件流

```bash
$ kubectl get events --field-selector involvedObject.name=cluster-test

LAST SEEN   TYPE     REASON                MESSAGE
5m          Normal   IntegrationTestStarted  Started integration test
4m          Normal   StepStarted             [Round 1] Started step 'create-cluster'
30s         Normal   StepSucceeded           [Round 1] Step 'create-cluster' succeeded
```

### Prometheus 指标

```
controller_runtime_reconcile_total{controller="integrationtest"}
controller_runtime_reconcile_errors_total{controller="integrationtest"}
```

---

## 十、插件扩展体系

### 注册自定义函数

```go
// internal/builtins/custom.go

func init() {
    plugin.Register("MyCustomCheck", MyCustomCheck)
}

func MyCustomCheck(snapshot map[string]interface{}, params plugin.Params) plugin.Result {
    expected := params.String("threshold")
    actual := plugin.GetNestedString(snapshot, "status.metrics.value")

    if actual >= expected {
        return plugin.Pass().
            WithActual(actual).
            Result()
    }
    return plugin.Fail().
        WithActual(actual).
        WithMessage("value below threshold").
        Result()
}
```

### 使用

```yaml
expect:
  allOf:
    - function: MyCustomCheck
      params: '{"threshold": "100"}'
```

### 辅助函数

```go
// 安全的嵌套取值
plugin.GetNestedString(data, "status.nodes[0].ip")
plugin.GetNestedInt(data, "spec.replicas")
plugin.GetNestedMap(data, "status.conditions")
```

---

## 总结

| 特性 | 传统测试框架 | TestPlane |
|------|------------|-----------|
| **定义方式** | 代码 | YAML 声明 |
| **执行模式** | 阻塞轮询 | 非阻塞 Reconcile |
| **状态持久化** | 内存 | etcd（CR Status） |
| **崩溃恢复** | 丢失 | 自动继续 |
| **资源清理** | 手动 teardown | 自动（Finalizer + OwnerRef） |
| **可观测性** | 日志文件 | kubectl + Events + Prometheus |
| **扩展性** | 改代码 | 插件注册 / Webhook |
| **并行能力** | 多进程/线程 | 单 Controller 原生并行 |
| **重复执行** | 循环脚本 | 原生 Repeat + 多种停止条件 |
| **环境注入** | 硬编码/配置文件 | 动态提取 + 自动注入 |
