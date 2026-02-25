# PicoClaw 系统架构文档

## 概述

PicoClaw 是一个采用 Go 语言编写的超轻量级 AI 个人助手，采用模块化架构设计。整体系统围绕消息总线（Message Bus）构建，通过事件驱动的方式协调各个组件之间的通信。

---

## 整体架构

PicoClaw 采用分层架构设计，从上到下分为：

1. **CLI 层**：命令行接口
2. **Gateway 层**：消息网关和通道管理
3. **Agent 层**：AI 代理核心逻辑
4. **Provider 层**：LLM 提供商适配
5. **工具层**：各种工具能力的实现
6. **基础设施层**：配置、状态、日志等

---

## 核心组件

### 1. CLI 层（cmd/picoclaw）

CLI 层是用户与 PicoClaw 交互的入口点，负责解析命令行参数并调用相应的功能模块。

#### 主要命令

| 命令 | 文件 | 功能 |
|-----|------|------|
| `onboard` | cmd_onboard.go | 初始化配置和工作区 |
| `agent` | cmd_agent.go | 与 AI 代理交互 |
| `gateway` | cmd_gateway.go | 启动消息网关服务 |
| `status` | cmd_status.go | 显示系统状态 |
| `auth` | cmd_auth.go | 管理认证凭据 |
| `cron` | cmd_cron.go | 管理定时任务 |
| `migrate` | cmd_migrate.go | 从 OpenClaw 迁移 |
| `skills` | cmd_skills.go | 管理技能插件 |

#### 核心文件

```go
// cmd/picoclaw/main.go
func main() {
    switch command {
    case "onboard": onboard()
    case "agent":   agentCmd()
    case "gateway": gatewayCmd()
    // ...
    }
}
```

---

### 2. 消息总线（pkg/bus）

消息总线是 PicoClaw 的核心通信枢纽，采用发布-订阅模式实现组件间的松耦合通信。

#### 消息类型

```go
// pkg/bus/types.go
type InboundMessage struct {
    Channel    string            // 消息来源渠道
    SenderID   string            // 发送者 ID
    ChatID     string            // 会话 ID
    Content    string            // 消息内容
    SessionKey string            // 会话键
    Metadata   map[string]string // 附加元数据
}

type OutboundMessage struct {
    Channel string // 目标渠道
    ChatID  string // 目标会话
    Content string // 消息内容
}
```

#### 核心功能

- **消息消费**：从通道接收入站消息
- **消息发布**：向通道发布出站消息
- **上下文支持**：支持 context.Context 用于取消和超时控制

---

### 3. 通道管理（pkg/channels）

通道管理器负责管理与各种即时通讯平台的集成，将外部消息转换为内部消息格式。

#### 支持的通道

| 通道 | 文件 | 协议 |
|-----|------|------|
| Telegram | telegram.go | MTProto |
| Discord | discord.go | Discord API |
| QQ | qq.go | OneBot 协议 |
| DingTalk | dingtalk.go |钉钉 API |
| LINE | line.go | LINE Messaging API |
| WeCom | wecom.go, wecom_app.go | 企业微信 API |
| Slack | slack.go | Slack API |
| 飞书 | feishu_32.go, feishu_64.go | 飞书 API |
| WhatsApp | whatsapp.go | WhatsApp API |
| MaixCAM | maixcam.go | 自定义协议 |

#### 通道基类

```go
// pkg/channels/base.go
type Channel interface {
    Name() string                    // 通道名称
    Start() error                    // 启动通道
    Stop() error                     // 停止通道
    SendMessage(chatID, content string) error  // 发送消息
    ParseInboundMessage(payload []byte) (*bus.InboundMessage, error)  // 解析入站消息
}
```

#### 通道管理器

```go
// pkg/channels/manager.go
type Manager struct {
    channels map[string]Channel  // 通道映射
    config   *config.Config      // 配置引用
}

func (m *Manager) Register(channel Channel)  // 注册通道
func (m *Manager) GetChannel(name string) (Channel, bool)  // 获取通道
func (m *Manager) GetEnabledChannels() []string  // 获取已启用通道列表
```

---

### 4. Agent 核心（pkg/agent）

Agent 是 PicoClaw 的核心 AI 处理单元，负责处理用户消息、调用 LLM、执行工具并返回响应。

#### 核心结构

