# TestPlane

TestPlane 是一个基于 Kubebuilder（Operator SDK 插件） 的 Kubernetes Operator，为云平台资源提供声明式基础设施测试能力。

通过 CRD 化的方式，将复杂、异步、长生命周期的测试过程纳入统一的状态管理体系。

## 核心特性

- **声明式测试**：通过 CRD 描述测试步骤与期望，配置即测试
- **异步友好**：控制器级等待与超时机制，替代脚本式轮询
- **插件化断言**：业务判断封装为 Expectation 插件，可按需扩展
- **可观测性**：测试进度与结果写入 CR `status`，便于审计与排障
- **中断恢复**：控制器重启不会中断测试，可从当前阶段继续执行
- **状态对齐**：断言基于资源 status（由资源控制器维护）进行验证

## 核心 CRD

| CRD | 说明 | 状态机 |
|-----|------|--------|
| **IntegrationTest** | 集成测试用例，包含步骤和期望，用于验证基础设施行为 | Pending → Running → Succeeded/Failed/Aborted |
| **LoadTest** | 负载测试用例，支持持续运行和周期性检查 | Pending → Initializing → Running → Succeeded/Failed |

## 快速开始

### 前置条件

- Go 1.24+
- Docker（用于构建镜像）
- kubectl（已配置集群访问）
- Kind 或 Kubernetes 集群

### 安装

```bash
# 克隆项目
git clone https://github.com/lunz1207/testplane.git
cd testplane

# 安装 CRD 到集群
make install

# 本地运行控制器
make run
```

### 构建与部署

```bash
# 构建控制器
make build

# 构建并推送 Docker 镜像
make docker-build docker-push IMG=<registry>/testplane:tag

# 部署控制器到集群
make deploy IMG=<registry>/testplane:tag
```

## IntegrationTest 使用示例

### 创建集成测试

```yaml
apiVersion: infra.testplane.io/v1alpha1
kind: IntegrationTest
metadata:
  name: deployment-test
spec:
  mode: Sequential
  steps:
    - name: create-deployment
      template:
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
      expectations:
        timeoutSeconds: 300
        allOf:
          - function: ResourceExists
```

```bash
kubectl apply -f integrationtest.yaml
```

### 查看测试状态

```bash
# 查看测试进度
kubectl get integrationtest deployment-test -o yaml

# 查看测试阶段
kubectl get integrationtest deployment-test -o jsonpath='{.status.phase}'
```

## LoadTest 使用示例

### 创建负载测试

```yaml
apiVersion: infra.testplane.io/v1alpha1
kind: LoadTest
metadata:
  name: app-load-test
spec:
  # 测试目标（Template 或 Selector 二选一）
  target:
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
    readyCondition:
      timeoutSeconds: 300
      allOf:
        - function: ResourceExists

  # 负载定义
  workload:
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
                    - name: load-generator
                      image: load-test:latest

  # 运行期断言（周期性检查，默认检查 target 资源）
  expectations:
    intervalSeconds: 30
    failureThreshold: 3
    allOf:
      - function: ResourceExists
```

### 查看负载测试状态

```bash
# 查看测试状态
kubectl get loadtest app-load-test

# 查看详细进度
kubectl describe loadtest app-load-test

# 查看注入的环境变量
kubectl get loadtest app-load-test -o jsonpath='{.status.injectedValues}'
```

## 内置期望函数

### 通用函数

| 期望函数 | 说明 | 参数 |
|---------|------|------|
| `ResourceExists` | 检查资源是否存在 | 无 |
| `ResourceNotExists` | 检查资源是否不存在 | 无 |
| `DeploymentAvailable` | 检查 Deployment 是否有可用副本 | 无 |

### Kubernetes 资源就绪检查

