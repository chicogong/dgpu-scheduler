# 贡献指南

感谢你对 DGPU Scheduler 项目的关注！我们欢迎任何形式的贡献，无论是错误报告、功能建议、文档改进还是代码贡献。

## 📋 开始之前

在贡献之前，请：

1. 🔍 搜索现有的 [Issues](https://github.com/chicogong/dgpu-scheduler/issues) 和 [Pull Requests](https://github.com/chicogong/dgpu-scheduler/pulls)，避免重复工作
2. 📖 阅读 [设计文档](../docs/plans/2025-12-14-dgpu-scheduler-design.md) 了解项目架构
3. 📝 查看 [开发指南](../docs/DEVELOPMENT.md) 了解开发环境搭建

## 🐛 报告 Bug

发现问题？请帮助我们改进！

### Bug 报告模板

```markdown
**Bug 描述**
简洁清晰地描述问题

**复现步骤**
1. 执行 '...'
2. 点击 '....'
3. 滚动到 '....'
4. 看到错误

**期望行为**
描述你期望发生什么

**实际行为**
描述实际发生了什么

**环境信息**
- OS: [例如 Ubuntu 20.04]
- Go 版本: [例如 1.21.0]
- 项目版本: [例如 v1.0.0]
- GPU 信息: [例如 NVIDIA V100]

**日志输出**
```
粘贴相关日志
```

**额外上下文**
添加其他有助于问题理解的信息
```

## 💡 功能建议

有新想法？我们很乐意听到！

### 功能建议模板

```markdown
**功能描述**
简洁清晰地描述你想要的功能

**问题背景**
描述这个功能要解决什么问题

**解决方案**
详细描述你希望的实现方式

**替代方案**
描述你考虑过的其他解决方案

**额外上下文**
添加其他相关信息，如截图、链接等
```

## 🔧 代码贡献

### 开发流程

1. **Fork 项目**
   ```bash
   # 在 GitHub 上点击 Fork 按钮
   git clone https://github.com/your-username/dgpu-scheduler.git
   cd dgpu-scheduler
   git remote add upstream https://github.com/chicogong/dgpu-scheduler.git
   ```

2. **创建分支**
   ```bash
   git checkout -b feature/your-feature-name
   # 或者
   git checkout -b fix/your-bug-fix
   ```

3. **开发环境搭建**
   ```bash
   # 安装依赖
   make deps

   # 生成 protobuf 代码
   make proto

   # 构建项目
   make build

   # 运行测试
   make test
   ```

4. **进行开发**
   - 遵循 [Go 编码规范](https://golang.org/doc/effective_go.html)
   - 为新功能添加测试
   - 更新相关文档
   - 确保代码通过所有测试

5. **提交代码**
   ```bash
   # 格式化代码
   make fmt

   # 运行代码检查
   make lint

   # 运行测试
   make test

   # 提交更改
   git add .
   git commit -m "feat: add GPU affinity scheduling"
   ```

6. **推送并创建 PR**
   ```bash
   git push origin feature/your-feature-name
   # 在 GitHub 上创建 Pull Request
   ```

### 提交信息规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**类型 (type)：**
- `feat`: 新功能
- `fix`: 错误修复
- `docs`: 文档更新
- `style`: 代码格式（不影响功能）
- `refactor`: 重构（既不修复错误也不添加功能）
- `test`: 添加测试
- `chore`: 构建过程或辅助工具的变动

**示例：**
```
feat(scheduler): add GPU affinity scheduling

Add support for GPU affinity in task scheduling to improve
performance for multi-GPU workloads.

Closes #123
```

### 代码规范

#### Go 代码风格

1. **格式化**：使用 `gofmt` 格式化代码
2. **命名**：
   - 包名：小写，简洁
   - 函数/变量：驼峰命名
   - 常量：全大写，下划线分隔
   - 导出函数：首字母大写

3. **错误处理**：
   ```go
   // ✅ 正确
   if err != nil {
       return fmt.Errorf("failed to process task: %w", err)
   }

   // ❌ 错误
   if err != nil {
       panic(err)
   }
   ```

4. **日志记录**：
   ```go
   // ✅ 使用结构化日志
   log.Info("Task scheduled",
       zap.String("task_id", task.ID),
       zap.Int("gpu_count", task.GPUCount),
   )

   // ❌ 避免使用 fmt.Printf
   fmt.Printf("Task %s scheduled\n", task.ID)
   ```

#### 测试规范

1. **测试文件命名**：`*_test.go`
2. **测试函数命名**：`TestFunctionName` 或 `TestStructName_MethodName`
3. **使用表驱动测试**：
   ```go
   func TestQuotaCheck(t *testing.T) {
       tests := []struct {
           name     string
           task     *Task
           quota    *Quota
           expected bool
       }{
           {
               name: "sufficient quota",
               task: &Task{Priority: PriorityHigh, GPUCount: 2},
               quota: &Quota{OnlineQuota: 10, OnlineUsed: 5},
               expected: true,
           },
           // 更多测试用例...
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               result := CanScheduleTask(tt.task, tt.quota)
               assert.Equal(t, tt.expected, result)
           })
       }
   }
   ```

### Pull Request 检查清单

在提交 PR 之前，请确保：

- [ ] 代码已通过 `make fmt` 格式化
- [ ] 代码已通过 `make lint` 检查
- [ ] 所有测试已通过 `make test`
- [ ] 新功能已添加相应测试
- [ ] 文档已更新（如果适用）
- [ ] Commit 信息遵循规范
- [ ] PR 描述清楚说明了变更内容

### PR 模板

```markdown
## 变更类型
- [ ] Bug 修复
- [ ] 新功能
- [ ] 性能改进
- [ ] 重构
- [ ] 文档更新
- [ ] 其他: ________

## 描述
简洁地描述这次变更

## 相关 Issue
Closes #(issue)

## 测试
描述你如何测试了这些变更

## 检查清单
- [ ] 代码已格式化
- [ ] 通过了 lint 检查
- [ ] 添加/更新了测试
- [ ] 更新了文档
- [ ] 本地测试通过

## 截图（如果适用）
添加截图帮助解释你的变更
```

## 🏷️ 发布流程

我们使用语义版本控制（SemVer）：

- **MAJOR** version when you make incompatible API changes
- **MINOR** version when you add functionality in a backwards compatible manner
- **PATCH** version when you make backwards compatible bug fixes

发布流程：

1. 更新 CHANGELOG.md
2. 创建版本标签：`git tag -a v1.0.0 -m "Release v1.0.0"`
3. 推送标签：`git push upstream v1.0.0`
4. GitHub Actions 会自动构建和发布

## 📞 联系方式

有问题？可以通过以下方式联系我们：

- 🐛 [提交 Issue](https://github.com/chicogong/dgpu-scheduler/issues)
- 💬 [GitHub Discussions](https://github.com/chicogong/dgpu-scheduler/discussions)
- 📧 [Email](mailto:your-email@example.com)

## 📄 许可证

通过贡献代码，你同意你的贡献将在 MIT 许可证下授权。

---

**感谢你的贡献！** 🎉

每一个贡献，无论大小，都能让这个项目变得更好。