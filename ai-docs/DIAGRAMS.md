# PicoClaw 架构图表文档

本文档包含 PicoClaw 项目的各种架构图表，使用 Mermaid 语法绘制。

---

## 1. 系统总体架构图

### 1.1 整体架构（C4 风格）

```mermaid
graph TB
    subgraph 用户层
        U1[Telegram 用户]
        U2[Discord 用户]
        U3[QQ 用户]
        U4[钉钉用户]
        U5[LINE 用户]
        U6[WeCom 用户]
        U7[CLI 用户]
    end

    subgraph 网关层
        GW[Gateway]
        CM[Channel Manager]
    end

    subgraph 核心层
        MB[Message Bus]
        AL[Agent Loop]
        AR[Agent Registry]
    end

    subgraph 工具层
        TR[Tools Registry]
        T1[Web Search]
        T2[File Ops]
        T3[Exec]
        T4[Message]
        T5[Spawn]
        T6[Hardware]
    end

    subgraph 提供商层
        PF[Provider Factory]
        P1[OpenAI]
        P2[Anthropic]
        P3[智谱 AI]
        P4[DeepSeek]
        P5[Groq]
        P6[其他...]
    end

    U1 --> GW
    U2 --> GW
    U3 --> GW
    U4 --> GW
    U5 --> GW
    U6 --> GW
    U7 --> AL

    GW --> CM
    CM --> MB
    MB --> AL

    AL --> AR
    AL --> TR

    TR --> T1
    TR --> T2
    TR --> T3
    TR --> T4
    TR --> T5
    TR --> T6

    AR --> PF
    PF --> P1
    PF --> P2
    PF --> P3
    PF --> P4
    PF --> P5
    PF --> P6
```

---

## 2. 消息处理流程图

### 2.1 消息处理序列图

```mermaid
sequenceDiagram
    participant User as 用户
    participant Channel as Channel
    participant Bus as Message Bus
    participant Agent as Agent Loop
    participant Tools as Tools Registry
    participant Provider as LLM Provider

    User->>Channel: 发送消息
    Channel->>Channel: ParseInboundMessage
    Channel->>Bus: PublishInbound(InboundMessage)
    
    Bus->>Agent: ConsumeInbound
    Agent->>Agent: 路由决策
    
    rect rgb(240, 248, 255)
        note right of Agent: 迭代循环开始
        loop 最多 MaxIterations 次
            Agent->>Provider: Chat(messages, tools)
            Provider-->>Agent: LLM Response
            
            alt 有 Tool Calls
                Agent->>Tools: Execute(tool_call)
                Tools-->>Agent: Tool Result
                Agent->>Agent: 添加工具结果到消息
            else 无 Tool Calls
                Agent->>Agent: 准备返回响应
            end
        end
    end
    
    Agent->>Bus: PublishOutbound(响应)
    Bus->>Channel: 出站消息
    Channel->>User: 发送响应
```

### 2.2 工具执行流程图

```mermaid
flowchart TD
    A[LLM 返回 Tool Calls] --> B{是否有 Tool Calls}
    
    B -->|是| C[遍历 Tool Calls]
    B -->|否| D[返回内容]
    
    C --> E[获取工具]
    E --> F{工具是否存在}
    
    F -->|是| G[执行工具]
    F -->|否| H[返回错误]
    
    G --> I{执行结果}
    
    I -->|成功| J[构建 Tool Result 消息]
    I -->|失败| K[构建错误消息]
    
    J --> L[添加到消息列表]
    K --> L
    
    L --> M[继续下一轮迭代]
    M --> A
    
    D --> N[保存到会话]
    N --> O[返回最终响应]
    
    H --> O
```

---

## 3. 组件关系图

### 3.1 Agent 组件关系

