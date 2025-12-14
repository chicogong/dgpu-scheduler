# DGPU Scheduler 分布式调度系统设计

**版本**: 1.0
**日期**: 2025-12-14
**状态**: 设计阶段

## 1. 项目概述

### 1.1 背景

构建一个面向GPU集群的分布式调度系统，支持混合负载（在线推理服务 + 批处理任务），提供资源配额管理、优先级调度、高可用保障等能力。

### 1.2 设计目标

- **集群规模**: 中等规模（50-200台GPU节点，多种GPU型号）
- **工作负载**: 混合负载（在线推理 + 批处理任务）
- **资源隔离**: 严格配额隔离（在线服务60-70%，批处理30-40%）
- **调度策略**: FIFO + 优先级队列
- **扩展性**: 第一版静态资源分配，预留自动扩缩容接口
- **架构**: 集中式调度（主备模式）+ 分布式Agent执行
- **依赖**: 无外部中间件（不依赖Redis/etcd）
- **API**: gRPC（内部Agent）+ HTTP REST（外部用户）
- **可观测性**: 第一版结构化日志，第二版Prometheus指标

---

## 2. 系统架构

### 2.1 整体架构

系统采用三层架构设计：

```
┌─────────────────────────────────────────────────┐
│           用户/服务 (HTTP REST)                   │
└─────────────────┬───────────────────────────────┘
                  │
        ┌─────────▼─────────┐
        │   API Gateway     │
        │  (REST + gRPC)    │
        └─────────┬─────────┘
                  │
    ┌─────────────▼──────────────┐
    │   Scheduler Master (主备)   │
    │   - 调度引擎                │
    │   - 状态管理                │
    │   - 配额管理                │
    │   - 主备同步                │
    └─────────────┬──────────────┘
                  │ gRPC
         ┌────────┼────────┐
         │        │        │
    ┌────▼───┐ ┌─▼────┐ ┌─▼────┐
    │ Agent  │ │Agent │ │Agent │
    │ GPU节点 │ │GPU节点│ │GPU节点│
    └────────┘ └──────┘ └──────┘
```

### 2.2 核心组件

**1. 调度层（Scheduler Master）**
- 主节点（Active Master）：接收调度请求，执行调度决策
- 备节点（Standby Master）：热备状态，实时同步主节点状态
- 内存存储：所有GPU状态、任务队列、配额信息保存在内存
- 持久化：定期快照到本地文件，用于故障恢复

**2. 执行层（Agent）**
- 每个GPU节点运行一个Agent进程
- 职责：心跳上报GPU状态、接收任务、执行任务、上报结果
- 通过gRPC与Scheduler Master通信（双向流）

**3. 接入层（API Gateway）**
- 对内：gRPC接口（Agent调用，高性能）
- 对外：HTTP REST接口（用户/服务调用，易用性）
- 两种协议共享底层调度逻辑

---

## 3. 核心组件详细设计

### 3.1 Scheduler Master

**核心模块：**

#### 3.1.1 状态管理器（State Manager）
- 维护全局状态（内存）：
  - GPU资源池：每个GPU的ID、类型（A100/V100/T4）、状态（空闲/占用/故障）
  - 任务队列：高优先级队列（在线推理）、低优先级队列（批处理）
  - 配额信息：在线服务已用/总配额、批处理已用/总配额
- 提供线程安全的读写接口（读写锁）
- 定期快照到本地文件（每30秒或状态变更时）

#### 3.1.2 调度引擎（Scheduler Engine）
- 调度算法：
  - 检查配额是否充足
  - 从对应优先级队列取任务（FIFO）
  - 选择空闲GPU（简单轮询或随机）
  - 更新GPU状态为占用
- 调度周期：事件驱动（任务提交、任务完成时触发）+ 定期扫描（每5秒）

#### 3.1.3 主备同步器（Replication Manager）
- 主节点：每次状态变更时，通过gRPC流推送给备节点
- 备节点：接收状态更新，同步到本地内存
- 心跳检测：主备互相ping（每2秒），超时则触发切换