| 期望函数 | 说明 | 参数 |
|---------|------|------|
| `DeploymentReady` | Deployment 就绪（available >= replicas 且 updated >= replicas） | 无 |
| `StatefulSetReady` | StatefulSet 就绪（ready >= replicas 且版本一致） | 无 |
| `DaemonSetReady` | DaemonSet 就绪（ready >= desired） | 无 |
| `PodReady` | Pod 就绪（Running 且所有容器 Ready） | 无 |
| `PodComplete` | Pod 已完成（phase=Succeeded） | 无 |
| `JobComplete` | Job 已完成（succeeded >= completions） | 无 |
| `ServiceReady` | Service 已就绪（有 ClusterIP 或 ExternalName） | 无 |
| `PVCBound` | PVC 已绑定（phase=Bound） | 无 |

### Cluster 断言函数

| 期望函数 | 说明 | 参数 |
|---------|------|------|
| `ClusterReady` | 集群就绪（phase=active 且无 transitionStatus） | 无 |
| `ClusterHealthy` | 集群健康（phase=active、health=healthy） | 无 |
| `ClusterPending` | 集群 pending 状态 | 无 |
| `ClusterStopped` | 集群已停止 | 无 |
| `ClusterDeleted` | 集群已删除 | 无 |
| `ClusterCeased` | 集群已销毁 | 无 |
| `ClusterPhaseEquals` | 通用 phase 检查 | `phase: string`, `ignoreTransition: bool` |
| `ClusterNodeCount` | 检查集群节点数量 | `expected: int` |
| `ClusterSecurityGroupExists` | 检查集群安全组存在 | `id: string`（可选） |
| `ClusterSecurityGroupNotExists` | 检查集群安全组不存在 | `id: string`（可选） |

### Instance 断言函数

| 期望函数 | 说明 | 参数 |
|---------|------|------|
| `InstanceReady` | 实例就绪（phase=running） | 无 |
| `InstanceStopped` | 实例已停止 | 无 |
| `InstancePending` | 实例 pending 状态 | 无 |
| `InstanceSuspended` | 实例已暂停 | 无 |
| `InstanceTerminated` | 实例已终止 | 无 |
| `InstanceCeased` | 实例已销毁 | 无 |
| `InstancePhaseEquals` | 通用 phase 检查 | `phase: string`, `ignoreTransition: bool` |
| `InstanceSecurityGroupExists` | 检查实例安全组存在 | `id: string`（可选） |
| `InstanceSecurityGroupNotExists` | 检查实例安全组不存在 | `id: string`（可选） |

### 提取函数（用于 LoadTest EnvInjection）

| 提取函数 | 说明 | 参数 |
|---------|------|------|
| `FieldPath` | 通用字段路径提取器 | `path: string`（如 "status.phase"） |
| `ClusterNodeURL` | 获取指定角色节点的 IP | `role: string`, `index: int`（默认 0） |
| `ClusterNodeIP` | 获取节点私有 IP | `role: string`（可选）, `index: int` |
| `ClusterID` | 获取集群 ID | 无 |
| `ClusterVIP` | 获取指定名称的 VIP | `name: string` |
| `ClusterClientPort` | 获取客户端端口 | 无 |

## 执行模式

### IntegrationTest 模式

- **Sequential（顺序模式）**：按步骤顺序执行，每步验证期望后再执行下一步
- **Parallel（并行模式）**：所有步骤并行执行，全部完成后验证最终期望

### 重复执行

```yaml
spec:
  repeat:
    count: 10                    # 重复 10 轮
    maxDurationSeconds: 1800     # 最长执行 30 分钟
    delayBetweenRounds: 30       # 每轮间隔 30 秒
    untilFailure: true           # 遇到失败则停止
```

### 断言配置

#### WaitCondition（统一断言配置）

WaitCondition 支持两种模式：
- **超时模式**：用于 IntegrationTest 步骤/最终断言，通过 `timeoutSeconds` 配置
- **周期模式**：用于 LoadTest 运行期断言，通过 `intervalSeconds` + `failureThreshold` 配置

```yaml
# IntegrationTest 步骤/最终断言（超时模式）
expectations:
  timeoutSeconds: 300          # 超时时间（秒）
  allOf:                       # 所有期望都必须满足
    - function: ResourceExists
  anyOf:                       # 任一期望满足即可
    - function: ResourceExists
```

