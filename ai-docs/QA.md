# PicoClaw 核心技术实现详解

本文档详细介绍了 PicoClaw 项目的两个核心技术的实现原理：消息总线（Message Bus）和沙盒环境（Sandbox）。

---

## 一、消息总线（Message Bus）工作原理

### 1.1 核心数据结构

消息总线的核心定义在 `pkg/bus/bus.go` 中：

```go
type MessageBus struct {
    inbound  chan InboundMessage    // 入站消息通道（容量 100）
    outbound chan OutboundMessage   // 出站消息通道（容量 100）
    handlers map[string]MessageHandler
    closed   bool
    mu       sync.RWMutex
}
```

### 1.2 消息类型定义（pkg/bus/types.go）

```go
// 入站消息：从外部渠道收到用户消息
type InboundMessage struct {
    Channel    string            // 渠道名称：telegram, discord, qq 等
    SenderID   string            // 发送者 ID
    ChatID     string            // 会话 ID
    Content    string            // 消息内容
    Media      []string          // 附件列表
    SessionKey string            // 会话键
    Metadata   map[string]string // 附加信息
}

// 出站消息：发送给用户的响应
type OutboundMessage struct {
    Channel string // 目标渠道
    ChatID  string // 目标会话
    Content string // 消息内容
}
```

### 1.3 消息流转完整流程

```
用户发送消息
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  1. Channel 接收消息                                                         │
│     Telegram/Discord/QQ 等 channel 收到外部平台的消息                         │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  2. 解析并构建 InboundMessage                                                │
│     - 解析消息内容、发送者 ID、会话 ID                                        │
│     - 添加元数据（群组信息、用户信息等）                                       │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  3. 调用 HandleMessage() → bus.PublishInbound()                             │
│     BaseChannel.HandleMessage() (pkg/channels/base.go:84-99)                │
│     代码:                                                                    │
│     msg := bus.InboundMessage{...}                                          │
│     c.bus.PublishInbound(msg)                                               │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼ 写入 inbound channel
    │
┌─────────────────────────────────────────────────────────────────────────────┐
│  4. MessageBus 接收                                                         │
│     mb.inbound <- msg (有缓冲，容量 100)                                    │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  5. Agent Loop 消费消息                                                      │
│     agent_loop.go:166                                                       │
│     msg, ok := al.bus.ConsumeInbound(ctx)                                  │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  6. Agent 处理消息                                                          │
│     - 路由决策 → 确定使用哪个 Agent                                          │
│     - 加载会话历史                                                           │
│     - 调用 LLM + 执行工具                                                    │
│     - 返回响应                                                               │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  7. Agent 发布响应 → bus.PublishOutbound()                                  │
│     agent_loop.go:191                                                       │
│     al.bus.PublishOutbound(bus.OutboundMessage{...})                       │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼ 写入 outbound channel
    │
┌─────────────────────────────────────────────────────────────────────────────┐
│  8. Channel Manager 订阅出站消息                                             │
│     manager.go:280                                                          │
│     msg, ok := m.bus.SubscribeOutbound(ctx)                                │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  9. 分发到对应 Channel                                                       │
│     manager.go:301                                                          │
│     channel.Send(ctx, msg)                                                  │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  10. Channel 发送消息给用户                                                  │
│      例如：telegram.go:149 Send() 方法调用 Telegram API                     │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
用户收到响应
```

### 1.4 关键代码解析

#### 1.4.1 创建消息总线

```go
// pkg/bus/bus.go:16-22
func NewMessageBus() *MessageBus {
    return &MessageBus{
        inbound:  make(chan InboundMessage, 100),  // 有缓冲，避免阻塞
        outbound: make(chan OutboundMessage, 100),
        handlers: make(map[string]MessageHandler),
    }
}
```

#### 1.4.2 发布入站消息（Channel → Bus）

```go
// pkg/bus/bus.go:24-31
func (mb *MessageBus) PublishInbound(msg InboundMessage) {
    mb.mu.RLock()
    defer mb.mu.RUnlock()
    if mb.closed {
        return
    }
    mb.inbound <- msg  // 写入入站通道
}
```

#### 1.4.3 消费入站消息（Bus → Agent）

```go
// pkg/bus/bus.go:33-40
func (mb *MessageBus) ConsumeInbound(ctx context.Context) (InboundMessage, bool) {
    select {
    case msg := <-mb.inbound:  // 从入站通道读取
        return msg, true
    case <-ctx.Done():  // 支持上下文取消
        return InboundMessage{}, false
    }
}
```

#### 1.4.4 Channel 发送消息到总线（BaseChannel）

```go
// pkg/channels/base.go:84-99
func (c *BaseChannel) HandleMessage(senderID, chatID, content string, media []string, metadata map[string]string) {
    if !c.IsAllowed(senderID) {
        return  // 白名单检查
    }

    msg := bus.InboundMessage{
        Channel:  c.name,
        SenderID: senderID,
        ChatID:   chatID,
        Content:  content,
        Media:    media,
        Metadata: metadata,
    }

    c.bus.PublishInbound(msg)  // 发布到总线
}
```

