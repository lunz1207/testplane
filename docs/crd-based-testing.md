# 基于 CRD 的自动化测试：工具 vs 服务

## 概述

本文对比两种自动化测试方式：

| 方式 | 定位 | 典型技术栈 |
|------|------|--------|
| **传统 API 测试** | 脚本/工具驱动的测试 | Python + Pytest |
| **CRD 测试** | 服务化的声明式测试 | Go + Kubebuilder |

**架构对比**：

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  传统方式：                                                      │
│  测试工具 ──HTTP API──→ 被测系统                                  │
│                                                                 │
│  CRD 方式：                                                      │
│  TestPlane ──→ 资源 Operator ──HTTP API──→ 被测系统              │
│  (测试框架)    (CRD 封装层)                                       │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```


---

## 一、核心差异：工具 vs 服务

> **传统 API 测试是工具，CRD 测试是服务。**
> **工具需要人来使用，服务可以自己运行。**

```
┌─────────────────────────────────────────────────────────────────┐
│                      工具 vs 服务                                │
├───────────────────────────┬─────────────────────────────────────┤
│   传统 API 测试（工具）     │      CRD 测试（服务）               │
├───────────────────────────┼─────────────────────────────────────┤
│                           │                                     │
│   ┌─────────┐             │      ┌─────────┐                   │
│   │  使用者  │             │      │  使用者  │                   │
│   └────┬────┘             │      └────┬────┘                   │
│        │                  │           │                        │
│        │ 执行             │           │ 提交请求                │
│        │ 等待             │           │ (kubectl apply)        │
│        │ 监控             │           │                        │
│        │ 清理             │           │ 然后可以离开            │
│        ↓                  │           ↓                        │
│   ┌─────────┐             │      ┌─────────┐                   │
│   │ 测试脚本 │             │      │ 测试服务 │ ←── 7×24 运行     │
│   │         │             │      │         │                   │
│   │ 人走即停 │             │      │ • 自动执行│                   │
│   │ 人管资源 │             │      │ • 自动清理│                   │
│   └─────────┘             │      │ • 自动恢复│                   │
│        ↓                  │      │ • 自动报告│                   │
│   人工收集报告             │      └─────────┘                   │
│                           │           ↓                        │
│                           │      结果持久化，随时可查            │
│                           │                                     │
└───────────────────────────┴─────────────────────────────────────┘
```

### 本质区别

| 维度 | 工具（传统 API 测试） | 服务（CRD 测试） |
|------|---------------------|-----------------|
| **运行模式** | 人启动 → 人等待 → 人结束 | 提交即运行，自主完成 |
| **人的参与** | 全程依赖 | 仅提交请求 |
| **运行时长** | 人在才运行 | 7×24 自主运行 |
| **故障恢复** | 人工重跑 | 自动恢复继续 |
| **结果获取** | 人工收集 | 随时可查 |
| **资源管理** | 人工清理 | 自动清理 |
| **使用门槛** | 需要会用工具 | 只需会提交请求 |

---

## 二、核心场景一：协作

> **场景定义**：开发修改代码后，需要用测试团队提供的测试用例验证是否破坏功能。

### 工具模式：测试是执行者

```
┌─────────────────────────────────────────────────────────────────┐
│                    工具模式下的协作流程                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  开发                        测试                               │
│    │                          │                                │
│    │  1. 修改代码              │                                │
│    │     部署到测试环境         │                                │
│    │                          │                                │
│    │  2. "帮我跑下回归测试" ──→ │                                │
│    │                          │                                │
│    │      等待...              │  3. 配置环境                    │
│    │      等待...              │     - API 地址                  │
│    │      等待...              │     - 认证密钥                  │
│    │      等待...              │     - Zone 配置                 │
│    │                          │                                │
│    │      等待...              │  4. 执行测试                    │
│    │      等待...              │     $ test-run -e dev -m P0        │
│    │      等待...              │                                │
│    │      等待...              │     等待 30-60 分钟...          │
│    │                          │                                │
│    │  ←─────────────────────── │  5. 发送报告                    │
│    │  "测试通过/失败，见附件"    │                                │
│    │                          │                                │
│    ↓                          │                                │
│  6. 查看报告                   │                                │
│     分析失败原因               │                                │
│     (如需调试，回到步骤 1)      │                                │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