```yaml
# LoadTest 运行期断言（周期模式）
expectations:
  intervalSeconds: 30          # 检查间隔（秒）
  failureThreshold: 3          # 连续失败阈值
  allOf:
    - function: ResourceExists
```

## 开发指南

### 常用命令

```bash
# 运行单元测试
make test

# 运行 e2e 测试
make test-e2e

# 代码格式化与检查
make fmt lint

# 生成 CRD manifests 和 DeepCopy 方法（修改 API 类型后必须执行）
make manifests generate
```

### 添加新的期望函数

1. 在 `internal/controller/framework/plugin/` 中创建期望函数：

```go
func MyExpect(s Snapshot, p Params) Result {
    expected := p.String("expected")
    actual := s.Status().String("myField")

    return Check(actual == expected).
        Expected(expected).
        Actual(actual).
        Result()
}
```

2. 在 `internal/controller/framework/plugin/builtin.go` 中注册：

```go
registry.Register("MyExpect", MyExpect)
```

3. 在 Case YAML 中使用：

```yaml
expectations:
  allOf:
    - function: MyExpect
      params:
        expected: "value"
```

### 添加新的 Extractor 函数

1. 在 `internal/builtins/extraction.go` 中创建提取函数：

```go
func MyExtractor(resource, params map[string]interface{}) plugin.Result {
    value := plugin.GetNestedString(resource, "status", "someField")
    return plugin.Extract(value)
}
```

2. 在 `internal/builtins/register.go` 中注册：

```go
r.Register("MyExtractor", MyExtractor)
```

3. 在 LoadTest 中使用（提取的值会注入到 Pod annotations）：

```yaml
workload:
  envInjection:
    - name: MY_VAR
      extract:
        function: MyExtractor
        params:
          key: value
  resources:
    - manifest:
        # ... Pod/Deployment 模板
        spec:
          containers:
            - env:
                - name: MY_VAR
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.annotations['testplane.io/inject-my-var']
```

**注入机制说明**：
- Controller 从 Target 资源提取值，写入 Workload Pod template 的 annotations
- Annotation key 格式：`testplane.io/inject-{name}`（name 转为 kebab-case）
- 用户通过 Kubernetes Downward API 引用这些 annotations 作为环境变量

## 项目结构

```
├── api/v1alpha1/                    # CRD 类型定义
│   ├── common_types.go              # 共用类型（Expectation、ResourceSelector 等）
│   ├── integrationtest_types.go     # IntegrationTest CRD
│   └── loadtest_types.go            # LoadTest CRD
├── cmd/                             # 程序入口
├── internal/controller/
│   └── framework/                   # 测试框架
│       ├── expectation_runner.go    # 期望执行引擎
│       ├── plugin/                  # 期望函数插件
│       │   ├── functions.go         # 期望函数实现
│       │   ├── builtin.go           # 内置函数注册
│       │   ├── registry.go          # 函数注册表
│       │   └── result.go            # 结果类型
│       ├── integrationtest/         # IntegrationTest 控制器
│       │   ├── integrationtest_controller.go
│       │   ├── execution.go         # 执行逻辑（顺序/并行）
│       │   ├── step_runner.go       # 步骤执行器
│       │   └── lifecycle.go         # 生命周期管理
│       ├── loadtest/                # LoadTest 控制器
│       │   ├── loadtest_controller.go
│       │   ├── target.go            # Target 处理
│       │   ├── workload.go          # Workload 应用与 annotation 注入
│       │   ├── injection.go         # 值提取
│       │   └── running.go           # 运行期断言检查
│       └── resource/                # 资源管理
│           ├── manager.go           # 资源应用/删除/等待
│           └── template.go          # 资源模板展开
├── config/
│   ├── crd/bases/                   # 生成的 CRD YAML
│   └── samples/                     # 示例 CR
├── docs/                            # 详细文档
└── test/e2e/                        # 端到端测试
```

## 设计文档

- [TestPlane API 设计](docs/TestPlane.md)
- [断言系统设计](docs/expectation.md)
- [控制器设计](docs/controller.md)
- [事件机制](docs/event.md)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0.
