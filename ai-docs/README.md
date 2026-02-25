# PicoClaw 项目文档

> 超轻量级个人 AI 助手 - 10美元硬件 · 10MB内存 · 1秒启动

---

## 项目简介

PicoClaw 是一个用 Go 语言编写的超轻量级个人 AI 助手，灵感来源于 nanobot 项目。该项目通过 AI 驱动的自举过程从头开始重构，实现了在低价硬件（10美元）上运行且内存占用小于 10MB 的目标。

**核心特点：**

- 🪶 **超轻量级**：<10MB 内存占用，比同类产品小 99%
- 💰 **极低成本**：可在 10 美元的 Linux 开发板上运行
- ⚡️ **极速启动**：1 秒内启动完成
- 🌍 **真正常驻**：单二进制文件，跨 RISC-V、ARM、x86 架构

---

## 文档索引

### 📚 核心文档

| 文档 | 说明 |
|------|------|
| **[FEATURES.md](./FEATURES.md)** | 功能特性详解 - 包含所有功能、工具、通道支持的详细说明 |
| **[ARCHITECTURE.md](./ARCHITECTURE.md)** | 系统架构文档 - 深入解析各组件的设计和交互 |
| **[DIAGRAMS.md](./DIAGRAMS.md)** | 架构图表 - 使用 Mermaid 绘制的各类架构图 |

---

## 快速链接

### 官方资源