#### 3.1.4 Agent管理器（Agent Manager）
- 维护Agent列表（节点ID、心跳时间、GPU列表）
- 接收Agent心跳（gRPC双向流），更新GPU状态
- 超时检测：Agent超过15秒未心跳则标记为离线，释放其GPU

### 3.2 Agent

**核心模块：**

#### 3.2.1 GPU检测器（GPU Detector）
- 启动时：探测本机GPU信息（nvidia-smi或NVML API）
  - GPU ID（0, 1, 2...）
  - GPU型号（A100/V100/T4）
  - 显存大小
- 注册到Scheduler Master
- 定期检测GPU健康状态（温度、显存占用）

#### 3.2.2 心跳管理器（Heartbeat Manager）
- 建立与Scheduler Master的gRPC���向流连接
- 每5秒发送心跳：
  - Agent ID
  - 每个GPU的当前状态（空闲/忙碌）
  - 每个GPU的利用率、显存使用量
- 接收Scheduler Master的响应（确认存活）

#### 3.2.3 任务执行器（Task Executor）
- 从Scheduler Master接收任务分配
- 执行流程：
  1. 设置环境变量（CUDA_VISIBLE_DEVICES）
  2. 启动任务进程（Docker容器或本地进程）
  3. 监控任务状态（运行中/成功/失败）
  4. 任务结束后通知Scheduler Master释放GPU
- 支持任务类型：
  - 在线推理：长期运行的服务进程
  - 批处理：一次性任务

#### 3.2.4 故障恢复（Failure Recovery）
- Scheduler连接断开时：
  - 尝试重连（指数退避：1s, 2s, 4s...最多30s）
  - 主备切换：如果主Scheduler不可达，尝试连接备Scheduler
- 任务失败处理：
  - 记录失败日志
  - 上报Scheduler Master（任务失败 + 错误码）
  - Scheduler决定是否重试

---

## 4. 数据模型

### 4.1 GPU资源模型

```go
type GPU struct {
    ID          string        // "node-1-gpu-0"
    NodeID      string        // "node-1"
    DeviceIndex int           // 0, 1, 2...
    Model       string        // "A100", "V100", "T4"
    Memory      int64         // 显存大小（MB）
    Status      GPUStatus     // Idle, Busy, Offline
    CurrentTask *string       // 当前运行的任务ID（nullable）
    UpdatedAt   time.Time     // 最后更新时间
}

type GPUStatus string
const (
    GPUStatusIdle    = "idle"
    GPUStatusBusy    = "busy"
    GPUStatusOffline = "offline"
)
```

### 4.2 任务模型

```go
type Task struct {
    ID          string        // 任务唯一ID
    Priority    Priority      // High (在线), Low (批处理)
    GPUCount    int           // 需要的GPU数量
    GPUModel    *string       // 期望的GPU型号（可选）
    Command     string        // 执行命令
    Env         map[string]string  // 环境变量
    Status      TaskStatus    // Pending, Running, Success, Failed
    AssignedGPUs []string     // 分配的GPU ID列表
    CreatedAt   time.Time
    StartedAt   *time.Time    // nullable
    FinishedAt  *time.Time    // nullable
    Error       *string       // 错误信息（nullable）
}

type Priority string
const (
    PriorityHigh = "high"  // 在线推理
    PriorityLow  = "low"   // 批处理
)

type TaskStatus string
const (
    TaskStatusPending = "pending"
    TaskStatusRunning = "running"
    TaskStatusSuccess = "success"
    TaskStatusFailed  = "failed"
)
```

### 4.3 Agent节点模型

```go
type Agent struct {
    ID            string      // 节点ID
    Address       string      // gRPC地址
    GPUs          []GPU       // 该节点的GPU列表
    LastHeartbeat time.Time   // 最后心跳时间
    Status        AgentStatus // Online, Offline
}

type AgentStatus string
const (
    AgentStatusOnline  = "online"
    AgentStatusOffline = "offline"
)
```

### 4.4 配额模型