#### 1.4.5 Agent Loop 处理消息

```go
// pkg/agent/loop.go:158-202
func (al *AgentLoop) Run(ctx context.Context) error {
    al.running.Store(true)

    for al.running.Load() {
        select {
        case <-ctx.Done():
            return nil
        default:
            msg, ok := al.bus.ConsumeInbound(ctx)  // 消费消息
            if !ok {
                continue
            }

            response, err := al.processMessage(ctx, msg)  // 处理消息
            // ...
            
            if response != "" {
                al.bus.PublishOutbound(bus.OutboundMessage{  // 发布响应
                    Channel: msg.Channel,
                    ChatID:  msg.ChatID,
                    Content: response,
                })
            }
        }
    }
    return nil
}
```

#### 1.4.6 Channel Manager 分发出站消息

```go
// pkg/channels/manager.go:271-309
func (m *Manager) dispatchOutbound(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
            msg, ok := m.bus.SubscribeOutbound(ctx)  // 订阅出站消息
            if !ok {
                continue
            }

            // 跳过内部渠道
            if constants.IsInternalChannel(msg.Channel) {
                continue
            }

            m.mu.RLock()
            channel, exists := m.channels[msg.Channel]
            m.mu.RUnlock()

            if !exists {
                continue
            }

            // 发送到对应渠道
            if err := channel.Send(ctx, msg); err != nil {
                // 错误处理
            }
        }
    }
}
```

### 1.5 完整的初始化流程（gatewayCmd）

```go
// cmd/picoclaw/cmd_gateway.go:31-207

func gatewayCmd() {
    // 1. 加载配置
    cfg, err := loadConfig()
    
    // 2. 创建 LLM Provider
    provider, modelID, err := providers.CreateProvider(cfg)
    
    // 3. 创建消息总线 ⭐
    msgBus := bus.NewMessageBus()
    
    // 4. 创建 Agent Loop（传入 msgBus）
    agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)
    
    // 5. 设置 Cron 和 Heartbeat（传入 msgBus）
    cronService := setupCronTool(agentLoop, msgBus, ...)
    heartbeatService.SetBus(msgBus)
    
    // 6. 创建 Channel Manager（传入 msgBus）
    channelManager, err := channels.NewManager(cfg, msgBus)
    
    // 7. 启动所有 Channel
    channelManager.StartAll(ctx)
    
    // 8. 启动 Agent Loop（开始消费消息）
    go agentLoop.Run(ctx)
    
    // 9. 等待信号退出
}
```

### 1.6 设计特点总结

| 特点 | 说明 |
|------|------|
| **有缓冲通道** | inbound/outbound 都是带缓冲的 channel（容量 100），避免阻塞 |
| **上下文支持** | 所有操作都支持 context.Context，可优雅取消 |
| **读写锁保护** | 使用 sync.RWMutex 保护 Publish 操作，防止并发问题 |
| **线程安全** | Channel 的注册和查询都有锁保护 |
| **内部渠道过滤** | 出站时自动跳过 cli、system、subagent 等内部渠道 |
| **松耦合** | 各组件通过消息总线通信，互不直接依赖 |

---

## 二、沙盒环境（Sandbox）实现原理

### 2.1 整体架构

沙盒功能通过以下三个层面实现：

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           沙盒实现架构                                       │
└─────────────────────────────────────────────────────────────────────────────┘

配置层 (config)
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  agents.defaults.restrict_to_workspace = true (默认)                       │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
工具层 (tools)
    │
    │
    ├──────────────────┬──────────────────┬──────────────────┐
    ▼                  ▼                  ▼                  ▼
┌────────┐       ┌────────┐       ┌────────┐       ┌────────┐
│文件读取 │       │文件写入 │       │目录列表 │       │命令执行 │
│read_   │       │write_  │       │list_   │       │exec    │
│file    │       │file    │       │dir     │       │        │
└────────┘       └────────┘       └────────┘       └────────┘
    │                  │                  │                  │
    └──────────────────┴──────────────────┴──────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  sandboxFs (os.Root)     vs     hostFs (原生文件系统)                       │
│  - 使用 Go 1.22+ 的        - 无限制                                        │
│    os.Root 实现                                                           │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 配置定义

#### 2.2.1 配置结构（pkg/config/config.go:169-181）

```go
type AgentDefaults struct {
    Workspace           string   `json:"workspace" env:"PICOCLAW_AGENTS_DEFAULTS_WORKSPACE"`
    RestrictToWorkspace bool     `json:"restrict_to_workspace" env:"PICOCLAW_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE"`
    // ... 其他字段
}
```

**默认值为 `true`**：默认启用工作区限制。

### 2.3 核心实现代码