```mermaid
classDiagram
    class AgentLoop {
        -bus: MessageBus
        -cfg: Config
        -registry: AgentRegistry
        -state: state.Manager
        -fallback: FallbackChain
        +Run(ctx) error
        +Stop()
        +ProcessDirect() string
        +ProcessHeartbeat() string
    }
    
    class AgentRegistry {
        -agents: map[string]*AgentInstance
        +Register(agent *AgentInstance)
        +GetAgent(id string) *AgentInstance
        +ResolveRoute() Route
    }
    
    class AgentInstance {
        -id: string
        -model: string
        -provider: LLMProvider
        -tools: tools.Registry
        -sessions: session.Manager
        -contextBuilder: context.ContextBuilder
    }
    
    class SessionManager {
        -sessions: map[string]*Session
        +AddMessage()
        +GetHistory() []Message
        +Save()
    }
    
    class ToolsRegistry {
        -tools: map[string]Tool
        +Register(tool Tool)
        +Execute() ToolResult
        +ToProviderDefs() []ToolDefinition
    }
    
    AgentLoop --> AgentRegistry
    AgentLoop --> SessionManager
    AgentRegistry --> AgentInstance
    AgentInstance --> ToolsRegistry
    AgentInstance --> SessionManager
```

### 3.2 Provider 架构

```mermaid
classDiagram
    class LLMProvider {
        <<interface>>
        +Chat() *LLMResponse
    }
    
    class ProviderFactory {
        -providers: map[string]LLMProvider
        +CreateProvider() LLMProvider
        +Register()
    }
    
    class FallbackChain {
        -cooldown: CooldownTracker
        +Execute() *FallbackResult
    }
    
    class AnthropicProvider {
        +Chat() *LLMResponse
    }
    
    class OpenAIProvider {
        +Chat() *LLMResponse
    }
    
    class ZhipuProvider {
        +Chat() *LLMResponse
    }
    
    class ClaudeCLIProvider {
        +Chat() *LLMResponse
    }
    
    LLMProvider <|.. AnthropicProvider
    LLMProvider <|.. OpenAIProvider
    LLMProvider <|.. ZhipuProvider
    LLMProvider <|.. ClaudeCLIProvider
    
    ProviderFactory --> LLMProvider
    ProviderFactory --> FallbackChain
```

---

## 4. 通道集成架构

### 4.1 通道管理器架构

```mermaid
graph TB
    subgraph Channels
        TG[Telegram]
        DC[Discord]
        QQ[QQ]
        DT[DingTalk]
        LN[LINE]
        WC[WeCom]
        SL[Slack]
        FS[飞书]
        WA[WhatsApp]
    end
    
    subgraph Manager
        CM[Channel Manager]
    end
    
    subgraph Bus
        MB[Message Bus]
    end
    
    TG --> CM
    DC --> CM
    QQ --> CM
    DT --> CM
    LN --> CM
    WC --> CM
    SL --> CM
    FS --> CM
    WA --> CM
    
    CM --> MB
    
    MB --> TG
    MB --> DC
    MB --> QQ
    MB --> DT
    MB --> LN
    MB --> WC
    MB --> SL
    MB --> FS
    MB --> WA
```

### 4.2 通道消息流

```mermaid
flowchart LR
    subgraph Inbound
        External[外部平台] -->|HTTP/WebSocket| Channel[Channel]
        Channel -->|ParseInbound| InboundMsg[InboundMessage]
        InboundMsg -->|Publish| Bus
    end
    
    subgraph Outbound
        Bus -->|Consume| OutboundMsg[OutboundMessage]
        OutboundMsg -->|SendMessage| Channel
        Channel -->|HTTP/WebSocket| External
    end
```

---

## 5. 会话管理流程

### 5.1 会话状态图

```mermaid
stateDiagram-v2
    [*] --> NewSession: 用户新会话
    NewSession --> Active: 首条消息
    Active --> Active: 继续对话
    Active --> Summarizing: 超过阈值
    Summarizing --> Active: 摘要完成
    Active --> Compressing: 上下文溢出
    Compressing --> Active: 压缩完成
    Active --> Expired: 超时无活动
    Expired --> [*]
```

### 5.2 会话历史管理

```mermaid
flowchart TD
    A[用户消息] --> B[添加到历史]
    B --> C{历史长度 > 阈值?}
    
    C -->|是| D[触发摘要]
    D --> E{历史 token > 75% 窗口?}
    
    E -->|是| F[强制压缩]
    E -->|否| G[正常摘要]
    
    F --> H[保留后 50% 消息]
    G --> I[保留最后 4 条]
    
    H --> J[摘要前 N-4 条]
    I --> J
    
    J --> K[保存摘要]
    K --> L[截断历史]
    L --> M[保存会话]
    
    C -->|否| M
    M --> N[继续处理]
```