```go
type Quota struct {
    TotalGPUs       int  // 集群总GPU数
    OnlineQuota     int  // 在线服务配额（60-70%）
    BatchQuota      int  // 批处理配额（30-40%）
    OnlineUsed      int  // 在线服务已用
    BatchUsed       int  // 批处理已用
}
```

---

## 5. 调度流程

### 5.1 任务提交流程

```
用户/服务 → REST API: POST /tasks
  ↓
验证任务参数（GPUCount、Priority等）
  ↓
检查配额是否充足
  ↓
任务入队：
  - Priority=High → 高优先级队列
  - Priority=Low → 低优先级队列
  ↓
返回任务ID给用户
  ↓
触发调度器执行调度
```

### 5.2 调度决策流程

```
调度器轮询（事件驱动 + 定期5秒）：
  ↓
Step 1: 从队列取任务（优先高优先级队列）
  ↓
Step 2: 检查配额
  - 任务Priority=High: OnlineUsed + GPUCount <= OnlineQuota?
  - 任务Priority=Low: BatchUsed + GPUCount <= BatchQuota?
  - 不满足 → 任务保持Pending，跳过
  ↓
Step 3: 查找可用GPU
  - 过滤条件：Status=Idle && (任务指定型号 ? Model匹配 : true)
  - 选择算法：随机选择或轮询（负载均衡）
  - 找不到足够GPU → 任务保持Pending
  ↓
Step 4: 分配GPU
  - 更新GPU.Status = Busy
  - GPU.CurrentTask = TaskID
  - Task.Status = Running
  - Task.AssignedGPUs = [选中的GPU IDs]
  - 更新配额计数（OnlineUsed++ 或 BatchUsed++）
  ↓
Step 5: 下发任务到Agent
  - gRPC调用：agent.RunTask(task)
  - Agent返回确认
```

### 5.3 任务完成流程

```
Agent执行完任务
  ↓
gRPC上报：scheduler.TaskFinished(taskID, status, error)
  ↓
Scheduler更新状态：
  - Task.Status = Success/Failed
  - Task.FinishedAt = now()
  - 释放GPU：GPU.Status = Idle, CurrentTask = nil
  - 更新配额：OnlineUsed-- 或 BatchUsed--
  ↓
触发下一轮调度（处理队列中的Pending任务）
```

### 5.4 失败重试策略

- 任务失败时，根据错误码判断是否重试
- GPU故障（OOM、硬件错误）→ 不重试，标记Failed
- 临时错误（网络超时）→ 重试最多3次
- 重试任务重新入队，优先级不变

---

## 6. 配额管理

### 6.1 配额初始化

```
启动时计算：
  - 统计所有在线GPU总数 → TotalGPUs
  - 配置文件指定比例（默认70:30）
  - OnlineQuota = TotalGPUs × 0.7
  - BatchQuota = TotalGPUs × 0.3
  - OnlineUsed = 0
  - BatchUsed = 0
```

### 6.2 配额检查逻辑

```go
func CanScheduleTask(task *Task, quota *Quota) bool {
    switch task.Priority {
    case PriorityHigh:
        return quota.OnlineUsed + task.GPUCount <= quota.OnlineQuota
    case PriorityLow:
        return quota.BatchUsed + task.GPUCount <= quota.BatchQuota
    default:
        return false
    }
}
```

### 6.3 配额动态调整

支持热更新配额比例：
- REST API: `PUT /quota`
- 参数：`onlinePercent` (0.6-0.8)
- 验证：`onlineUsed <= 新OnlineQuota`
- 更新配额，不影响已运行任务

### 6.4 配额超售处理（未来扩展）

- 第一版：严格隔离，不允许超售
- 第二版可选：允许批处理任务借用在线配额
  - 条件：`OnlineUsed < OnlineQuota × 0.5`（在线服务空闲）
  - 批处理任务使用"借来的"GPU
  - 在线任务提交时，可抢占批处理任务
  - 需要实现抢占机制（杀掉批处理任务，释放GPU）

### 6.5 配额统计和监控

实时统计：
- 每次任务分配/释放时更新计数
- 定期校验：遍历所有GPU，重新计算Used
- 防止计数器漂移（任务异常退出导致）