问题：
• 开发依赖测试团队，需要排队
• 测试成为瓶颈，重复做配置和执行工作
• 反馈周期长，可能需要数小时
• 开发无法自助排查问题
```

### 服务模式：测试是服务提供者

```
┌─────────────────────────────────────────────────────────────────┐
│                    服务模式下的协作流程                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  测试（服务维护者）                                               │
│    │                                                            │
│    │  维护测试服务：                                              │
│    │  • 测试用例 CR（YAML）                                       │
│    │  • 期望函数（断言逻辑）                                       │
│    │  • TestPlane Controller                                    │
│    │                                                            │
│    ↓                                                            │
│  ┌────────────────────────────────────────┐                     │
│  │         测试服务（K8s 集群内）           │                     │
│  │                                        │                     │
│  │  tests/regression/                     │                     │
│  │  ├── cluster-lifecycle.yaml           │                     │
│  │  ├── cluster-scale.yaml               │                     │
│  │  ├── instance-lifecycle.yaml          │                     │
│  │  └── ...                              │                     │
│  └────────────────────────────────────────┘                     │
│                       ↑                                         │
│                       │                                         │
│  开发（服务使用者）     │ 自助使用                                 │
│    │                  │                                         │
│    │  1. 修改代码      │                                         │
│    │     部署到测试环境 │                                         │
│    │                  │                                         │
│    │  2. 自助运行测试 ─┘                                         │
│    │     $ kubectl apply -f tests/regression/cluster-scale.yaml │
│    │                                                            │
│    │  3. 继续做其他事（无需等待）                                  │
│    │                                                            │
│    │  4. 随时查看进度                                            │
│    │     $ kubectl get integrationtest -w                       │
│    │                                                            │
│    │  5. 自助查看结果                                            │
│    │     $ kubectl describe integrationtest                     │
│    │                                                            │
│    ↓                                                            │
│  6. 测试通过，提交 PR                                            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

优势：
• 开发自助，无需等待测试团队
• 测试专注维护服务质量，而非重复执行
• 反馈即时，kubectl get -w 实时可见
• 开发可自助排查，kubectl describe
```

### 协作模式对比

| 维度 | 工具模式 | 服务模式 |
|------|---------|---------|
| **测试角色** | 执行者（被动响应请求） | 服务维护者（主动建设） |
| **开发角色** | 请求者（等待他人） | 服务使用者（自助） |
| **协作方式** | 人对人（请求→执行→反馈） | 人对服务（提交→查询） |
| **瓶颈** | 测试团队人力 | 无（服务可并行） |
| **反馈速度** | 依赖测试排期 | 即时 |
| **自助能力** | 无 | 完全自助 |

### 开发参与用例开发

> **痛点**：开发想参与测试用例开发，或者修改测试数据，在传统模式下非常困难。

#### 工具模式：代码壁垒

```
开发想参与用例开发：

  ┌─────────────────────────────────────────────────────────┐
  │  api-autotest 代码结构                             │
  │                                                         │
  │  testcases/                                             │
  │  ├── resources/                                              │
  │  │   └── compute/                                       │
  │  │       └── test_cluster.py   ← Python + Pytest        │
  │  │                                                      │
  │  apis/                                                  │
  │  ├── resources/                                              │
  │  │   └── compute/                                       │
  │  │       └── cluster.py        ← API 封装层              │
  │  │                                                      │
  │  data/                                                  │
  │  └── test_data/                                         │
  │      └── cluster_data.yaml     ← 测试数据               │
  └─────────────────────────────────────────────────────────┘

  开发需要：
  1. 克隆测试仓库
  2. 理解项目结构（testcases/apis/data 分离）
  3. 学习 Python + Pytest
  4. 学习 test-framework 框架（装饰器、异步处理）
  5. 理解 API 封装层
  6. 配置本地环境运行测试

  结果：门槛过高，开发放弃参与