#### 2.3.1 文件操作工具的沙盒实现（pkg/tools/filesystem.go）

##### 工具创建时决定使用哪种文件系统

```go
// pkg/tools/filesystem.go:88-96
func NewReadFileTool(workspace string, restrict bool) *ReadFileTool {
    var fs fileSystem
    if restrict {
        // 使用沙盒文件系统
        fs = &sandboxFs{workspace: workspace}
    } else {
        // 使用原生文件系统
        fs = &hostFs{}
    }
    return &ReadFileTool{fs: fs}
}
```

**关键点**：`restrict` 参数决定使用哪种文件系统。

##### 沙盒文件系统实现（使用 os.Root）

```go
// pkg/tools/filesystem.go:300-383

// sandboxFs 使用 os.Root 实现沙盒
type sandboxFs struct {
    workspace string
}

// 核心执行方法：使用 os.OpenRoot 打开工作区
func (r *sandboxFs) execute(path string, fn func(root *os.Root, relPath string) error) error {
    // 1. 打开工作区作为根目录
    root, err := os.OpenRoot(r.workspace)
    if err != nil {
        return fmt.Errorf("failed to open workspace: %w", err)
    }
    defer root.Close()

    // 2. 计算相对路径
    relPath, err := getSafeRelPath(r.workspace, path)
    if err != nil {
        return err
    }

    // 3. 在沙盒内执行操作
    return fn(root, relPath)
}

// 沙盒内读取文件
func (r *sandboxFs) ReadFile(path string) ([]byte, error) {
    return r.execute(path, func(root *os.Root, relPath string) error {
        // os.Root 会自动阻止超出工作区的文件访问
        fileContent, err := root.ReadFile(relPath)
        if err != nil {
            // "escapes from parent" 是 os.Root 返回的错误
            if strings.Contains(err.Error(), "escapes from parent") {
                return fmt.Errorf("access denied: path outside workspace")
            }
        }
        content = fileContent
        return nil
    })
}

// 沙盒内写入文件
func (r *sandboxFs) WriteFile(path string, data []byte) error {
    return r.execute(path, func(root *os.Root, relPath string) error {
        // 在沙盒内创建目录和写入文件
        // ...
    })
}

// 沙盒内读取目录
func (r *sandboxFs) ReadDir(path string) ([]os.DirEntry, error) {
    return r.execute(path, func(root *os.Root, relPath string) error {
        // 在沙盒内读取目录
    })
}
```

##### 路径验证辅助函数

```go
// pkg/tools/filesystem.go:14-82

// validatePath 验证路径是否在工作区内
func validatePath(path, workspace string, restrict bool) (string, error) {
    // 1. 获取工作区的绝对路径
    absWorkspace, err := filepath.Abs(workspace)
    
    // 2. 获取请求路径的绝对路径
    absPath, err := filepath.Abs(filepath.Join(absWorkspace, path))
    
    if restrict {
        // 3. 检查是否在工作区内
        if !isWithinWorkspace(absPath, absWorkspace) {
            return "", fmt.Errorf("access denied: path is outside the workspace")
        }

        // 4. 处理符号链接（防止通过符号链接逃逸）
        if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
            if !isWithinWorkspace(resolved, absWorkspace) {
                return "", fmt.Errorf("access denied: symlink resolves outside workspace")
            }
        }
    }
    return absPath, nil
}

// isWithinWorkspace 检查路径是否在工作区内
func isWithinWorkspace(candidate, workspace string) bool {
    rel, err := filepath.Rel(filepath.Clean(workspace), filepath.Clean(candidate))
    // 检查相对路径是否以 ".." 开头（逃逸）
    return err == nil && filepath.IsLocal(rel)
}
```

#### 2.3.2 命令执行工具的沙盒实现（pkg/tools/shell.go）

##### ExecTool 结构

```go
// pkg/tools/shell.go:19-25
type ExecTool struct {
    workingDir          string           // 工作目录
    timeout             time.Duration   // 超时时间
    denyPatterns        []*regexp.Regexp // 危险命令模式
    allowPatterns       []*regexp.Regexp // 允许命令模式
    restrictToWorkspace bool             // 是否限制工作区
}
```

##### 危险命令模式列表