```go
// pkg/agent/loop.go
type AgentLoop struct {
    bus            *bus.MessageBus       // 消息总线
    cfg            *config.Config         // 配置
    registry       *AgentRegistry        // 代理注册表
    state          *state.Manager        // 状态管理
    running        atomic.Bool           // 运行状态
    summarizing    sync.Map              // 摘要处理中映射
    fallback       *providers.FallbackChain  // 提供商回退链
    channelManager *channels.Manager     // 通道管理器
}
```

#### 代理实例

```go
// pkg/agent/instance.go
type AgentInstance struct {
    ID             string                    // 代理 ID
    Name           string                    // 代理名称
    Model          string                    // 模型名称
    Provider       providers.LLMProvider     // LLM 提供商
    Tools          *tools.Registry           // 工具注册表
    Sessions       *session.Manager          // 会话管理
    ContextBuilder *context.ContextBuilder  // 上下文构建器
    MaxIterations  int                       // 最大迭代次数
    MaxTokens      int                       // 最大 token 数
    Temperature    float64                   // 温度参数
}
```

#### 消息处理流程

1. **接收消息**：从消息总线获取入站消息
2. **路由决策**：根据消息来源确定使用哪个代理
3. **历史加载**：加载会话历史和摘要
4. **构建上下文**：构建 LLM 消息上下文
5. **迭代执行**：循环调用 LLM 并执行工具
6. **响应返回**：返回最终响应并保存到会话

---

### 5. Provider 层（pkg/providers）

Provider 层负责与各种 LLM 服务商进行通信，支持多种协议和认证方式。

#### 支持的协议

| 协议 | 提供商 |
|------|--------|
| **OpenAI 兼容** | OpenAI、Anthropic、智谱 AI、DeepSeek、Groq、Moonshot、Qwen、Ollama、OpenRouter、VLLM、Cerebras、火山引擎 |
| **Anthropic 原生** | Claude（官方 API） |
| **自定义** | Antigravity、Google Cloud OAuth |
| **gRPC** | GitHub Copilot |

#### 核心接口

```go
// pkg/providers/factory_provider.go
type LLMProvider interface {
    Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]any) (*LLMResponse, error)
}
```

#### 提供商工厂

```go
// pkg/providers/factory_provider.go
type ProviderFactory struct {
    providers map[string]LLMProvider  // 提供商映射
}

func (f *ProviderFactory) CreateProvider(config *ModelConfig) (LLMProvider, error)
func (f *ProviderFactory) Register(name string, provider LLMProvider)
```

#### 主流提供商实现

| 提供商 | 文件 | 特点 |
|--------|------|------|
| Anthropic | anthropic/provider.go | Claude 原生 API 支持 |
| Claude CLI | claude_cli_provider.go | 本地 Claude CLI 集成 |
| OpenAI | openai_provider.go | GPT 模型支持 |
| Zhipu | zhipu_provider.go | 智谱 AI GLM 模型 |
| Groq | groq_provider.go | 快速推理 + Whisper 转录 |

#### 回退机制

```go
// pkg/providers/fallback.go
type FallbackChain struct {
    cooldown *CooldownTracker  // 冷却追踪
}

func (f *FallbackChain) Execute(ctx context.Context, candidates []string, callFn func(ctx context.Context, provider, model string) (*LLMResponse, error)) (*FallbackResult, error)
```

---

### 6. 工具系统（pkg/tools）

工具系统为 AI 代理提供执行各种操作的能力，包括文件操作、网络请求、硬件控制等。

#### 工具注册表

```go
// pkg/tools/registry.go
type Registry struct {
    tools map[string]Tool  // 工具映射
    mu    sync.RWMutex     // 读写锁
}

func (r *Registry) Register(tool Tool)      // 注册工具
func (r *Registry) Get(name string) (Tool, bool)  // 获取工具
func (r *Registry) List() []string           // 列出所有工具
func (r *Registry) ToProviderDefs() []providers.ToolDefinition  // 转换为 Provider 格式
```

#### 工具接口

```go
// pkg/tools/tool.go
type Tool interface {
    Name() string                      // 工具名称
    Description() string               // 工具描述
    Parameters() map[string]Param      // 参数定义
    Execute(ctx context.Context, args map[string]any, channel, chatID string) *ToolResult  // 执行
}
```

#### 工具分类

| 类别 | 工具 | 说明 |
|------|------|------|
| **网络** | web_search, web_fetch | 搜索和获取网页内容 |
| **文件** | read_file, write_file, list_dir, edit_file | 文件操作 |
| **执行** | exec | 命令执行 |
| **消息** | message | 发送消息 |
| **子代理** | spawn, subagent | 创建子代理 |
| **硬件** | i2c, spi | 硬件通信（Linux） |
| **技能** | find_skills, install_skill | 技能管理 |
| **任务** | cron | 定时任务 |