```

```
开发想修改测试数据（如：改节点数从 3 改成 5）：

  工具模式：

  1. 找到数据文件
     $ vim data/test_data/cluster_data.yaml

  2. 或者找到代码中的硬编码
     $ vim testcases/resources/compute/test_cluster.py
     # 修改 node_count = 5

  3. 需要懂 Python 才能确保不破坏语法

  4. 提交代码变更
     $ git add . && git commit && git push

  5. 等待 CI 或请求测试团队重新执行

  问题：
  • 改个参数需要改代码
  • 需要懂 Python/Pytest
  • 改动影响所有人
  • 无法快速验证
```

#### 服务模式：YAML 即用例

```
开发想参与用例开发：

  ┌─────────────────────────────────────────────────────────┐
  │  测试用例 = YAML 文件                                     │
  │                                                         │
  │  apiVersion: infra.testplane.io/v1alpha1                │
  │  kind: IntegrationTest                                  │
  │  metadata:                                              │
  │    name: cluster-scale-test                             │
  │  spec:                                                  │
  │    steps:                                               │
  │      - name: create-cluster                             │
  │        resource:                                        │
  │          manifest:                                      │
  │            apiVersion: example.io/v1alpha1    │
  │            kind: Cluster                                │
  │            spec:                                        │
  │              nodeCount: 3           ← 测试数据直接可见   │
  │        expect:                                          │
  │          - ClusterHealthy           ← 断言直观易懂       │
  │                                                         │
  │      - name: scale-up                                   │
  │        ...                                              │
  └─────────────────────────────────────────────────────────┘

  开发只需要：
  1. 会写 K8s YAML（已有技能）
  2. 理解业务场景

  结果：开发可以直接贡献用例
```

```
开发想修改测试数据（如：改节点数从 3 改成 5）：

  服务模式：

  方式一：临时修改，仅本次生效

  $ kubectl get integrationtest cluster-test -o yaml > my-test.yaml
  $ vim my-test.yaml                    # 改 nodeCount: 5
  $ kubectl apply -f my-test.yaml       # 立即运行

  方式二：创建自己的变体测试

  # my-5-node-test.yaml
  apiVersion: infra.testplane.io/v1alpha1
  kind: IntegrationTest
  metadata:
    name: cluster-test-5node           # 不同名字，不影响他人
  spec:
    steps:
      - name: create-cluster
        resource:
          manifest:
            spec:
              nodeCount: 5              # 我想要的值

  $ kubectl apply -f my-5-node-test.yaml

  优势：
  • 改参数只改 YAML，无需懂代码
  • 立即生效，无需提交代码
  • 创建变体测试，不影响他人
  • 秒级验证
```

**对比**：

| 维度 | 工具模式 | 服务模式 |
|------|---------|---------|
| **用例格式** | Python 代码 | YAML 声明 |
| **开发参与门槛** | 学 Python + 框架 | 会写 K8s YAML |
| **修改测试数据** | 改代码 + 提交 | 改 YAML + apply |
| **生效时间** | 等 CI / 等测试团队 | 即时 |
| **影响范围** | 改动影响所有人 | 可创建个人变体 |
| **技能复用** | 需学新技能 | 复用 K8s 技能 |

### 测试团队角色转变

```
工具模式：                          服务模式：

测试团队 = 执行者                    测试团队 = 服务提供者
• 接收测试请求                       • 设计测试用例（CR）
• 配置测试环境                       • 开发期望函数（断言）
• 执行测试脚本                       • 维护测试服务
• 分析测试结果                       • 优化测试覆盖
• 发送测试报告                       • 赋能开发自助
     ↓                                   ↓
工作量 = 请求数 × 单次执行时间        工作量 = 服务建设（一次性）
                                         + 维护优化（持续）