暴露指标：
- `quota_total`：总GPU数
- `quota_online_limit`：在线配额
- `quota_online_used`：在线已用
- `quota_batch_limit`：批处理配额
- `quota_batch_used`：批处理已用

### 6.6 边界情况处理

- Agent离线：释放该Agent的GPU，更新配额计数
- GPU故障：从TotalGPUs中扣除，重新计算配额
- 集群扩容：新增GPU自动纳入配额池

---

## 7. 容错和高可用

### 7.1 主备切换机制

**部署模型：**
- 主节点：`scheduler-master-1`（默认Active）
- 备节点：`scheduler-master-2`（Standby）

**启动时角色选举：**
- 简化方案：配置文件指定主备角色（`master=true/false`）
- 未来可选：基于文件锁的自动选举（共享存储上创建lock文件）

### 7.2 状态同步协议

**主节点 → 备节点（gRPC流）：**
- 每次状态变更立即推送：
  - 任务提交/完成事件
  - GPU状态变更
  - 配额更新
- 备节点接收后更新内存状态
- 备节点确认（ACK）

**数据格式：**
```protobuf
message StateUpdate {
    enum Type { TASK, GPU, QUOTA }
    Type type = 1;
    bytes data = 2;  // JSON序列化的数据
    int64 version = 3;  // 版本号，防止乱序
}
```

### 7.3 心跳检测与切换

**主备互ping：**
- 每2秒发送心跳
- 超时阈值：3次心跳（6秒）

**主节点故障检测（备节点视角）：**
- 连续3次心跳超时
- gRPC连接断开
- → 备节点提升为主节点

**切换流程：**
1. 备节点标记自己为Active
2. 开始接受任务提交请求
3. 开始执行调度逻辑
4. 通知所有Agent：新的Master地址（通过心跳响应）

**Agent侧切换：**
- Agent同时配置主备地址
- 心跳响应中包含"我是Master"标志
- Agent自动切换连接目标

### 7.4 脑裂预防

**问题：** 网络分区导致主备都认为对方挂了

**解决方案（简化版）：**
- 主节点定期写入"我还活着"标记到共享存储（NFS/本地文件）
- 备节点检测到主节点心跳超时后，检查共享存储
- 如果主节点标记仍在更新 → 不切换（网络问题）
- 如果主节点标记超时 → 切换

**未来优化：**
- 引入第三方仲裁（如etcd的lease机制）
- 但第一版尽量简单

### 7.5 状态恢复机制

**主节点故障后重启：**
1. 读取本地快照文件（最后一次持久化的状态）
2. 检查当前谁是Active Master
3. 如果备节点已接管 → 自己降级为Standby
4. 如果无Active Master → 提升为Active
5. 从Agent心跳中恢复最新GPU状态
6. 重新加载Pending队列中的任务

### 7.6 Agent容错

**Agent离线检测：**
- Scheduler超过15秒未收到Agent心跳
- 标记Agent为Offline
- 释放该Agent管理的所有GPU
- 运行在该Agent上的任务标记为Failed

**Agent恢复：**
- Agent重连后重新注册
- 上报GPU状态
- Scheduler重新纳入调度池

---

## 8. API接口设计

### 8.1 REST API（外部用户接口）

#### 8.1.1 任务管理接口

**提交任务：**
```http
POST /api/v1/tasks
Content-Type: application/json

{
  "priority": "high",          // "high" | "low"
  "gpu_count": 2,              // 需要的GPU数量
  "gpu_model": "A100",         // 可选，指定GPU型号
  "command": "python train.py",
  "env": {                     // 环境变量
    "MODEL_PATH": "/models/gpt"
  }
}

Response 201:
{
  "task_id": "task-abc123",
  "status": "pending",
  "created_at": "2025-12-14T10:00:00Z"
}
```