```go
// pkg/tools/shell.go:27-70
var defaultDenyPatterns = []*regexp.Regexp{
    // 批量删除
    regexp.MustCompile(`\brm\s+-[rf]{1,2}\b`),
    regexp.MustCompile(`\bdel\s+/[fq]\b`),
    regexp.MustCompile(`\brmdir\s+/s\b`),
    
    // 磁盘格式化
    regexp.MustCompile(`\b(format|mkfs|diskpart)\b\s`),
    
    // 磁盘成像
    regexp.MustCompile(`\bdd\s+if=`),
    
    // 直接磁盘写入
    regexp.MustCompile(`>\s*/dev/sd[a-z]\b`),
    
    // 系统关机
    regexp.MustCompile(`\b(shutdown|reboot|poweroff)\b`),
    
    // Fork 炸弹
    regexp.MustCompile(`:\(\)\s*\{.*\};\s*:`),
    
    // 命令注入
    regexp.MustCompile(`\$\([^)]+\)`),      // $(...)
    regexp.MustCompile(`\$\{[^}]+\}`),       // ${...}
    regexp.MustCompile("`[^`]+`"),           // `...`
    regexp.MustCompile(`\|\s*sh\b`),        // | sh
    regexp.MustCompile(`\|\s*bash\b`),      // | bash
    
    // 提权
    regexp.MustCompile(`\bsudo\b`),
    regexp.MustCompile(`\bchmod\s+[0-7]{3,4}\b`),
    regexp.MustCompile(`\bchown\b`),
    
    // 进程管理
    regexp.MustCompile(`\bpkill\b`),
    regexp.MustCompile(`\bkillall\b`),
    
    // 远程访问
    regexp.MustCompile(`\bssh\b.*@`),
    
    // 其他危险操作
    regexp.MustCompile(`\beval\b`),
    regexp.MustCompile(`\bdocker\s+run\b`),
    regexp.MustCompile(`\bdocker\s+exec\b`),
    regexp.MustCompile(`\bgit\s+push\b`),
}
```

##### 命令验证逻辑

```go
// pkg/tools/shell.go:259-313

func (t *ExecTool) guardCommand(command, cwd string) string {
    cmd := strings.TrimSpace(command)
    lower := strings.ToLower(cmd)

    // 1. 检查危险命令模式
    for _, pattern := range t.denyPatterns {
        if pattern.MatchString(lower) {
            return "Command blocked by safety guard (dangerous pattern detected)"
        }
    }

    // 2. 如果配置了白名单，检查是否在白名单中
    if len(t.allowPatterns) > 0 {
        allowed := false
        for _, pattern := range t.allowPatterns {
            if pattern.MatchString(lower) {
                allowed = true
                break
            }
        }
        if !allowed {
            return "Command blocked by safety guard (not in allowlist)"
        }
    }

    // 3. 如果限制工作区，检查路径
    if t.restrictToWorkspace {
        // 3.1 防止路径遍历攻击
        if strings.Contains(cmd, "..\\") || strings.Contains(cmd, "../") {
            return "Command blocked by safety guard (path traversal detected)"
        }

        // 3.2 提取命令中的所有路径
        pathPattern := regexp.MustCompile(`[A-Za-z]:\\[^\\\"']+|/[^\s\"']+`)
        matches := pathPattern.FindAllString(cmd, -1)

        // 3.3 检查每个路径是否在工作区内
        for _, raw := range matches {
            p, err := filepath.Abs(raw)
            if err != nil {
                continue
            }

            rel, err := filepath.Rel(cwdPath, p)
            if err != nil {
                continue
            }

            // 如果相对路径以 ".." 开头，说明在工作区外
            if strings.HasPrefix(rel, "..") {
                return "Command blocked by safety guard (path outside working dir)"
            }
        }
    }

    return ""
}
```

#### 2.3.3 工具注册（pkg/agent/instance.go）

```go
// pkg/agent/instance.go:37-56
func NewAgentInstance(
    agentCfg *config.AgentConfig,
    defaults *config.AgentDefaults,
    cfg *config.Config,
    provider providers.LLMProvider,
) *AgentInstance {
    // 1. 解析工作区路径
    workspace := resolveAgentWorkspace(agentCfg, defaults)
    
    // 2. 获取是否启用限制
    restrict := defaults.RestrictToWorkspace  // 默认 true
    
    // 3. 创建工具注册表
    toolsRegistry := tools.NewToolRegistry()
    
    // 4. 注册带沙盒限制的工具
    toolsRegistry.Register(tools.NewReadFileTool(workspace, restrict))
    toolsRegistry.Register(tools.NewWriteFileTool(workspace, restrict))
    toolsRegistry.Register(tools.NewListDirTool(workspace, restrict))
    toolsRegistry.Register(tools.NewExecToolWithConfig(workspace, restrict, cfg))
    toolsRegistry.Register(tools.NewEditFileTool(workspace, restrict))
    toolsRegistry.Register(tools.NewAppendFileTool(workspace, restrict))
    
    // ...
}
```

### 2.4 完整执行流程（以读取文件为例）

```
用户请求读取 /etc/passwd
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  1. LLM 调用 read_file 工具                                                │
│     参数: path = "/etc/passwd"                                            │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  2. ReadFileTool.Execute() 执行                                            │
│     - 传入 workspace = "~/.picoclaw/workspace"                              │
│     - 传入 restrict = true                                                 │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  3. sandboxFs.ReadFile() 执行                                               │
│     - os.OpenRoot(workspace) 打开工作区作为根                               │
│     - getSafeRelPath() 计算相对路径                                        │
│       - absWorkspace = "/home/user/.picoclaw/workspace"                   │
│       - absPath = "/etc/passwd"                                            │
│       - rel, err = Rel("/home/user/.picoclaw/workspace", "/etc/passwd")   │
│       - rel = "../.." （不是本地路径！）                                    │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  4. 路径验证失败                                                            │
│     - isWithinWorkspace("/etc/passwd", workspace) = false                  │
│     - 返回错误: "access denied: path is outside the workspace"            │
└─────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  5. 工具返回错误给 LLM                                                     │
│     ToolResult { ForLLM: "access denied: path is outside the workspace" } │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.5 安全特性总结