结果：                               结果：
• 测试成为瓶颈                       • 测试价值放大
• 重复劳动                           • 开发效率提升
• 疲于应付                           • 专注质量建设
```

---

## 三、核心场景二：交付验证

> **场景定义**：客户环境部署新系统后，用自动化测试用例验证功能。

### 工具模式：带着工具去现场

```
┌─────────────────────────────────────────────────────────────────┐
│                    工具模式下的交付验证                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                     客户现场（K8s 集群）                   │   │
│  │                                                         │   │
│  │   云平台（新部署的被测系统）                              │   │
│  │                                                         │   │
│  │         ↑ 网络隔离                                       │   │
│  └─────────│───────────────────────────────────────────────┘   │
│            │                                                    │
│            │ 需要：VPN / 防火墙配置 / 客户凭证                    │
│            │                                                    │
│  ┌─────────│───────────────────────────────────────────────┐   │
│  │         ↓              交付工程师本地                      │   │
│  │                                                         │   │
│  │   1. 克隆 api-autotest                             │   │
│  │                                                         │   │
│  │   2. 配置客户环境                                         │   │
│  │      customer-env:                                       │   │
│  │        host: "https://api.customer-env.example.com"                  │   │
│  │        access_key_id: "ACCESS_KEY"      ← 明文凭证         │   │
│  │        secret_access_key: "SECRET_KEY"                    │   │
│  │                                                         │   │
│  │   3. 配置 VPN 连接客户网络                                 │   │
│  │                                                         │   │
│  │   4. 执行测试                                            │   │
│  │      $ test-run -e customer-env -m P0 P1                    │   │
│  │                                                         │   │
│  │   5. 等待数小时...                                        │   │
│  │                                                         │   │
│  │   6. 导出 Allure 报告，发送给客户                          │   │
│  │                                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  工程师离场后：                                                  │
│  • 客户无法自主重新验证                                          │
│  • 问题无法复现                                                  │
│  • 报告是静态文件，与系统状态分离                                  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

问题：
• 需要外部网络访问（VPN/防火墙）
• 凭证明文存储，安全风险
• 工程师必须在场
• 客户无法自主验收
• 离场后无法复现
```

### 服务模式：服务随产品交付

```
┌─────────────────────────────────────────────────────────────────┐
│                    服务模式下的交付验证                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                     客户 K8s 集群                         │   │
│  │                                                         │   │
│  │   ┌─────────────┐    ┌─────────────┐    ┌───────────┐   │   │
│  │   │ TestPlane   │    │ resource-  │    │ 测试用例   │   │   │
│  │   │ Controller  │    │ operator    │    │ CR        │   │   │
│  │   │             │    │ (CRD封装层) │    │           │   │   │
│  │   └──────┬──────┘    └──────┬──────┘    └─────┬─────┘   │   │
│  │          │                  │                 │         │   │
│  │          │    集群内通信     │                 │         │   │
│  │          │ ←───────────────→│ ←──────────────→│         │   │
│  │          │   无需外部网络    │                 │         │   │
│  │          │                  │                 │         │   │
│  │          ↓                  ↓                 ↓         │   │
│  │   ┌─────────────────────────────────────────────────┐   │   │
│  │   │              测试结果（Status）                   │   │   │
│  │   │  • 进度实时可见：kubectl get integrationtest -w  │   │   │
│  │   │  • 结果持久化：随时可查                           │   │   │
│  │   │  • 客户自主：无需外部依赖                         │   │   │
│  │   └─────────────────────────────────────────────────┘   │   │
│  │                                                         │   │
│  └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│  交付流程：                                                      │
│  1. Helm 安装测试服务（随产品一起）                                │
│     $ helm install acceptance-test ./charts/acceptance          │
│                                                                 │
│  2. 客户自主触发验收                                              │
│     $ kubectl apply -f acceptance-tests/                        │
│                                                                 │
│  3. 客户自主查看结果                                              │
│     $ kubectl get integrationtest -w                            │
│     $ kubectl describe integrationtest                          │
│                                                                 │
│  工程师离场后：                                                   │
│  • 客户可随时重新验证                                             │
│  • 问题可复现（重新 apply 同一 CR）                                │
│  • 结果持久化在集群中                                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

优势：
• 无需外部网络访问
• 使用 K8s ServiceAccount，无明文凭证
• 客户完全自主
• 随时可复现
• 测试服务成为产品的一部分
```

### 交付验证对比

| 维度 | 工具模式 | 服务模式 |
|------|---------|---------|
| **网络需求** | 外部访问 + VPN | 集群内通信 |
| **凭证管理** | 明文 AK/SK | K8s ServiceAccount |
| **执行者** | 交付工程师 | 客户自主 |
| **可复现性** | 离场后无法复现 | 随时重新 apply |
| **结果存储** | 本地报告文件 | K8s etcd 持久化 |
| **交付物** | 产品 + 报告文件 | 产品 + 测试服务 |
| **长期价值** | 一次性验收 | 持续可用 |

### 客户视角的变化

```
工具模式（客户被动）：              服务模式（客户主动）：