---

## 6. 心跳任务流程

### 6.1 心跳任务序列图

```mermaid
sequenceDiagram
    participant Timer as 定时器
    participant Heartbeat as Heartbeat Service
    participant Agent as Agent Loop
    participant File as HEARTBEAT.md
    participant Spawn as Spawn Tool
    participant User as 用户

    Timer->>Heartbeat: 定时触发（30分钟）
    Heartbeat->>File: 读取任务列表
    
    rect rgb(240, 248, 255)
        note right of Heartbeat: 处理每个任务
        loop 任务列表
            Heartbeat->>Heartbeat: 解析任务
            
            alt 快速任务
                Heartbeat->>Agent: ProcessHeartbeat()
                Agent-->>User: 直接响应
            else 长时间任务
                Heartbeat->>Spawn: spawn 子代理
                Spawn-->>User: 异步通知
            end
        end
    end
```

### 6.2 子代理通信

```mermaid
flowchart LR
    A[Heartbeat] -->|spawn| B[子代理]
    
    B --> C[独立执行任务]
    C --> D{任务完成?}
    
    D -->|是| E[发送系统消息]
    D -->|否| F[重试/失败]
    
    E --> G[Agent Loop]
    G --> H{消息渠道}
    
    H -->|内部渠道| I[仅记录日志]
    H -->|外部渠道| J[通知用户]
    
    F --> K[结束]
    J --> K
    I --> K
```

---

## 7. 技能系统架构

### 7.1 技能加载流程

```mermaid
flowchart TD
    A[开始] --> B[加载工作区技能]
    B --> C[加载全局技能]
    C --> D[加载内置技能]
    
    D --> E{有 SKILL.md?}
    E -->|是| F[解析技能定义]
    E -->|否| G[跳过]
    
    F --> H[注册到工具]
    H --> I[添加到上下文]
    
    I --> J[完成]
    G --> J
```

### 7.2 技能搜索流程

```mermaid
flowchart TD
    A[用户请求技能] --> B{使用缓存?}
    
    B -->|是| C[从缓存获取]
    B -->|否| D[查询 ClawHub]
    
    C --> E[返回结果]
    D --> F{找到结果?}
    
    F -->|是| G[添加到缓存]
    G --> E
    
    F -->|否| H[返回空结果]
    H --> E
    
    E --> I[用户选择安装]
    I --> J[下载技能]
    J --> K[解压到工作区]
    K --> L[重新加载技能]
```

---

## 8. 路由系统

### 8.1 消息路由决策

```mermaid
flowchart TD
    A[收到消息] --> B{是否有 account_id?}
    
    B -->|是| C[按 account_id 路由]
    B -->|否| D{是否有 guild_id?}
    
    C --> E{查找代理?}
    D -->|是| F[按 guild_id 路由]
    D -->|否| G{是否有 team_id?}
    
    E -->|找到| H[使用匹配代理]
    E -->|未找到| I[使用默认代理]
    
    F --> J[按频道+服务器路由]
    G -->|是| K[按 team_id 路由]
    G -->|否| L[按渠道路由]
    
    J --> M[构建 Session Key]
    K --> M
    L --> M
    H --> M
    I --> M
    
    M --> N[返回路由结果]
```

---

## 9. 配置加载流程

### 9.1 启动配置流程

```mermaid
flowchart TD
    A[CLI 启动] --> B{配置文件存在?}
    
    B -->|否| C[提示运行 onboard]
    B -->|是| D[加载 config.json]
    
    D --> E{解析错误?}
    E -->|是| F[报错退出]
    E -->|否| G{有 model_list?}
    
    G -->|是| H[使用 model_list]
    G -->|否| I[使用 legacy providers]
    
    H --> J[创建 Provider Factory]
    I --> K[创建单个 Provider]
    
    J --> L[创建 Agent Registry]
    K --> L
    
    L --> M[加载技能]
    M --> N[注册工具]
    
    N --> O[创建 Agent Loop]
    O --> P[初始化通道]
    P --> Q[启动服务]
```