**查询任务状态：**
```http
GET /api/v1/tasks/{task_id}

Response 200:
{
  "task_id": "task-abc123",
  "priority": "high",
  "status": "running",         // "pending" | "running" | "success" | "failed"
  "assigned_gpus": ["node-1-gpu-0", "node-1-gpu-1"],
  "created_at": "2025-12-14T10:00:00Z",
  "started_at": "2025-12-14T10:00:05Z",
  "finished_at": null,
  "error": null
}
```

**取消任务：**
```http
DELETE /api/v1/tasks/{task_id}

Response 200:
{
  "message": "Task cancelled"
}
```

**列出任务：**
```http
GET /api/v1/tasks?status=running&priority=high&limit=50

Response 200:
{
  "tasks": [...],
  "total": 100
}
```

#### 8.1.2 集群状态接口

**查询GPU资源：**
```http
GET /api/v1/gpus

Response 200:
{
  "total": 100,
  "idle": 45,
  "busy": 50,
  "offline": 5,
  "gpus": [
    {
      "id": "node-1-gpu-0",
      "model": "A100",
      "status": "busy",
      "current_task": "task-abc123"
    }
  ]
}
```

**查询配额：**
```http
GET /api/v1/quota

Response 200:
{
  "total_gpus": 100,
  "online": {
    "quota": 70,
    "used": 45,
    "available": 25
  },
  "batch": {
    "quota": 30,
    "used": 20,
    "available": 10
  }
}
```

**更新配额比例（管理员）：**
```http
PUT /api/v1/quota
Content-Type: application/json

{
  "online_percent": 0.65  // 65%给在线服务
}

Response 200:
{
  "message": "Quota updated"
}
```

### 8.2 gRPC API（内部Agent接口）

**protobuf定义：**

```protobuf
service SchedulerService {
  // Agent注册
  rpc RegisterAgent(RegisterRequest) returns (RegisterResponse);

  // Agent心跳（双向流）
  rpc Heartbeat(stream HeartbeatRequest) returns (stream HeartbeatResponse);

  // 任务完成通知
  rpc TaskFinished(TaskFinishedRequest) returns (TaskFinishedResponse);
}

message RegisterRequest {
  string agent_id = 1;
  string address = 2;
  repeated GPU gpus = 3;
}

message HeartbeatRequest {
  string agent_id = 1;
  repeated GPUStatus gpu_status = 2;
}

message HeartbeatResponse {
  bool is_master = 1;      // 标识是否为主节点
  repeated Task tasks = 2;  // 下发的新任务
}

message TaskFinishedRequest {
  string task_id = 1;
  string status = 2;       // "success" | "failed"
  string error = 3;        // 可选错误信息
}

message GPU {
  string id = 1;
  int32 device_index = 2;
  string model = 3;
  int64 memory = 4;
}

message GPUStatus {
  string id = 1;
  string status = 2;       // "idle" | "busy" | "offline"
  float utilization = 3;   // GPU利用率 0-100
  int64 memory_used = 4;   // 已用显存
}
```

**主备同步gRPC：**

```protobuf
service ReplicationService {
  // 主节点推送状态更新到备节点
  rpc SyncState(stream StateUpdate) returns (stream SyncAck);

  // 心跳检测
  rpc Ping(PingRequest) returns (PingResponse);
}

message StateUpdate {
  enum Type {
    TASK = 0;
    GPU = 1;
    QUOTA = 2;
  }
  Type type = 1;
  bytes data = 2;      // JSON序列化
  int64 version = 3;   // 版本号
}
```

---

## 9. 监控和日志

### 9.1 日志设计（第一版重点）

#### 9.1.1 日志格式

采用结构化JSON日志：

```json
{
  "timestamp": "2025-12-14T10:00:00.123Z",
  "level": "info",           // debug, info, warn, error
  "component": "scheduler",  // scheduler, agent, api
  "event": "task_scheduled", // 事件类型
  "task_id": "task-abc123",
  "details": {
    "priority": "high",
    "gpu_count": 2,
    "assigned_gpus": ["node-1-gpu-0", "node-1-gpu-1"]
  }
}
```

#### 9.1.2 关键日志事件

