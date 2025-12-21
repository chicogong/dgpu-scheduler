# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### 规划中
- 高可用功能（主备复制、故障切换）
- Prometheus 指标集成
- Web UI 管理界面
- 任务优先级动态调整
- GPU 亲和性调度

## [0.1.0] - 2024-12-21

### Added
- 🎉 项目初始版本
- ⚙️ 核心调度器实现
  - FIFO + 优先级队列调度算法
  - 配额感知的资源管理
  - 事件驱动 + 定期扫描调度
- 🖥️ GPU Agent 功能
  - NVML 和 nvidia-smi GPU 检测
  - Docker 和 process 任务执行
  - 自动心跳和重连机制
- 🌐 双重 API 支持
  - HTTP REST API（用户接口）
  - gRPC API（Agent 内部通信）
- 📊 完整的状态管理
  - 内存状态存储
  - 定期快照持久化（30秒周期）
  - 故障恢复机制
- 📈 性能优化
  - 微秒级调度延迟（4-17μs）
  - O(n) 算法复杂度
  - 高并发支持
- 🧪 测试框架
  - 单元测试覆盖核心逻辑
  - 集成测试支持
  - 本地开发测试环境
- 📖 完整文档
  - 中英双语 README
  - 详细的设计文档
  - 开发指南和 API 文档
- 🐳 容器化支持
  - 多阶段 Docker 构建
  - Docker Compose 配置
  - Kubernetes 部署清单
- ⚡ CI/CD 流水线
  - GitHub Actions 自动化
  - 代码质量检查
  - 安全扫描
  - 多平台构建发布

### Technical Specifications
- **调度性能**: 4-17微秒调度延迟
- **集群规模**: 支持 50-200 GPU 节点
- **并发能力**: 设计吞吐量 1000 任务/秒
- **容错性**: Agent 自动重连，调度器快照恢复
- **配额管理**: 可配置在线/离线资源分配比例

### API Endpoints
- `POST /api/v1/tasks` - 提交任务
- `GET /api/v1/tasks/{id}` - 查询任务状态
- `DELETE /api/v1/tasks/{id}` - 取消任务
- `GET /api/v1/tasks` - 列出任务（支持过滤）
- `GET /api/v1/gpus` - 查询 GPU 资源
- `GET /api/v1/quota` - 查询配额状态
- `PUT /api/v1/quota` - 更新配额比例

### Configuration
- 调度器配置：端口、角色、配额、复制设置
- Agent 配置：GPU 检测、任务执行、心跳设置
- 本地测试配置：模拟 GPU 环境

[Unreleased]: https://github.com/chicogong/dgpu-scheduler/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/chicogong/dgpu-scheduler/releases/tag/v0.1.0