"工程师来验收了"                    "我们自己验收"
      ↓                                 ↓
等待工程师配置                      kubectl apply -f tests/
      ↓                                 ↓
等待测试执行                        kubectl get integrationtest -w
      ↓                                 ↓
收到一份报告                        kubectl describe integrationtest
      ↓                                 ↓
工程师离场                          测试服务留在集群中
      ↓                                 ↓
有问题要再请工程师                   有问题自己重新验证
```

---

## 四、实现层面的对比

> 传统 API 测试的四大技术痛点：
> 1. 异步轮询
> 2. 配置复杂（环境配置、测试数据配置）
> 3. 可观测性
> 4. 资源清理

---

### 4.1 异步轮询

> **痛点**：云资源操作是异步的，需要轮询等待操作完成。

#### 工具模式：阻塞轮询

```python
# 传统方式：装饰器 + 阻塞轮询
# 文件：api-autotest/apis/resources/compute/instance.py

@test-framework.async_api(api_handler, "job_id", condition=cond)
def create_cluster(self, ...):
    """
    内部流程：
    1. 调用 API 创建集群
    2. 从响应中提取 job_id
    3. 循环轮询 job 状态（阻塞）
    4. 直到 job 完成或超时
    """
    resp = self.send_http_by_sign("get", params)
    return resp

# 问题：
# - 每个异步操作都需要配置轮询逻辑
# - 测试进程被阻塞，无法并行
# - 超时、重试策略分散在各处
# - 进程崩溃时，轮询中断，状态丢失
```

```
执行时间线（工具模式）：

测试进程
   │
   ├── create_cluster() 调用 API
   │         ↓
   │    轮询 job 状态...  ← 阻塞等待
   │    轮询 job 状态...
   │    轮询 job 状态...
   │    （5-20 分钟）
   │         ↓
   │    job 完成
   │         ↓
   ├── 继续下一步
   │
   └── 测试结束

问题：整个进程被阻塞，资源浪费
```

#### 服务模式：非阻塞 Reconcile

```go
// CRD 方式：Reconcile 循环，非阻塞
// 文件：internal/controller/integrationtest/step_runner.go

func (r *Reconciler) Reconcile(ctx context.Context, req Request) (Result, error) {
    // 1. 检查当前状态
    if step.State == "" {
        // 首次执行：应用资源
        r.applyResource(ctx, manifest)
        return ctrl.Result{RequeueAfter: 5*time.Second}, nil  // 立即返回
    }

    // 2. 检查资源是否就绪
    if !isReady(resource) {
        return ctrl.Result{RequeueAfter: 5*time.Second}, nil  // 继续等待
    }

    // 3. 资源就绪，继续下一步
    return r.nextStep(ctx)
}
```

```
执行时间线（服务模式）：

Controller
   │
   ├── Reconcile #1: 应用资源，返回（耗时 < 1秒）
   │         ↓
   │    （Controller 可以处理其他测试）
   │         ↓
   ├── Reconcile #2: 检查状态，未就绪，返回
   │         ↓
   │    （Controller 可以处理其他测试）
   │         ↓
   ├── Reconcile #N: 检查状态，已就绪，继续
   │
   └── 测试完成

优势：非阻塞，可并行处理多个测试
```

**对比**：

| 维度 | 工具模式 | 服务模式 |
|------|---------|---------|
| 等待方式 | 阻塞轮询 | 非阻塞 Requeue |
| 并行能力 | 需要多进程/线程 | 单 Controller 并行处理 |
| 资源占用 | 进程持续占用 | 按需调度 |
| 崩溃恢复 | 轮询中断，状态丢失 | 从 Status 恢复继续 |
| 超时处理 | 每处单独配置 | 统一 Deadline 机制 |

---

### 4.2 配置复杂

> **痛点**：环境配置（API 地址、凭证、Zone）和测试数据配置繁琐。

#### 工具模式：多层配置

```yaml
# 传统方式：每个环境需要完整配置
# 文件：api-autotest/conf/config.yaml