| 层面 | 技术实现 | 说明 |
|------|---------|------|
| **文件系统** | `os.Root` (Go 1.22+) | 使用操作系统级沙盒，无法逃逸 |
| **符号链接** | `filepath.EvalSymlinks` | 防止通过符号链接绕过限制 |
| **路径遍历** | `filepath.IsLocal` | 拒绝包含 `..` 的路径 |
| **命令注入** | 正则表达式黑名单 | 阻止 40+ 种危险命令模式 |
| **路径限制** | 路径白名单验证 | 命令中的所有路径必须在工作区内 |

### 2.6 配置示例

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.picoclaw/workspace",
      "restrict_to_workspace": true
    }
  }
}
```

**禁用沙盒（不推荐）**：

```json
{
  "agents": {
    "defaults": {
      "restrict_to_workspace": false
    }
  }
}
```

**环境变量方式**：

```bash
export PICOCLAW_AGENTS_DEFAULTS_RESTRICT_TO_WORKSPACE=false
```

---

## 三、相关代码文件索引

### 消息总线相关

| 文件路径 | 说明 |
|---------|------|
| `pkg/bus/bus.go` | 消息总线核心实现 |
| `pkg/bus/types.go` | 消息类型定义 |
| `pkg/channels/base.go` | Channel 基类 |
| `pkg/channels/manager.go` | Channel 管理器 |
| `pkg/channels/telegram.go` | Telegram 通道实现 |
| `pkg/agent/loop.go` | Agent 循环处理 |

### 沙盒相关

| 文件路径 | 说明 |
|---------|------|
| `pkg/tools/filesystem.go` | 文件操作沙盒实现 |
| `pkg/tools/shell.go` | 命令执行沙盒实现 |
| `pkg/agent/instance.go` | Agent 实例创建 |
| `pkg/config/config.go` | 配置定义 |

---

## 四、AgentLoop 循环管理机制（Q&A）

### Q: AgentLoop.Run() 消费消息和 runAgentLoop() 执行代理循环这两个是怎么管理循环的？

#### A: PicoClaw 采用了三层循环架构来管理消息处理

##### 1. 第一层循环：Run() - 消息消费循环

```go
// pkg/agent/loop.go:158-202
func (al *AgentLoop) Run(ctx context.Context) error {
    al.running.Store(true)

    for al.running.Load() {  // 检查运行状态
        select {
        case <-ctx.Done():   // 响应上下文取消
            return nil
        default:
            msg, ok := al.bus.ConsumeInbound(ctx)  // 阻塞等待消息
            if !ok {
                continue
            }
            response, err := al.processMessage(ctx, msg)
            // 发布响应...
        }
    }
    return nil
}
```

**特点**：
- 外层无限循环，只要 `running = true` 就持续运行
- 阻塞等待消息（`ConsumeInbound()`）
- 支持上下文取消（`ctx.Done()`）
- 串行处理：一次只处理一条消息

##### 2. 第二层：processMessage() - 消息路由与预处理

```go
func (al *AgentLoop) processMessage(ctx context.Context, msg bus.InboundMessage) (string, error) {
    // 1. 命令处理
    if response, handled := al.handleCommand(ctx, msg); handled {
        return response, nil
    }
    // 2. 路由决策
    route := al.registry.ResolveRoute(...)
    // 3. 调用 runAgentLoop()
    return al.runAgentLoop(ctx, agent, processOptions{...})
}
```

##### 3. 第三层循环：runAgentLoop() → runLLMIteration() - LLM 迭代循环

```go
// pkg/agent/loop.go:475-729
func (al *AgentLoop) runLLMIteration(...) (string, int, error) {
    iteration := 0
    var finalContent string

    // LLM 迭代循环，最多 MaxIterations 次（默认 20）
    for iteration < agent.MaxIterations {
        iteration++

        // 1. 调用 LLM
        response, err := callLLM()
        if err != nil {
            return "", iteration, err
        }

        // 2. 无工具调用？直接返回
        if len(response.ToolCalls) == 0 {
            finalContent = response.Content
            break
        }

        // 3. 有工具调用？执行工具后继续迭代
        for _, tc := range normalizedToolCalls {
            toolResult := agent.Tools.ExecuteWithContext(...)
            messages = append(messages, toolResultMsg)
        }
    }
    return finalContent, iteration, nil
}
```

##### 循环退出条件

| 循环 | 退出条件 |
|------|---------|
| **Run()** | `ctx.Done()` 被调用 或 `running = false` |
| **runLLMIteration()** | 1. LLM 返回无 Tool Calls<br>2. 达到 `MaxIterations` 上限<br>3. LLM 调用出错 |

##### 三层循环流程图

```
Run() 消息消费循环
    │
    ├── ConsumeInbound() 阻塞等待
    │
    ▼