**Scheduler Master日志：**
- `scheduler_started`: 调度器启动
- `role_changed`: 主备角色切换（master/standby）
- `task_submitted`: 任务提交
- `task_scheduled`: 任务调度成功
- `task_pending`: 任务因配额/资源不足保持Pending
- `task_finished`: 任务完成
- `task_failed`: 任务失败
- `agent_registered`: Agent注册
- `agent_offline`: Agent离线
- `quota_updated`: 配额更新

**Agent日志：**
- `agent_started`: Agent启动
- `gpu_detected`: GPU检测
- `heartbeat_sent`: 心跳发送
- `task_received`: 收到任务
- `task_started`: 任务开始执行
- `task_completed`: 任务执行完成
- `master_switched`: Master切换

#### 9.1.3 日志输出

**第一版：**
- 输出到stdout/stderr（JSON格式）
- 容器化部署时由日志收集器采集（如Fluentd）
- 本地开发时可重定向到文件

**目录结构：**
```
/var/log/dgpu-scheduler/
  ├── scheduler.log      # Scheduler Master日志
  ├── agent.log          # Agent日志
  └── api.log            # API Gateway日志
```

### 9.2 监控指标（第二版迭代）

#### 9.2.1 Prometheus指标

**任务指标：**
- `task_submitted_total{priority="high|low"}` - 任务提交总数
- `task_scheduled_total{priority="high|low"}` - 调度成功总数
- `task_pending_total{priority="high|low"}` - 当前Pending任务数
- `task_running_total{priority="high|low"}` - 当前运行任务数
- `task_finished_total{status="success|failed"}` - 完成任务数

**GPU指标：**
- `gpu_total` - GPU总数
- `gpu_idle_total` - 空闲GPU数
- `gpu_busy_total` - 占用GPU数
- `gpu_offline_total` - 离线GPU数
- `gpu_utilization{gpu_id, model}` - GPU利用率

**配额指标：**
- `quota_online_limit` - 在线配额
- `quota_online_used` - 在线已用
- `quota_batch_limit` - 批处理配额
- `quota_batch_used` - 批处理已用

**调度器指标：**
- `scheduler_is_master` - 是否为主节点（0/1）
- `agent_count{status="online|offline"}` - Agent数量

#### 9.2.2 暴露指标接口

```http
GET /metrics
# Prometheus抓取这个端点
```

### 9.3 告警规则（第二版）

**预定义告警（未来接入AlertManager）：**

```yaml
# 配额使用率过高
- alert: QuotaHighUsage
  expr: quota_online_used / quota_online_limit > 0.9
  for: 5m

# Agent大量离线
- alert: AgentMassOffline
  expr: agent_count{status="offline"} > 5
  for: 2m

# 任务大量失败
- alert: TaskHighFailureRate
  expr: rate(task_finished_total{status="failed"}[5m]) > 10
  for: 5m
```

---

## 10. 实施路线图

### 10.1 第一阶段（MVP - 4周）

**核心功能：**
- Scheduler Master基础框架（单实例）
- Agent基础框架（GPU检测、心跳、任务执行）
- 简单FIFO调度算法
- 配额管理（静态配置）
- REST API基本接口（提交任务、查询状态）
- gRPC通信（Agent <-> Scheduler）
- 结构化日志

**交付物：**
- 可运行的原型系统
- 支持基本的任务调度和执行
- 基础文档

### 10.2 第二阶段（HA + 优化 - 3周）

**增强功能：**
- 主备切换机制
- 状态持久化和恢复
- 配额动态调整
- 任务失败重试
- Agent容错处理
- 完善REST API（取消任务、列表查询）

**交付物：**
- 高可用调度系统
- 运维手册

### 10.3 第三阶段（可观测性 - 2周）

**监控增强：**
- Prometheus指标暴露
- Grafana Dashboard
- 告警规则配置
- 日志聚合和查询

**交付物：**
- 完整监控方案
- 告警手册

### 10.4 第四阶段（高级特性 - 按需）

**可选扩展：**
- 自动扩缩容
- GPU类型感知调度
- 配额超售和抢占
- 多租户支持
- Web UI控制台