---

## 10. 错误处理流程

### 10.1 LLM 错误处理

```mermaid
flowchart TD
    A[LLM 调用] --> B{成功?}
    
    B -->|是| C[返回响应]
    B -->|否| D{错误类型}
    
    D --> E[Token/Context 错误]
    D --> F[Rate Limit 错误]
    D --> G[认证错误]
    D --> H[其他错误]
    
    E --> I{重试次数 < 3?}
    I -->|是| J[压缩上下文]
    J --> K[重试调用]
    K --> A
    
    I -->|否| L[返回错误]
    
    F --> M{使用 Fallback?}
    M -->|是| N[切换 Provider]
    N --> A
    
    M -->|否| L
    
    G --> O[提示认证错误]
    O --> L
    
    H --> P[返回错误信息]
    P --> L
```

---

## 11. 部署架构

### 11.1 本地部署

```mermaid
graph LR
    subgraph 本地部署
        CLI[PicoClaw CLI]
        AG[Agent]
        PROV[LLM Providers]
    end
    
    User1[用户] --> CLI
    CLI --> AG
    AG --> PROV
    PROV --> AG
    AG --> User1
```

### 11.2 网关部署

```mermaid
graph TB
    subgraph Docker/Server
        GW[Gateway]
        CH[Channels]
        AL[Agent Loop]
        PROV[Providers]
    end
    
    User1[Telegram] -->|HTTPS| GW
    User2[Discord] -->|HTTPS| GW
    User3[QQ] -->|HTTPS| GW
    
    GW --> CH
    CH --> AL
    AL --> PROV
    PROV --> AL
    AL --> CH
    CH --> GW
```

---

## 12. 安全沙箱

### 12.1 工作区限制流程

```mermaid
flowchart TD
    A[工具请求] --> B{工具类型}
    
    B -->|文件操作| C{restrict_to_workspace?}
    B -->|exec| D{restrict_to_workspace?}
    
    C -->|true| E[检查路径前缀]
    C -->|false| F[允许访问]
    
    D -->|true| G[检查命令路径]
    D -->|false| H[允许执行]
    
    E -->|在区内| I[允许]
    E -->|在区外| J[拒绝]
    
    G -->|安全| I
    G -->|危险| K[检查危险模式]
    
    K -->|匹配| J
    K -->|不匹配| I
    
    F --> I
    H --> I
    
    J --> L[返回错误]
    I --> M[执行工具]
```

---

## 13. 数据存储结构

### 13.1 工作区目录树

```mermaid
graph TD
    root[~/.picoclaw/]
    
    ws[workspace/]
    sessions[sessions/]
    memory[memory/]
    state[state/]
    cron[cron/]
    skills[skills/]
    
    files1[AGENTS.md]
    files2[HEARTBEAT.md]
    files3[IDENTITY.md]
    files4[SOUL.md]
    files5[TOOLS.md]
    files6[USER.md]
    files7[MEMORY.md]
    
    root --> ws
    ws --> sessions
    ws --> memory
    ws --> state
    ws --> cron
    ws --> skills
    ws --> files1
    ws --> files2
    ws --> files3
    ws --> files4
    ws --> files5
    ws --> files6
    
    memory --> files7
```

---

## 14. 版本与构建信息

### 14.1 发布流程

```mermaid
flowchart LR
    A[代码提交] --> B[CI 构建]
    B --> C{测试通过?}
    
    C -->|否| D[修复问题]
    D --> A
    
    C -->|是| E[构建多平台]
    E --> F[创建 Release]
    F --> G[上传 Assets]
    G --> H[用户下载]
```

### 14.2 版本号管理

```mermaid
flowchart TD
    A[v0.1.0] -->|功能增加| B[v0.2.0]
    B -->|重大变更| C[v1.0.0]
    C -->|bug修复| D[v1.0.1]
    D -->|新功能| E[v1.1.0]
    
    style A fill:#e1f5fe
    style B fill:#e1f5fe
    style C fill:#fff3e0
    style D fill:#e8f5e9
    style E fill:#e1f5fe
```