processMessage() 路由决策
    │
    ├── 命令处理
    ├── Agent 路由
    │
    ▼
runAgentLoop() 处理主流程
    │
    ├── 加载会话历史
    ├── runLLMIteration() ←──┐
    │    │                    │
    │    ├── 调用 LLM         │
    │    │                    │
    │    ├── 有 Tool Calls?  ─┼── 执行工具 ──→ 继续迭代
    │    │                    │
    │    ├── 无 Tool Calls?  ─┼── 返回结果
    │    │                    │
    │    └── 达到上限?        ──→ 返回结果
    │
    ▼
返回 finalContent
```

##### 关键设计特点

| 特点 | 说明 |
|------|------|
| **串行处理** | 每条消息处理是串行的，避免并发问题 |
| **有限迭代** | LLM 迭代有上限（默认 20 次），防止无限循环 |
| **上下文压缩** | 遇到上下文溢出时自动压缩历史重试 |
| **状态管理** | 使用 `atomic.Bool` 确保线程安全的启停控制 |
| **会话持久化** | 每条消息处理后都保存会话状态 |

---

## 五、Workspace 文件作用详解

### Q: workspace 中的这些文件有啥作用？

#### A: Workspace 文件是 PicoClaw 上下文感知和个性化服务的核心机制

##### 1. 目录结构

```
~/.picoclaw/workspace/
├── AGENT.md              # Agent 行为指南
├── IDENTITY.md           # Agent 身份定义
├── SOUL.md              # Agent 灵魂/性格
├── USER.md              # 用户信息
├── memory/
│   └── MEMORY.md        # 长期记忆
└── skills/              # 技能插件目录
    ├── weather/SKILL.md
    ├── github/
    └── ...
```

##### 2. 各文件作用

| 文件 | 作用 |
|------|------|
| **AGENT.md** | 定义 Agent 的行为准则和指导原则，告诉 AI 应该如何表现 |
| **IDENTITY.md** | 定义 Agent 的身份、名称、能力、版本、理念等元信息 |
| **SOUL.md** | 定义 Agent 的性格特点和价值观，让 AI 有一个清晰的"人格"设定 |
| **USER.md** | 存储用户偏好设置和个人信息，AI 可以根据这些信息提供个性化服务 |
| **memory/MEMORY.md** | 存储跨会话的重要信息，AI 会在每次对话中加载以便记住重要信息 |
| **skills/\*/SKILL.md** | 技能插件定义，扩展 Agent 的能力 |

##### 3. 加载逻辑（pkg/agent/context.go）

```go
func (cb *ContextBuilder) LoadBootstrapFiles() string {
    bootstrapFiles := []string{
        "AGENTS.md",
        "SOUL.md",
        "USER.md",
        "IDENTITY.md",
    }

    for _, filename := range bootstrapFiles {
        filePath := filepath.Join(cb.workspace, filename)
        if data, err := os.ReadFile(filePath); err == nil {
            // 读取并添加到系统提示
        }
    }
}
```

##### 4. 系统提示构建流程

```
BuildSystemPrompt()
    │
    ├─ getIdentity()        # 硬编码的核心身份
    ├─ LoadBootstrapFiles() # AGENT.md, SOUL.md, USER.md, IDENTITY.md
    ├─ Skills Summary       # skills/*/SKILL.md
    └─ Memory Context      # memory/MEMORY.md

最终系统提示 = 身份 + 行为准则 + 用户信息 + 技能 + 记忆
```

##### 5. 示例内容

**AGENT.md**:
```markdown
## Guidelines
- Always explain what you're doing before taking actions
- Ask for clarification when request is ambiguous
- Use tools to help accomplish tasks
```

**IDENTITY.md**:
```markdown
## Name
PicoClaw 🦞