#### 工具执行上下文

```go
// pkg/tools/context.go
type ExecuteContext struct {
    Context   context.Context  // Go 上下文
    Channel   string           // 当前渠道
    ChatID    string           // 当前会话 ID
    Workspace string           // 工作区路径
}
```

---

### 7. 技能系统（pkg/skills）

技能系统允许用户扩展 PicoClaw 的能力，通过安装额外的技能插件来实现新的功能。

#### 技能组件

| 组件 | 文件 | 功能 |
|------|------|------|
| 技能加载器 | loader.go | 加载和解析技能 |
| 技能注册表 | registry.go | 管理已安装技能 |
| 技能安装器 | installer.go | 从远程安装技能 |
| 技能搜索 | search_cache.go | 技能搜索缓存 |
| ClawHub 注册表 | clawhub_registry.go | ClawHub 技能市场 |

#### 技能结构

```
skill-name/
├── SKILL.md           # 技能定义
├── references/        # 参考文档
└── ...                # 其他文件
```

---

### 8. 会话管理（pkg/session）

会话管理系统负责维护与用户的对话历史，支持多会话并行处理。

#### 会话管理器

```go
// pkg/session/manager.go
type Manager struct {
    sessions    map[string]*Session  // 会话映射
    workspace   string                // 工作区路径
    mu          sync.RWMutex         // 读写锁
}

type Session struct {
    Key     string              // 会话键
    History []providers.Message // 消息历史
    Summary string              // 历史摘要
}
```

---

### 9. 状态管理（pkg/state）

状态管理系统负责持久化存储代理的状态信息，如最后活跃的渠道等。

#### 状态管理器

```go
// pkg/state/state.go
type Manager struct {
    workspace string  // 工作区路径
}

func (m *Manager) SetLastChannel(channel string) error
func (m *Manager) GetLastChannel() (string, error)
func (m *Manager) SetLastChatID(chatID string) error
func (m *Manager) GetLastChatID() (string, error)
```

---

### 10. 路由系统（pkg/routing）

路由系统负责根据消息来源确定使用哪个代理和处理会话。

#### 路由决策

```go
// pkg/routing/route.go
type RouteInput struct {
    Channel   string          // 消息渠道
    AccountID string          // 账户 ID
    Peer      *RoutePeer      // 对等方信息
    GuildID   string          // Discord 服务器 ID
    TeamID    string          // 团队 ID
}

type Route struct {
    AgentID   string  // 目标代理 ID
    SessionKey string // 会话键
    MatchedBy string  // 匹配方式
}
```

---

### 11. 配置管理（pkg/config）

配置管理系统负责加载和管理所有配置项。

#### 配置结构

```go
// pkg/config/config.go
type Config struct {
    Agents      AgentsConfig      `json:"agents"`       // 代理配置
    ModelList   []ModelConfig     `json:"model_list"`   // 模型列表
    Providers   ProvidersConfig   `json:"providers"`   // 提供商配置（已废弃）
    Channels    ChannelsConfig    `json:"channels"`     // 通道配置
    Tools       ToolsConfig        `json:"tools"`        // 工具配置
    Heartbeat   HeartbeatConfig    `json:"heartbeat"`   // 心跳配置
}
```

---

### 12. 定时任务（pkg/cron）

定时任务系统负责执行和管理定时任务。

#### Cron 服务

```go
// pkg/cron/service.go
type Service struct {
    workspace string  // 工作区路径
    msgBus    *bus.MessageBus  // 消息总线
}
```

---

### 13. 心跳服务（pkg/heartbeat）

心跳服务负责周期性执行预设任务。

#### 心跳服务

```go
// pkg/heartbeat/service.go
type Service struct {
    cfg      *config.Config  // 配置
    agentLoop *agent.AgentLoop  // Agent 循环
    msgBus   *bus.MessageBus    // 消息总线
}
```

---

### 14. 健康检查（pkg/health）

健康检查服务提供系统状态监控接口。

#### 健康检查服务器

```go
// pkg/health/server.go
type Server struct {
    addr string  // 监听地址
}
```

---

### 15. 设备服务（pkg/devices）

设备服务负责管理外部硬件设备的连接和通信。

#### 设备源

| 设备 | 文件 | 说明 |
|------|------|------|
| USB | usb_linux.go | USB 设备连接 |

---

### 16. 语音处理（pkg/voice）

语音处理模块负责处理语音消息。

#### 语音转录