# 环境 1
testing:
  account:
    user: "test@example.com"
    pwd: "xxx"
  host: "https://console.test-env.example.com"
  api_host: "https://api.testing.com"
  access_key_id: "AK_EXAMPLE"
  secret_access_key: "SECRET_XXX"
  zone: ["zone-1"]
  region: "region-1"
  # ... 更多配置

# 环境 2
staging:
  account:
    user: "stage@example.com"
    pwd: "yyy"
  host: "https://console.staging-env.example.com"
  # ... 完全独立的配置

# 客户环境 1
customer-a:
  # ... 又一套配置

# 客户环境 2
customer-b:
  # ... 又一套配置

# 问题：
# - 配置文件越来越大
# - 每个环境需要维护完整配置
# - 凭证明文存储
# - 环境切换需要修改配置或命令行参数
```

```python
# 测试数据也需要单独管理
# 文件：data/test_data/cluster_data/

cluster_config.yaml
├── image_id: "image-001"
├── instance_type: "standard-2c4g"
├── node_count: 3
└── ...

# 问题：
# - 测试数据与测试用例分离
# - 数据版本管理复杂
# - 不同环境可能需要不同数据
```

#### 服务模式：声明式 + K8s 原生

```yaml
# CRD 方式：配置即资源，K8s 原生管理

# 1. 环境隔离：通过 Namespace
apiVersion: v1
kind: Namespace
metadata:
  name: testing      # 测试环境
---
apiVersion: v1
kind: Namespace
metadata:
  name: staging      # 预发环境
---
apiVersion: v1
kind: Namespace
metadata:
  name: customer-a  # 客户环境

# 2. 凭证管理：通过 Secret（加密存储）
apiVersion: v1
kind: Secret
metadata:
  name: resource-credentials
  namespace: testing
data:
  access_key_id: <base64>
  secret_access_key: <base64>

# 3. 配置管理：通过 ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: testing
data:
  zone: "zone-1"
  region: "region-1"

# 4. 测试数据：内嵌或模板引用

# 方式一：内嵌在测试用例中
apiVersion: infra.testplane.io/v1alpha1
kind: IntegrationTest
metadata:
  name: cluster-test
  namespace: testing
spec:
  steps:
    - name: create-cluster
      resource:
        manifest:
          apiVersion: example.io/v1alpha1
          kind: Cluster
          spec:
            # 测试数据直接定义
            confOverrides:
              cluster:
                maininstance:
                  count: 3

---
# 方式二：模板引用（复用配置）
apiVersion: example.io/v1alpha1
kind: AppVersion                    # 配置模板
metadata:
  name: app-template-v1
spec:
  appID: app-12345678
  conf:
    cluster:
      maininstance:
        cpu: 4
        memory: 8192
        count: 3

---
apiVersion: infra.testplane.io/v1alpha1
kind: IntegrationTest
metadata:
  name: cluster-test
spec:
  steps:
    - name: create-cluster
      resource:
        manifest:
          apiVersion: example.io/v1alpha1
          kind: Cluster
          spec:
            appVersionRef:
              name: app-template-v1   # 引用模板
            confOverrides:             # 仅覆盖差异部分
              cluster:
                maininstance:
                  count: 5             # 覆盖为 5 节点
```

```
配置管理对比：

工具模式：
  config.yaml
  ├── testing: {...完整配置...}
  ├── staging: {...完整配置...}
  ├── customer-1: {...完整配置...}
  └── customer-2: {...完整配置...}

  运行时：test-run -e testing -m P0
  切换环境：修改 -e 参数


服务模式：
  K8s Cluster
  ├── namespace: testing
  │   ├── Secret: credentials
  │   ├── ConfigMap: config
  │   └── IntegrationTest: my-test
  │
  ├── namespace: staging
  │   ├── Secret: credentials（不同的凭证）
  │   └── IntegrationTest: my-test（同样的测试）
  │
  └── namespace: customer-a
      └── ...

  运行时：kubectl apply -f test.yaml -n testing
  切换环境：改变 -n 参数