- 🌐 官网：[picoclaw.io](https://picoclaw.io)
- 📱 GitHub：[github.com/sipeed/picoclaw](https://github.com/sipeed/picoclaw)
- 💬 Discord：[discord.gg/V4sAZ9XWpN](https://discord.gg/V4sAZ9XWpN)
- 🐦 Twitter：[@SipeedIO](https://x.com/SipeedIO)

### 快速开始

```bash
# 1. 克隆项目
git clone https://github.com/sipeed/picoclaw.git
cd picoclaw

# 2. 安装依赖
make deps

# 3. 初始化
picoclaw onboard

# 4. 配置
# 编辑 ~/.picoclaw/config.json

# 5. 运行
picoclaw agent -m "你好！"
```

---

## 项目结构

```
picoclaw/
├── cmd/picoclaw/          # CLI 入口
│   ├── main.go            # 主程序
│   ├── cmd_agent.go       # Agent 命令
│   ├── cmd_gateway.go     # 网关命令
│   └── ...
│
├── pkg/                   # 核心包
│   ├── agent/            # AI 代理核心
│   │   ├── loop.go       # 代理循环
│   │   ├── instance.go   # 代理实例
│   │   └── registry.go   # 代理注册
│   │
│   ├── providers/         # LLM 提供商
│   │   ├── anthropic/    # Anthropic Claude
│   │   ├── openai/       # OpenAI GPT
│   │   ├── zhipu/        # 智谱 AI
│   │   └── ...
│   │
│   ├── channels/          # 消息通道
│   │   ├── telegram.go   # Telegram
│   │   ├── discord.go    # Discord
│   │   ├── wecom.go      # 企业微信
│   │   └── ...
│   │
│   ├── tools/            # 工具系统
│   │   ├── registry.go   # 工具注册表
│   │   ├── web_search.go # 网页搜索
│   │   ├── file.go       # 文件操作
│   │   └── ...
│   │
│   ├── bus/              # 消息总线
│   ├── config/           # 配置管理
│   ├── session/          # 会话管理
│   ├── skills/           # 技能系统
│   ├── routing/          # 路由系统
│   └── ...
│
├── config/               # 配置文件示例
├── docs/                 # 项目文档
└── workspace/           # 工作区模板
```

---

## 核心概念

### 1. 消息总线（Message Bus）

消息总线是 PicoClaw 的核心通信枢纽，采用发布-订阅模式实现组件间的松耦合通信。

```mermaid
graph LR
    Channel --> Bus
    Bus --> Agent
    Agent --> Bus
    Bus --> Channel
```

### 2. Agent（代理）

Agent 是处理用户消息的核心单元，包含：

- LLM 提供商连接
- 工具注册表
- 会话管理器
- 上下文构建器

### 3. Provider（提供商）

Provider 负责与各种 LLM 服务商通信，支持：

- OpenAI 兼容协议（OpenAI、智谱、DeepSeek、Groq 等）
- Anthropic 原生协议
- 自定义协议

### 4. Channel（通道）

Channel 负责与各种即时通讯平台集成：

- Telegram、Discord、QQ、钉钉、LINE、企业微信
- Slack、飞书、WhatsApp
- CLI（命令行）

### 5. Tools（工具）

Tools 为 AI 代理提供执行各种操作的能力：

- 网络工具（搜索、获取网页）
- 文件操作（读写、编辑）
- 命令执行
- 消息发送
- 子代理创建
- 硬件通信（I2C、SPI）

---

## 功能特性

### 支持的消息通道

| 通道 | 状态 | 配置难度 |
|------|------|---------|
| Telegram | ✅ 支持 | 简单 |
| Discord | ✅ 支持 | 简单 |
| QQ | ✅ 支持 | 简单 |
| DingTalk | ✅ 支持 | 中等 |
| LINE | ✅ 支持 | 中等 |
| WeCom | ✅ 支持 | 中等 |
| Slack | ✅ 支持 | 简单 |
| 飞书 | ✅ 支持 | 中等 |
| WhatsApp | ✅ 支持 | 简单 |

### 支持的 LLM 提供商

| 提供商 | 协议 | 免费额度 |
|--------|------|---------|
| OpenRouter | OpenAI | 200K tokens/月 |
| 智谱 AI | OpenAI | 200K tokens/月 |
| Groq | OpenAI | 有免费层 |
| Cerebras | OpenAI | 有免费层 |
| Ollama | OpenAI | 本地免费 |
| OpenAI | OpenAI | 付费 |
| Anthropic | Anthropic | 付费 |
| DeepSeek | OpenAI | 付费 |

### 工具能力

- 🌐 **网络搜索**：Brave、Tavily、DuckDuckGo、Perplexity
- 📁 **文件操作**：读、写、编辑、列表
- 💻 **命令执行**：安全沙箱
- 🤖 **子代理**：异步任务处理
- ⏰ **定时任务**：Cron 风格
- 🔧 **硬件**：I2C、SPI（Linux）

---

## 配置示例

### 最小配置

```json
{
  "model_list": [
    {
      "model_name": "gpt-4",
      "model": "openai/gpt-4",
      "api_key": "your-api-key"
    }
  ],
  "agents": {
    "defaults": {
      "model": "gpt-4"
    }
  }
}
```

### 多通道配置

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "YOUR_BOT_TOKEN",
      "allow_from": ["YOUR_USER_ID"]
    },
    "discord": {
      "enabled": true,
      "token": "YOUR_DISCORD_TOKEN",
      "allow_from": ["YOUR_USER_ID"]
    }
  }
}
```

---

## 安全特性

### 工作区沙盒

PicoClaw 默认在沙盒环境中运行，限制代理只能访问配置的工作区目录：

- 📁 文件操作限制在工作区内
- 💻 命令执行路径必须在工作区内
- 🔒 所有执行路径共享相同的安全限制

### 危险命令防护

自动阻止以下危险操作：

- 批量删除（`rm -rf`）
- 磁盘格式化
- 系统关机
- Fork 炸弹

---

## 性能对比

| 指标 | OpenClaw | NanoBot | **PicoClaw** |
|------|----------|---------|--------------|
| 语言 | TypeScript | Python | **Go** |
| 内存 | >1GB | >100MB | **<10MB** |
| 启动时间 | >500s | >30s | **<1s** |
| 成本 | $599 | ~$50 | **~$10** |

---

## 贡献指南

欢迎贡献代码！请查看 [CONTRIBUTING.md](../CONTRIBUTING.zh.md) 了解如何参与开发。

### 开发环境

```bash
# 安装 Go 1.21+
go version

# 克隆项目
git clone https://github.com/sipeed/picoclaw.git

# 安装依赖
make deps

# 运行测试
make test

# 构建
make build
```

---

## 路线图

查看完整路线图：[ROADMAP.md](../ROADMAP.md)

---

## 许可证

MIT License - 查看 [LICENSE](../LICENSE) 了解详情。

---

## 致谢

- 灵感来源：[nanobot](https://github.com/HKUDS/nanobot)
- Logo 设计：皮皮虾 🦐
- 贡献者：感谢所有参与项目的开发者！
