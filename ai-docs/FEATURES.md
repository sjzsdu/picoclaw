# PicoClaw 功能特性文档

## 项目概述

PicoClaw 是一个用 Go 语言编写的超轻量级个人 AI 助手，灵感来源于 nanobot 项目。该项目通过 AI 驱动的自举过程从头开始重构，实现了在低价硬件（10美元）上运行且内存占用小于 10MB 的目标。

---

## 核心特性

### 1. 超轻量级架构

- **极小内存占用**：<10MB RAM，比 OpenClaw 减少 99% 内存使用
- **极速启动**：1 秒内在 0.6GHz 单核上启动完成，比 OpenClaw 快 400 倍
- **单二进制文件**：跨 RISC-V、ARM 和 x86 架构的单文件部署

### 2. 多平台运行

- **低成本硬件支持**：可在 10 美元的 Linux 开发板上运行
- **老旧设备复用**：可在老旧 Android 手机上运行（通过 Termux）
- **多样化部署**：
  - LicheeRV-Nano（$9.9）- 最小化家庭助手
  - NanoKVM（$30-50）- 自动化服务器维护
  - MaixCAM（$50）- 智能监控

### 3. AI 驱动的开发

- **自举开发**：95% 核心代码由 AI Agent 生成
- **人工优化**：人类参与环路优化

---

## 消息通道支持

PicoClaw 支持多种即时通讯平台的集成：

### 支持的通道列表

| 通道类型 | 集成难度 | 说明 |
|---------|---------|------|
| **Telegram** | 简单 | 仅需 Bot Token |
| **Discord** | 简单 | Bot Token + Intents |
| **QQ** | 简单 | AppID + AppSecret |
| **DingTalk** | 中等 | 应用凭证 |
| **LINE** | 中等 | 凭证 + Webhook URL |
| **WeCom** | 中等 | 企业微信支持 |
| **WhatsApp** | 支持 | 集成中 |
| **Slack** | 支持 | 集成中 |
| **飞书** | 支持 | 集成中 |
| **MaixCAM** | 硬件 | AI 摄像头集成 |

### Telegram 集成功能

- 语音消息转文字（通过 Groq Whisper）
- 群组 @ 提及响应
- 用户白名单控制
- 命令自动补全

### Discord 集成功能

- 消息内容意图识别
- 服务器成员意图
- 群组 @ 提及模式
- 用户白名单控制

### WeCom 集成功能

- **WeCom Bot**：智能机器人，支持群聊
- **WeCom App**：自建应用，支持主动消息推送

---

## LLM 提供商支持

### 支持的模型供应商

| 提供商 | 协议 | 用途 | API Key 获取 |
|-------|------|------|--------------|
| **OpenAI** | OpenAI | GPT 模型 | platform.openai.com |
| **Anthropic** | Anthropic | Claude 模型 | console.anthropic.com |
| **智谱 AI (GLM)** | OpenAI | 中文大模型 | bigmodel.cn |
| **DeepSeek** | OpenAI | DeepSeek 模型 | platform.deepseek.com |
| **Google Gemini** | OpenAI | Gemini 模型 | aistudio.google.com |
| **Groq** | OpenAI | 快速推理 + 语音转录 | console.groq.com |
| **Moonshot** | OpenAI | Moonshot 模型 | platform.moonshot.cn |
| **通义千问 (Qwen)** | OpenAI | 阿里 Qwen 模型 | dashscope.aliyuncs.com |
| **NVIDIA** | OpenAI | NVIDIA NIM | build.nvidia.com |
| **Ollama** | OpenAI | 本地模型 | 本地部署 |
| **OpenRouter** | OpenAI | 多模型聚合 | openrouter.ai |
| **VLLM** | OpenAI | 本地 vLLM | 本地部署 |
| **Cerebras** | OpenAI | 快速推理 | cerebras.ai |
| **火山引擎** | OpenAI | 字节跳动模型 | console.volcengine.com |
| **Antigravity** | Custom | Google Cloud OAuth | - |
| **GitHub Copilot** | gRPC | 代码助手 | localhost:4321 |

### 模型配置特性

- **模型列表配置**：`model_list` 支持零代码添加新提供商
- **负载均衡**：同一模型配置多个端点，自动轮询
- **模型回退**：配置主备模型提高可用性
- **多代理支持**：不同代理使用不同 LLM 提供商

---

## 工具能力

### 1. 网络工具

| 工具 | 说明 | 配置 |
|-----|------|------|
| **Web Search** | 网页搜索 | Brave、Tavily、DuckDuckGo、Perplexity |
| **Web Fetch** | 网页内容获取 | 支持代理 |

### 2. 文件操作工具

| 工具 | 说明 | 沙盒限制 |
|-----|------|---------|
| **read_file** | 读取文件 | 仅工作区 |
| **write_file** | 写入文件 | 仅工作区 |
| **list_dir** | 列出目录 | 仅工作区 |
| **edit_file** | 编辑文件 | 仅工作区 |
| **append_file** | 追加内容 | 仅工作区 |