---

## 11. 技术选型

### 11.1 编程语言

**推荐：Go**
- 原生并发支持（goroutine）
- 优秀的gRPC生态
- 静态编译，部署简单
- 性能优秀

**备选：Rust**
- 极致性能和内存安全
- 学习曲线较陡

### 11.2 通信协议

- **gRPC**: Agent <-> Scheduler, Scheduler主备同步
- **HTTP REST**: 外部用户API

### 11.3 序列化

- **Protobuf**: gRPC接口定义
- **JSON**: REST API、日志、状态持久化

### 11.4 GPU检测

- **NVML（NVIDIA Management Library）**: 官方Go binding
- **nvidia-smi**: 命令行工具（备选）

### 11.5 日志库

- **zap** (Go): 高性能结构化日志
- **logrus** (Go): 备选

### 11.6 配置管理

- **Viper** (Go): 支持多种配置格式（YAML/JSON/TOML）

---

## 12. 部署架构

### 12.1 组件部署

**Scheduler Master:**
```
主节点: scheduler-master-1 (Active)
备节点: scheduler-master-2 (Standby)
部署方式: Docker容器 / 二进制
配置文件: /etc/dgpu-scheduler/scheduler.yaml
数据目录: /var/lib/dgpu-scheduler/state/
日志目录: /var/log/dgpu-scheduler/
```

**Agent:**
```
每个GPU节点: dgpu-agent
部署方式: Docker容器 / 二进制
配置文件: /etc/dgpu-scheduler/agent.yaml
日志目录: /var/log/dgpu-scheduler/
```

### 12.2 网络规划

- Scheduler Master主备: 互通（gRPC端口9090）
- Agent -> Scheduler Master: gRPC端口9090
- 用户 -> Scheduler Master: HTTP端口8080

### 12.3 存储需求

- Scheduler Master状态快照: 本地磁盘（几MB）
- 共享存储（可选）: NFS用于主备仲裁文件

---

## 13. 风险和缓解措施

### 13.1 风险点

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|----------|
| Scheduler单点故障 | 高 | 中 | 主备切换机制 |
| 状态同步延迟导致数据不一致 | 中 | 中 | 版本号机制 + 定期校验 |
| 网络分区导致脑裂 | 高 | 低 | 共享存储仲裁 |
| Agent大量离线导致资源不足 | 中 | 低 | 配额动态调整 + 告警 |
| GPU硬件故障 | 中 | 中 | 健康检测 + 自动隔离 |

### 13.2 性能瓶颈

**潜在瓶颈：**
- Scheduler调度吞吐量（目标：1000任务/秒）
- 主备状态同步延迟（目标：<100ms）
- Agent心跳风暴（100个Agent同时心跳）

**优化方向：**
- 批量处理任务队列
- 异步状态��步
- 心跳时间抖动（避免同时心跳）

---

## 14. 测试策略

### 14.1 单元测试

- 调度算法逻辑
- 配额计算逻辑
- 状态管理模块

### 14.2 集成测试

- Agent与Scheduler通信
- 主备切换流程
- 任务完整生命周期

### 14.3 性能测试

- 并发任务提交（1000任务/秒）
- 大规模Agent心跳（100节点）
- 主备同步延迟

### 14.4 故障注入测试

- Scheduler主节点崩溃
- Agent随机离线
- 网络分区模拟

---

## 15. 总结

本设计文档定义了DGPU Scheduler分布式调度系统的完整架构方案，采用务实的技术选型和渐进式迭代策略：

**核心特点：**
- 集中式调度（主备模式）确保全局视图和配额控制
- 无外部依赖，降低运维复杂度
- 先日志后指标，快速上线
- 第一版静态分配，预留扩展空间

**下一步：**
1. 技术预研（Go + gRPC原型验证）
2. 详细实施计划编写
3. MVP开发（4周）
4. 生产环境试点部署

---

**变更记录：**

| 版本 | 日期 | 作者 | 变更说明 |
|------|------|------|----------|
| 1.0 | 2025-12-14 | Claude | 初始版本 |