```

**对比**：

| 维度 | 工具模式 | 服务模式 |
|------|---------|---------|
| 环境隔离 | 配置文件切换 | Namespace 隔离 |
| 凭证存储 | 明文配置文件 | K8s Secret（加密） |
| 配置版本 | Git 管理配置文件 | K8s 资源版本化 |
| 测试数据 | 单独的数据文件 | 内嵌 YAML 或模板引用 |
| 环境切换 | 修改命令行参数 | 切换 Namespace |
| 权限控制 | 无 | RBAC 精细控制 |

---

### 4.3 可观测性

> **痛点**：测试执行过程不透明，问题排查困难。

#### 工具模式：事后查看

```
测试执行中：

  $ test-run -e testing -m cluster_P0

  执行中...
  执行中...
  执行中...（无法知道具体进度）

  30 分钟后...

  测试完成，生成报告


问题排查：

  1. 打开 Allure 报告（HTML 文件）
  2. 找到失败用例
  3. 查看日志文件（logs/xxx.log）
  4. 分析失败原因

  痛点：
  - 无法实时看到进度
  - 日志分散在多个文件
  - 结果是静态文件，无法查询
  - 无法与监控系统集成
```

#### 服务模式：实时可见

```bash
# 1. 实时监控进度
$ kubectl get integrationtest -w

NAME           PHASE      STEP              AGE
cluster-test   Running    创建集群           30s
cluster-test   Running    验证健康状态       5m
cluster-test   Succeeded  -                 8m

# 2. 查看详细状态
$ kubectl describe integrationtest cluster-test

Status:
  Phase: Succeeded
  Steps:
    - Name: 创建集群
      State: Succeeded
      Duration: 4m30s
      ExpectationResults:
        - Expect: ClusterHealthy
          Passed: true

# 3. 查看事件流
$ kubectl get events --field-selector involvedObject.name=cluster-test

LAST SEEN   TYPE     REASON         MESSAGE
5m          Normal   StepStarted    Started step '创建集群'
30s         Normal   StepSucceeded  Completed successfully

# 4. Prometheus 指标（自动提供）
controller_runtime_reconcile_total{controller="integrationtest"}
```

**对比**：

| 维度 | 工具模式 | 服务模式 |
|------|---------|---------|
| **进度查看** | 等待完成 | `kubectl get -w` 实时 |
| **详情查看** | 日志文件 | `kubectl describe` |
| **历史记录** | 本地文件 | etcd 持久化 |
| **事件追踪** | 无 | K8s Events |
| **监控告警** | 需自建 | Prometheus 原生 |
| **问题排查** | 翻日志文件 | 结构化 Status |

---

### 4.4 资源清理

> **痛点**：测试创建的云资源（集群、实例等）清理困难，容易残留。

#### 工具模式：努力清理

```python
# 传统方式：手动追踪 + teardown 清理

class TestCluster:
    def setup_method(self):
        self.created_resources = []  # 手动追踪

    def test_something(self):
        cluster_id = create_cluster(...)
        self.created_resources.append(cluster_id)  # 手动记录
        # ...

    def teardown_method(self):
        for resource_id in self.created_resources:
            try:
                delete_resource(resource_id)
            except:
                pass  # 清理失败，资源残留
```

**失败场景**：

```
场景 1：进程崩溃
  create_cluster() → 成功
  测试执行中...
  ╳ 进程崩溃（OOM/网络断开/手动中断）
  teardown_method() → 根本没执行
  结果：资源永久残留

场景 2：清理失败
  teardown_method()
  delete_cluster() → API 错误："集群正在操作中"
  后续清理代码 → 不执行
  结果：部分资源残留

场景 3：子资源遗漏
  delete_cluster() → 成功
  但集群自动创建的 Instance/Volume → 无人追踪
  结果：子资源残留
```

#### 服务模式：自动清理

```yaml
# CRD 方式：Kubernetes 原生机制

apiVersion: example.io/v1alpha1
kind: Cluster
metadata:
  name: test-cluster
  finalizers:
    - example.io/finalizer    # 终结器
  ownerReferences:
    - apiVersion: infra.testplane.io/v1alpha1
      kind: IntegrationTest
      name: my-test                      # 父资源
```

**Kubernetes 清理机制**：

```
1. Finalizer（终结器）
   → 删除前必须执行清理逻辑
   → 即使 Controller 重启，也会继续

2. Owner Reference（所有者引用）
   → 父资源删除，子资源自动级联删除
   → 无需手动追踪