## Capabilities
- Web search and content fetching
- File system operations
- Multi-channel messaging
```

**SOUL.md**:
```markdown
## Personality
- Helpful and friendly
- Concise and to the point
- Honest and transparent
```

**USER.md**:
```markdown
## Preferences
- Communication style: casual
- Timezone: Asia/Shanghai
- Language: Chinese
```

这些文件是 PicoClaw 实现**上下文感知**和**个性化服务**的核心机制，通过将配置内嵌到系统提示中，让 AI 能够了解自己是谁、用户是谁、如何表现。

---

## 六、Agent 与 Subagent 支持情况

### Q: 当前有多少个 agent？现在支持调用使用 subagent 吗？

#### A: PicoClaw 支持多 Agent 和 Subagent

##### 1. 当前 Agent 数量

**默认情况：1 个 Agent**

```go
// pkg/agent/registry.go:29-37
agentConfigs := cfg.Agents.List
if len(agentConfigs) == 0 {
    // 没有配置 agents.list 时，创建默认的 main agent
    implicitAgent := &config.AgentConfig{
        ID:      "main",
        Default: true,
    }
    instance := NewAgentInstance(implicitAgent, &cfg.Agents.Defaults, cfg, provider)
    registry.agents["main"] = instance
}
```

**配置多个 Agent**：
```json
{
  "agents": {
    "list": [
      {
        "id": "main",
        "name": "Main Agent",
        "default": true
      },
      {
        "id": "coder",
        "name": "Coding Agent"
      }
    ]
  }
}
```

##### 2. Subagent 支持情况

**支持两种 Subagent：**

| 工具 | 类型 | 状态 | 说明 |
|------|------|------|------|
| `spawn` | 异步 | ✅ 已注册 | 后台执行，任务完成后通知用户 |
| `subagent` | 同步 | ⚠️ 代码存在但未注册 | 等待任务完成后返回结果 |

**spawn 工具使用**：
```go
// pkg/agent/loop.go:143-151
subagentManager := tools.NewSubagentManager(provider, agent.Model, agent.Workspace, msgBus)
spawnTool := tools.NewSpawnTool(subagentManager)
agent.Tools.Register(spawnTool)
```

**执行流程**：
```
主 Agent 调用 spawn tool
    ↓
创建 SubagentManager.Spawn()
    ↓
后台执行: go sm.runTask()
    ↓
调用 RunToolLoop() 执行 LLM + 工具
    ↓
任务完成后通过消息总线通知主 Agent
    ↓
AgentLoop.processSystemMessage() 发送结果给用户
```

##### 3. 子代理权限控制

```json
{
  "agents": {
    "list": [
      {
        "id": "main",
        "subagents": {
          "allow_agents": ["coder", "researcher"],
          "model": {
            "primary": "gpt-4"
          }
        }
      }
    ]
  }
}
```

---

## 七、心跳任务维护机制

### Q: 心跳任务中的任务列表是怎么维护的？

#### A: 通过 HEARTBEAT.md 文件维护

##### 1. 任务列表文件位置

```
~/.picoclaw/workspace/
├── HEARTBEAT.md    ← 心跳任务列表（首次运行时自动创建）
├── AGENT.md
└── ...
```

##### 2. 文件结构

```markdown
# Heartbeat Check List

This file contains tasks for the heartbeat service to check periodically.

## Instructions

- Execute ALL tasks listed below. Do NOT skip any task.
- For simple tasks (e.g., report current time), respond directly.
- For complex tasks that may take time, use the spawn tool to create a subagent.
- The spawn tool is async - subagent results will be sent to the user automatically.
- After spawning a subagent, CONTINUE to process remaining tasks.
- Only respond with HEARTBEAT_OK when ALL tasks are done AND nothing needs attention.

---

Add your heartbeat tasks below this line:

## 每日任务

- 检查服务器磁盘使用情况
- 查看是否有安全更新
```

##### 3. 执行流程

```
定时器触发（默认 30 分钟）
    │
    ▼
1. buildPrompt() 读取 HEARTBEAT.md
    │
    ▼
2. 构建提示消息（包含当前时间）
    │
    ▼
3. 获取最后活跃渠道（from state）
    │
    ▼
4. 调用 AgentLoop.ProcessHeartbeat()
    │
    ▼
5. 处理结果：
   - Async: 后台任务，不打扰用户
   - Silent: 仅记录日志
   - 正常: 发送消息给用户