```go
// pkg/voice/transcriber.go
type Transcriber struct {
    provider providers.LLMProvider  // LLM 提供商
}
```

---

## 数据流

### 1. 消息处理流程

```
用户发送消息
    ↓
Channel 接收消息
    ↓
ParseInboundMessage() 转换为 InboundMessage
    ↓
MessageBus.PublishInbound() 发布到入站队列
    ↓
AgentLoop.Run() 消费消息
    ↓
processMessage() 处理消息
    ↓
路由决策 → 确定 Agent 和 Session
    ↓
runAgentLoop() 执行代理循环
    ↓
构建消息上下文 → 调用 LLM
    ↓
执行 Tool Calls（如有）
    ↓
返回响应 → 保存到会话
    ↓
MessageBus.PublishOutbound() 发布出站消息
    ↓
Channel.SendMessage() 发送响应给用户
```

### 2. LLM 调用流程

```
构建系统提示
    ↓
加载会话历史
    ↓
构建消息列表
    ↓
调用 LLM.Chat()
    ↓
检查 Tool Calls
    ↓
无 Tool Calls → 返回内容
    ↓
有 Tool Calls → 执行工具
    ↓
添加工具结果到消息
    ↓
继续迭代（最多 MaxIterations 次）
```

### 3. 心跳任务流程

```
Heartbeat 定时触发
    ↓
读取 HEARTBEAT.md 文件
    ↓
解析任务列表
    ↓
执行快速任务（直接响应）
    ↓
执行长时间任务（使用 spawn 创建子代理）
    ↓
子代理完成任务
    ↓
发送通知给用户
```

---

## 依赖关系

### 主要依赖

| 包 | 用途 |
|---|------|
| github.com/anthropic/anthropic-sdk-go | Anthropic Claude SDK |
| github.com/google/generative-ai-go | Google Gemini SDK |
| github.com/sashabaranov/go-openai | OpenAI 兼容 SDK |
| github.com/tmc/langchaingo | LangChain Go 实现 |
| gorm.io/gorm | 数据库 ORM |

---

## 安全性设计

### 1. 工作区隔离

- 所有文件操作限制在工作区内
- 命令执行路径必须位于工作区
- 子代理继承相同的安全限制

### 2. 危险命令防护

- 拦截批量删除命令
- 阻止磁盘格式化操作
- 禁止系统关机命令

### 3. API Key 安全

- 支持从环境变量读取
- 支持配置文件加密
- 支持外部密钥管理服务

---

## 扩展性设计

### 1. 通道扩展

新增通道只需实现 `Channel` 接口：

```go
type Channel interface {
    Name() string
    Start() error
    Stop() error
    SendMessage(chatID, content string) error
    ParseInboundMessage(payload []byte) (*bus.InboundMessage, error)
}
```

### 2. 提供商扩展

新增 LLM 提供商只需实现 `LLMProvider` 接口：

```go
type LLMProvider interface {
    Chat(ctx context.Context, messages []Message, tools []ToolDefinition, model string, options map[string]any) (*LLMResponse, error)
}
```

### 3. 工具扩展

新增工具只需实现 `Tool` 接口：

```go
type Tool interface {
    Name() string
    Description() string
    Parameters() map[string]Param
    Execute(ctx context.Context, args map[string]any, channel, chatID string) *ToolResult
}
```

---

## 部署架构

### 1. 本地部署

```
┌─────────────────┐
│   CLI / Agent   │
│   (命令行模式)   │
└─────────────────┘
```

### 2. 网关部署

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Channel Layer  │────▶│  Message Bus    │────▶│   Agent Loop    │
│  (多通道接入)    │     │  (消息队列)      │     │  (AI 处理核心)   │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                                         │
                                                         ▼
                                                ┌─────────────────┐
                                                │  LLM Providers  │
                                                │ (多模型支持)     │
                                                └─────────────────┘
```

### 3. Docker 部署

```
┌─────────────────────────────────────┐
│         Docker Compose              │
│  ┌─────────────────────────────┐   │
│  │   PicoClaw Gateway          │   │
│  │   (多通道 + Agent)          │   │
│  └─────────────────────────────┘   │
└─────────────────────────────────────┘
```

---

## 性能优化

### 1. 内存优化

- 按需加载会话历史
- 及时释放已结束会话
- 工具结果缓存

### 2. 上下文压缩

- 超过阈值自动摘要
- 紧急压缩处理上下文溢出
- 多部分摘要处理长对话

### 3. 并发优化

- 异步工具执行
- 并发会话处理
- 工具执行并发控制