3. Garbage Collection（垃圾回收）
   → 孤儿资源自动回收
   → 后台持续运行
```

**对比**：

| 场景 | 工具模式 | 服务模式 |
|------|---------|---------|
| 进程崩溃 | ❌ 资源残留 | ✅ 自动恢复清理 |
| 清理失败 | ❌ 部分残留 | ✅ 持续重试 |
| 子资源 | ❌ 无法追踪 | ✅ 自动级联 |
| 清理顺序 | ❌ 手动维护 | ✅ Finalizer 保证 |

---

### 4.5 与 K8s 环境的适配性

```
被测系统的 K8s 转型现状：

┌─────────────────────────────────────────────────────────────────┐
│                        K8s 集群                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   当前状态：                                                     │
│   ┌─────────────────┐                                           │
│   │ 云平台服务       │  ← 部署在 K8s（容器化）                    │
│   │ (API Server)    │  ← 但资源管理仍通过 HTTP API               │
│   └─────────────────┘                                           │
│                                                                 │
│   未来可能：                                                     │
│   ┌─────────────────┐                                           │
│   │ resource-      │  ← 资源管理 CRD 化                         │
│   │ operator        │  ← 声明式管理云资源                        │
│   └─────────────────┘                                           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**两种测试方式的适配性**：

```
工具模式（传统 API 测试）：

  ┌──────────────────┐         ┌──────────────────┐
  │ api-autotest│  HTTP   │ 云平台 API       │
  │ (外部工具)        │ ──────→ │ (K8s 内或外)     │
  └──────────────────┘         └──────────────────┘
         ↑
         │
  需要网络访问、凭证配置
  与 K8s 环境相对独立


服务模式（CRD 测试）：

  ┌─────────────────────────────────────────────────┐
  │                  K8s 集群内                       │
  │                                                 │
  │  ┌────────────┐    ┌──────────────┐    ┌─────┐ │
  │  │ TestPlane  │ →  │ resource-   │ →  │云平台│ │
  │  │ Controller │    │ operator     │    │ API │ │
  │  └────────────┘    │ (CRD封装层)  │    └─────┘ │
  │                    └──────────────┘            │
  │                                                 │
  │  天然融入 K8s 生态                               │
  └─────────────────────────────────────────────────┘
```

**适配性对比**：

| 场景 | 工具模式 | 服务模式 |
|------|---------|---------|
| 被测系统部署在 K8s | 需要额外网络配置 | 天然在同一集群 |
| 被测系统使用 API | ✅ 直接测试 API | ✅ 可通过 Webhook/Job 调用 API |
| 被测系统使用 CRD | ❌ 无法直接测试 | ✅ 原生支持 |
| 混合架构（API + CRD） | 部分支持 | 完全支持 |
| 未来 CRD 化演进 | 需要重写测试 | 平滑过渡 |

**面向未来的考虑**：

```
如果被测系统未来 CRD 化：

工具模式：
  现有测试 ──→ 测试 HTTP API
                    ↓
            API 废弃或变化
                    ↓
            测试需要重写

服务模式：
  现有测试 ──→ 测试 API（通过 Webhook/Job）
                    ↓
            被测系统 CRD 化
                    ↓
            测试平滑切换到测试 CR
            （只需修改 resource manifest）
```

**当前阶段的选择**：

即使被测系统暂未 CRD 化，服务模式仍有优势：
- 测试服务部署在 K8s，与被测系统同环境
- 利用 K8s 的调度、监控、日志能力
- 为未来 CRD 化做好架构准备
- 协作和交付场景的优势不受影响

---

## 五、总结

### 核心观点

| | 工具 | 服务 |
|---|------|------|
| **本质** | 人来使用 | 自己运行 |
| **协作** | 测试执行，开发等待 | 测试维护，开发自助 |
| **交付** | 带工具去现场 | 服务随产品交付 |
| **用例开发** | 代码壁垒，开发难参与 | YAML 声明，开发可贡献 |
| **修改数据** | 改代码 + 提交 + 等待 | 改 YAML + apply 即生效 |
| **清理** | 努力清理，可能残留 | 自动清理，可靠保证 |
| **状态** | 内存，崩溃丢失 | 持久化，可恢复 |