```

##### 4. 核心代码

```go
// pkg/heartbeat/service.go:218-248
func (hs *HeartbeatService) buildPrompt() string {
    heartbeatPath := filepath.Join(hs.workspace, "HEARTBEAT.md")
    
    data, err := os.ReadFile(heartbeatPath)
    if err != nil {
        if os.IsNotExist(err) {
            hs.createDefaultHeartbeatTemplate()  // 自动创建模板
            return ""
        }
        return ""
    }
    
    now := time.Now().Format("2006-01-02 15:04:05")
    return fmt.Sprintf(`# Heartbeat Check

Current time: %s

You are a proactive AI assistant. This is a scheduled heartbeat check.
Review the following tasks and execute any necessary actions using available skills.
If there is nothing that requires attention, respond ONLY with: HEARTBEAT_OK

%s
`, now, content)
}
```

##### 5. 配置选项

```json
{
  "heartbeat": {
    "enabled": true,
    "interval": 30  // 分钟，最小 5 分钟
  }
}
```

##### 6. 执行特点

| 特点 | 说明 |
|------|------|
| **无会话历史** | 每次心跳是独立的新会话，不累积上下文 |
| **最小间隔** | 5 分钟（防止过于频繁） |
| **静默响应** | 无需操作时返回 `HEARTBEAT_OK` 不打扰用户 |
| **异步任务** | 复杂任务使用 spawn 创建子代理后台执行 |
| **最后渠道** | 响应发送到用户最后活跃的渠道 |

---

## 八、飞书/Lark 通道支持

### Q: 当前支持 Lark 这个 channel 吗？Lark 的 channel 和 Feishu 是不是一样的？

#### A: 支持飞书通道，Lark 和 Feishu 是同一个产品

##### 1. 飞书/Lark 支持情况

**✅ 支持飞书通道（配置名：`feishu`）**

```go
// pkg/config/config.go:220-227
type FeishuConfig struct {
    Enabled           bool
    AppID             string
    AppSecret         string
    EncryptKey        string
    VerificationToken string
    AllowFrom         []string
}
```

**配置示例**：
```json
{
  "channels": {
    "feishu": {
      "enabled": true,
      "app_id": "cli_xxxx",
      "app_secret": "xxxx",
      "verification_token": "xxxx",
      "encrypt_key": "",
      "allow_from": []
    }
  }
}
```

##### 2. Lark 和 Feishu 的关系

**是的，Lark 就是飞书**

- **飞书**：中国大陆版
- **Lark**：国际版/企业版
- 使用相同的 API 和 SDK

```go
// pkg/channels/feishu_64.go
lark "github.com/larksuite/oapi-sdk-go/v3"
```

##### 3. 系统支持情况

| 架构 | 支持情况 |
|------|---------|
| amd64 (x86_64) | ✅ 支持 |
| arm64 | ✅ 支持 |
| riscv64 | ✅ 支持 |
| 32位 (armv7l, 386) | ❌ 不支持 |

---

## 九、程序员使用场景

### Q: 我是个程序员，能够利用这个工具干啥事情？

#### A: PicoClaw 可作为程序员的编程助手、远程终端和自动化工具

##### 1. 核心使用场景

| 场景 | 说明 |
|------|------|
| **编程助手** | 代码生成、审查、调试、解释 |
| **远程终端** | 通过 Telegram/飞书执行服务器命令 |
| **定时任务** | 定时执行脚本、发送报告 |
| **API 网关** | 接收 webhook 并处理 |
| **自动化运维** | 监控、部署、日志查询 |

##### 2. 实际使用示例

**编程辅助**：
```
用户: 帮我写一个 Go 的 HTTP 服务器
AI: [生成代码]

用户: 加上日志中间件
AI: [添加 zerolog 中间件]

用户: 帮我写个单元测试
AI: [生成测试代码]
```

**服务器管理**：
```
用户: 服务器状态怎么样？
AI: 检查 CPU、内存、磁盘...

用户: 重启 docker 服务
AI: [执行 systemctl restart docker]
```

**自动化报告**（HEARTBEAT.md 配置）：
```markdown
## 每日任务
- 获取 GitHub issues 统计
- 汇总 CI/CD 构建状态
- 生成日报发送到群聊
```

##### 3. 开发自定义技能

程序员可以开发自定义技能（Skills）来扩展功能：

```yaml
# skills/mytool/SKILL.md
---
name: mytool
description: 自定义工具
---

# MyTool

## 功能

使用 curl 调用内部 API：
curl -s http://internal-api/health
```

**技能目录结构**：
```
skills/mytool/
├── SKILL.md        # 技能定义
└── references/    # 参考文档
```

##### 4. 安全特性（程序员友好）

| 特性 | 说明 |
|------|------|
| **沙盒限制** | 文件/命令操作限制在工作区，防止误操作 |
| **危险命令拦截** | 自动阻止 rm -rf、格式化等危险操作 |
| **白名单机制** | 可配置允许的用户ID |

##### 5. 对比其他工具

| 场景 | PicoClaw | ChatGPT | 其他 Bot |
|------|---------|---------|---------|
| 本地文件操作 | ✅ 沙盒安全 | ❌ | ⚠️ |
| 执行命令 | ✅ | ❌ | ⚠️ |
| 定时任务 | ✅ 心跳 | ❌ | ⚠️ |
| 多渠道接入 | ✅ 10+ | ❌ | ⚠️ |
| 树莓派运行 | ✅ <10MB | ❌ | ❌ |
| 开源可控 | ✅ | ❌ | ⚠️ |

##### 6. 总结

作为程序员，PicoClaw 可以成为你的：
- 🤖 **编程助手** - 代码生成、审查、调试
- 🖥️ **远程终端** - 通过消息渠道管理服务器
- ⏰ **定时任务调度器** - 自动化运维
- 🔌 **API 网关** - 连接各种服务
- 📱 **统一入口** - 一个入口管理多个服务