### 3. 命令执行工具

| 工具 | 说明 | 保护机制 |
|-----|------|---------|
| **exec** | 执行命令 | 路径限制 + 危险命令拦截 |

### 4. 消息工具

| 工具 | 说明 |
|-----|------|
| **message** | 发送消息到指定渠道 |
| **spawn** | 创建异步子代理 |

### 5. 硬件工具（Linux 特有）

| 工具 | 说明 |
|-----|------|
| **i2c** | I2C 设备通信 |
| **spi** | SPI 设备通信 |

### 6. 技能工具

| 工具 | 说明 |
|-----|------|
| **find_skills** | 发现和搜索技能 |
| **install_skill** | 安装技能 |

### 7. 定时任务工具

| 工具 | 说明 |
|-----|------|
| **cron** | 定时提醒和任务 |

---

## 安全特性

### 1. 工作区沙盒

- **限制文件访问**：仅允许访问配置的工作区目录
- **限制命令执行**：命令路径必须在工作区内
- **一致性保证**：所有执行路径（主代理、子代理、心跳任务）共享相同限制

### 2. 危险命令拦截

以下命令被自动阻止：

- 批量删除：`rm -rf`、`del /f`、`rmdir /s`
- 磁盘格式化：`format`、`mkfs`、`diskpart`
- 磁盘成像：`dd if=`
- 直接磁盘写入：`/dev/sd[a-z]`
- 系统关机：`shutdown`、`reboot`、`poweroff`
- Fork 炸弹：`:(){ :|:& };:`

### 3. 可选的安全模式

- 默认启用工作区限制
- 可通过配置或环境变量禁用

---

## 定时任务功能

### 心跳任务（Heartbeat）

- **自动执行**：每 30 分钟（可配置）检查并执行任务
- **任务定义**：在 `HEARTBEAT.md` 文件中定义
- **异步执行**：长时间任务使用 spawn 创建子代理

### Cron 定时任务

- **一次性提醒**：`10分钟后提醒我`
- **周期性任务**：`每2小时提醒我`
- **Cron 表达式**：`每天9点提醒我`
- **任务存储**：保存在 `~/.picoclaw/workspace/cron/`

---

## 工作区结构

```
~/.picoclaw/workspace/
├── sessions/          # 对话会话和历史
├── memory/           # 长期记忆 (MEMORY.md)
├── state/            # 持久状态（最后渠道等）
├── cron/             # 定时任务数据库
├── skills/          # 自定义技能
├── AGENTS.md        # 代理行为指南
├── HEARTBEAT.md     # 周期性任务提示（每30分钟检查）
├── IDENTITY.md      # 代理身份
├── SOUL.md          # 代理灵魂
├── TOOLS.md         # 工具描述
└── USER.md          # 用户偏好设置
```

---

## CLI 命令

| 命令 | 说明 |
|-----|------|
| `picoclaw onboard` | 初始化配置和工作区 |
| `picoclaw agent -m "..."` | 与代理对话 |
| `picoclaw agent` | 交互式聊天模式 |
| `picoclaw gateway` | 启动网关服务 |
| `picoclaw status` | 显示状态 |
| `picoclaw cron list` | 列出所有定时任务 |
| `picoclaw cron add ...` | 添加定时任务 |
| `picoclaw skills list` | 列出已安装技能 |
| `picoclaw skills install` | 安装技能 |
| `picoclaw auth login` | 登录认证 |

---

## Docker 支持

### 标准部署

```bash
docker compose --profile gateway up -d
```

### Agent 模式（一次性）

```bash
docker compose run --rm picoclaw-agent -m "What is 2+2?"
```

---

## 性能对比

| 指标 | OpenClaw | NanoBot | **PicoClaw** |
|------|----------|---------|--------------|
| **语言** | TypeScript | Python | **Go** |
| **内存** | >1GB | >100MB | **<10MB** |
| **启动时间**（0.8GHz） | >500s | >30s | **<1s** |
| **成本** | Mac Mini $599 | Linux SBC ~$50 | **任意开发板 ~$10** |

---

## 配置选项

### 代理默认配置

```json
{
  "agents": {
    "defaults": {
      "workspace": "~/.picoclaw/workspace",
      "model_name": "gpt-4",
      "max_tokens": 8192,
      "temperature": 0.7,
      "max_tool_iterations": 20,
      "restrict_to_workspace": true
    }
  }
}
```

### 心跳配置

```json
{
  "heartbeat": {
    "enabled": true,
    "interval": 30
  }
}
```

---

## 开发特性

- **易于贡献**：代码库小且可读性强
- **模块化设计**：清晰的组件划分
- **全面测试**：包含单元测试和集成测试
- **CI/CD 集成**：GitHub Actions 自动构建
