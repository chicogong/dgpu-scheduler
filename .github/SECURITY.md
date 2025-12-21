# Security Policy

## 🛡️ 安全承诺

DGPU Scheduler 项目致力于维护用户和系统的安全。我们非常重视安全问题，并会迅速响应和修复安全漏洞。

## 🚨 报告安全漏洞

### 优先级分类

**🔴 严重漏洞**
- 远程代码执行
- 权限提升
- 数据泄露
- 服务拒绝攻击

**🟡 中等漏洞**
- 信息泄露
- 访问控制绕过
- 配置问题

**🔵 低级漏洞**
- 信息收集
- 日志注入

### 报告流程

如果你发现了安全漏洞，请**不要**在公开的 GitHub Issues 中报告。请通过以下安全渠道报告：

1. **邮件报告**：security@dgpu-scheduler.io
2. **加密报告**：使用我们的 [PGP 公钥](https://keybase.io/dgpu_scheduler)

### 报告内容

请在安全报告中包含以下信息：

- 漏洞类型和严重程度
- 详细的复现步骤
- 受影响的版本范围
- 可能的影响和危害
- 建议的修复方案（如有）
- 你的联系方式

### 响应时间表

| 严重程度 | 初始响应 | 状态更新 | 修复发布 |
|---------|---------|---------|---------|
| 严重     | 24小时   | 48小时   | 7天内   |
| 中等     | 72小时   | 1周     | 30天内  |
| 低级     | 1周     | 2周     | 90天内  |

## 🔒 安全最佳实践

### 部署安全

1. **网络隔离**
   ```yaml
   # 使用防火墙规则限制访问
   # 只允许必要的端口（8080, 9090）
   # 使用 VPC/子网隔离
   ```

2. **身份认证**
   ```yaml
   # 配置 TLS 证书
   server:
     tls:
       enabled: true
       cert_file: "/path/to/cert.pem"
       key_file: "/path/to/key.pem"
   ```

3. **访问控制**
   ```yaml
   # 配置 API 密钥
   api:
     auth:
       enabled: true
       api_keys:
         - name: "admin"
           key: "your-secure-api-key"
           permissions: ["read", "write", "admin"]
   ```

### 容器安全

1. **非特权运行**
   ```dockerfile
   USER scheduler:scheduler
   # 避免使用 root 用户
   ```

2. **资源限制**
   ```yaml
   resources:
     limits:
       memory: "512Mi"
       cpu: "500m"
   ```

3. **安全上下文**
   ```yaml
   securityContext:
     runAsNonRoot: true
     readOnlyRootFilesystem: true
     allowPrivilegeEscalation: false
   ```

### 配置安全

1. **敏感信息管理**
   ```bash
   # 使用环境变量或密钥管理系统
   export SCHEDULER_API_KEY="$(cat /run/secrets/api_key)"
   ```

2. **文件权限**
   ```bash
   chmod 600 configs/scheduler.yaml
   chown scheduler:scheduler configs/scheduler.yaml
   ```

3. **日志安全**
   ```yaml
   logging:
     level: "info"  # 避免 debug 级别泄露敏感信息
     format: "json"
     sanitize_fields: ["password", "token", "key"]
   ```

## 🔍 安全检查

### 定期安全审计

1. **依赖扫描**
   ```bash
   go list -json -deps ./... | nancy sleuth
   govulncheck ./...
   ```

2. **容器镜像扫描**
   ```bash
   trivy image dgpu-scheduler:latest
   ```

3. **静态代码分析**
   ```bash
   gosec ./...
   ```

### 监控和告警

1. **异常访问监控**
   - 失败的认证尝试
   - 异常的 API 调用模式
   - 资源使用异常

2. **安全事件日志**
   ```json
   {
     "level": "warning",
     "event": "auth_failure",
     "ip": "192.168.1.100",
     "user_agent": "curl/7.68.0",
     "timestamp": "2024-12-21T10:30:00Z"
   }
   ```

## 🛠️ 安全更新

### 自动更新

我们建议启用自动安全更新：

```yaml
# Kubernetes 环境
spec:
  template:
    spec:
      containers:
      - name: scheduler
        image: dgpu-scheduler:latest
        imagePullPolicy: Always
```

### 更新通知

- **安全公告**：发布在 GitHub Security Advisories
- **邮件通知**：订阅 security@dgpu-scheduler.io
- **RSS 订阅**：关注 [安全更新 RSS](https://github.com/chicogong/dgpu-scheduler/security/advisories.atom)

## 📋 支持的版本

| 版本 | 支持状态 | 安全更新 |
|-----|---------|---------|
| 0.1.x | ✅ 支持 | ✅ 是   |
| 开发版 | ⚠️ 测试 | ❌ 否   |

## 🏆 安全致谢

我们感谢以下安全研究人员的贡献：

<!--
安全研究人员名单将在这里更新
格式：
- [研究人员姓名](GitHub链接) - 发现的漏洞类型 (日期)
-->

*目前还没有安全漏洞报告*

## 📚 相关资源

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Kubernetes 安全最佳实践](https://kubernetes.io/docs/concepts/security/)
- [Docker 安全指南](https://docs.docker.com/engine/security/)
- [Go 安全编码规范](https://github.com/securego/gosec)

## 📞 联系我们

- 安全邮箱：security@dgpu-scheduler.io
- 项目主页：https://github.com/chicogong/dgpu-scheduler
- 安全政策：https://github.com/chicogong/dgpu-scheduler/security/policy

---

**请记住：安全是每个人的责任** 🛡️