# 竞品桌面 Agent CoWork App - 产品特性分析与 Electron 开发需求文档

## 一、竞品产品特性梳理

基于截图分析，这是一个**AI Agent 协作工作台**，核心特性如下：

### 1. 核心交互层

| 特性 | 描述 |
|------|------|
| **多轮对话** | 类 ChatGPT 的聊天界面，支持文本输入 |
| **技能调用** | 输入框可附加 Skill（docx/web-search/xlsx/pptx/pdf 等） |
| **快捷操作** | 底部快捷按钮（制作幻灯片、数据分析、教育学习、创建网站） |
| **项目管理** | 支持绑定工作目录（project），文件附件 |
| **模型切换** | 顶部可切换不同 AI 模型（DeepSeek Chat） |

### 2. 任务系统

| 特性 | 描述 |
|------|------|
| **任务创建** | 新建 AI 任务/对话 |
| **任务搜索** | 全局搜索历史任务（标题+内容） |
| **任务历史** | 左侧任务记录列表（显示耗时、完成状态） |
| **定时任务** | 创建定时执行的自动化任务 |
| - 标题/提示词 | 任务的名称和执行指令 |
| - 调度计划 | 重复规则（不重复/每日/每周/每月/Cron）、日期、时间 |
| - 工作目录 | 任务执行的上下文路径 |
| - 到期时间 | 任务截止设置 |
| - 通知渠道 | 钉钉等 IM 通知 |

### 3. 技能系统（Skill Marketplace）

| 特性 | 描述 |
|------|------|
| **技能卡片** | 可视化展示已安装技能，带开关控制 |
| **技能导入** | 上传 .zip、上传文件夹、从 GitHub 导入 |
| **内置技能** | docx、xlsx、pptx、pdf、web-search、scheduled-task、web-game 等 |
| **技能元信息** | 名称、描述、图标、启用状态、安装时间 |

### 4. 模型配置系统

| 特性 | 描述 |
|------|------|
| **多提供商支持** | DeepSeek、Moonshot、Qwen、Zhipu、MiniMax、Ollama |
| **独立配置** | 每个提供商可设置 API Key、Base URL |
| **协议兼容** | 支持 Anthropic / OpenAI 格式切换 |
| **连接测试** | 一键测试 API 连通性 |
| **模型管理** | 自定义添加可用模型列表 |
| **导入导出** | 配置备份与恢复 |

### 5. 集成与通知

| 特性 | 描述 |
|------|------|
| **邮箱集成** | IMAP/SMTP 配置，支持自定义服务商 |
| **IM Bot** | 钉钉、飞书、Telegram、Discord 机器人配置 |
| **系统级通知** | 任务完成通知推送 |

### 6. 系统设置

| 特性 | 描述 |
|------|------|
| **国际化** | 语言切换（中文/英文） |
| **自启动** | 开机自启动开关 |
| **主题切换** | 浅色/深色/跟随系统 |
| **快捷键** | 自定义键盘快捷键 |

---

## 二、Electron App 开发需求文档

### 1. 项目概述

**核心定位**：Electron Desktop App 作为 **maxclaw Gateway 的桌面端封装**，提供原生桌面体验，而非重写 Agent 逻辑。

**架构原则**：
- **复用现有能力**：maxclaw Gateway 提供完整的 Agent Loop、WebUI Server、Cron Service、Channels
- **桌面增强**：Electron 提供系统级能力（托盘、自启动、通知、文件系统访问）
- **UI 升级**：基于现有 webui 进行桌面化改造，提供更丰富的交互体验

### 2. 技术栈选型（基于 LobsterAI 文档优化）

#### 2.1 核心技术

| 技术 | 版本 | 用途 |
|------|------|------|
| **Electron** | 40.2.1+ | 桌面应用框架，提供跨平台能力 |
| **React** | 18.2.0+ | UI 框架 |
| **TypeScript** | 5.7.3+ | 类型安全 |
| **Vite** | 5.1.4+ | 前端构建工具，提供快速 HMR |

#### 2.2 状态管理
| 技术 | 用途 |
|------|------|
| **Redux Toolkit** | 应用状态管理（chat、cowork、artifacts、scheduled tasks）|
| **better-sqlite3** | 本地 SQLite 数据库（UI 状态缓存、离线消息队列）|

#### 2.3 UI 组件与样式
| 技术 | 用途 |
|------|------|
| **Tailwind CSS** | Utility-first CSS 框架 |
| **Headless UI** | 无样式、可访问的 UI 组件库 |
| **Heroicons** | SVG 图标库 |

#### 2.4 AI & LLM 集成
| 技术 | 用途 |
|------|------|
| **SSE (Server-Sent Events)** | 流式响应，实时对话 |

#### 2.5 Markdown & 内容渲染
| 技术 | 用途 |
|------|------|
| **react-markdown** | Markdown 渲染 |
| **remark-gfm** | GitHub Flavored Markdown 支持 |
| **remark-math + rehype-katex** | LaTeX 数学公式渲染 |
| **mermaid** | 流程图、时序图渲染 |
| **react-syntax-highlighter** | 代码语法高亮 |
| **DOMPurify** | HTML/SVG 内容消毒，防止 XSS |

#### 2.6 IM Bot 集成（复用 maxclaw，Electron 侧仅配置 UI）
| 技术 | 平台 | 用途 |
|------|------|------|
| **dingtalk-stream** | 钉钉 | 钉钉机器人流式网关 |
| **@larksuiteoapi/node-sdk** | 飞书 | 飞书开放平台 SDK |
| **grammy + @grammyjs/runner** | Telegram | Telegram Bot 框架 |
| **discord.js** | Discord | Discord Bot 库 |

#### 2.7 任务调度
| 技术 | 用途 |
|------|------|
| **cron-parser** | Cron 表达式解析，用于定时任务展示 |

#### 2.8 构建与工具
| 技术 | 用途 |
|------|------|
| **electron-builder** | Electron 应用打包工具 |
| **ESLint** | 代码质量检查 |
| **patch-package** | 第三方包补丁管理 |
| **concurrently** | 并发运行多个命令 |
| **wait-on** | 等待端口/文件就绪 |

### 3. 架构设计

#### 3.1 进程架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    Electron Main Process                         │
│  ─────────────────────────────────────────────────────────────  │
│  • 窗口管理 (Window Manager)                                     │
│  • 系统托盘 (Tray)                                               │
│  • 自动启动 (Auto Launcher)                                      │
│  • Gateway 进程管理 (Child Process)                              │
│    - 启动/停止 maxclaw gateway                                │
│    - 健康检查与自动重启                                          │
│  • 本地 SQLite 缓存 (UI 状态、离线队列)                          │
│  • IPC 桥接 (上下文隔离)                                         │
│  • 系统级通知 (Notifications)                                    │
└─────────────────────────────────────────────────────────────────┘
                            │ IPC
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Electron Renderer Process                      │
│  ─────────────────────────────────────────────────────────────  │
│  • React Application                                             │
│  • Redux State Management                                        │
│  • UI Components                                                 │
│  • HTTP Client → Gateway API (/api/*)                            │
│  • WebSocket Client → 实时消息推送                               │
│  • Services Layer                                                │
└─────────────────────────────────────────────────────────────────┘
                            │ HTTP / WebSocket
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│              maxclaw Gateway (Child Process)                  │
│  ─────────────────────────────────────────────────────────────  │
│  • Web UI Server (port 18890)                                    │
│  • Agent Loop (对话处理、工具调用)                               │
│  • Cron Service (定时任务执行)                                   │
│  • Channel Registry (多平台消息通道)                             │
│  • Session Manager (会话持久化)                                  │
│  • Skills Loader (技能系统)                                      │
│  • Memory Summarizer (每日记忆汇总)                              │
└─────────────────────────────────────────────────────────────────┘
```

#### 3.2 数据流

```
用户操作 → React UI → Redux → IPC → Main Process
                                          ↓
                              ┌───────────┴───────────┐
                              ▼                       ▼
                      Gateway Client             SQLite Cache
                              ↓
                     maxclaw Gateway
                              ↓
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
         Agent Loop     Cron Service    Channels
              ↓               ↓               ↓
         LLM Provider    Job Handler    IM Platforms
```

### 4. 与 maxclaw 集成方案

#### 4.1 Gateway 进程管理

```typescript
// Electron Main Process - Gateway 管理器
interface GatewayManager {
  // 启动 Gateway 子进程
  start(): Promise<void>;
  
  // 停止 Gateway
  stop(): Promise<void>;
  
  // 重启 Gateway
  restart(): Promise<void>;
  
  // 健康检查
  healthCheck(): Promise<boolean>;
  
  // 获取 Gateway 状态
  getStatus(): 'running' | 'stopped' | 'error';
}

// 实现细节
// - 通过 child_process.spawn 启动 maxclaw gateway
// - 监听 stdout/stderr 进行日志收集
// - 轮询 http://localhost:18890/api/status 进行健康检查
// - 崩溃时自动重启（带指数退避）
```

#### 4.2 API 客户端封装

```typescript
// Renderer Process - Gateway API 客户端
class GatewayClient {
  baseURL: string = 'http://localhost:18890';
  
  // 对话相关
  async sendMessage(payload: MessagePayload): Promise<StreamResponse>;
  async getSessions(): Promise<Session[]>;
  async getSession(key: string): Promise<Session>;
  
  // 配置相关
  async getConfig(): Promise<Config>;
  async updateConfig(config: Config): Promise<Config>;
  
  // 状态相关
  async getStatus(): Promise<GatewayStatus>;
  async restartGateway(): Promise<void>;
  
  // 流式响应
  streamMessage(payload: MessagePayload, onDelta: (delta: string) => void): Promise<void>;
}
```

#### 4.3 实时消息推送

```typescript
// WebSocket 连接 Gateway 的 WebSocket Channel
// 用于接收：
// - 新消息通知（来自 Telegram/Discord 等）
// - 定时任务执行结果
// - 系统事件

interface WebSocketClient {
  connect(): void;
  disconnect(): void;
  onMessage(handler: (msg: ChannelMessage) => void): void;
}
```

#### 4.4 配置同步机制

```typescript
// Electron App 配置与 maxclaw 配置的映射

// ~/.maxclaw/config.json (maxclaw 配置)
interface NanobotConfig {
  agents: {
    defaults: {
      workspace: string;
      model: string;
      maxTokens: number;
      temperature: number;
    };
  };
  providers: Record<string, ProviderConfig>;
  channels: ChannelsConfig;
  tools: ToolsConfig;
}

// Electron App 本地配置 (SQLite)
interface AppConfig {
  // 窗口状态
  windowBounds: { width: number; height: number; x: number; y: number };
  // UI 设置
  theme: 'light' | 'dark' | 'system';
  language: 'zh' | 'en';
  // 系统设置
  autoLaunch: boolean;
  minimizeToTray: boolean;
  // 快捷键
  shortcuts: Record<string, string>;
}
```

### 5. 功能模块需求

#### 5.1 主界面模块 (Main Workspace)

##### 5.1.1 聊天界面
- [x] 消息列表（支持 Markdown、代码高亮）
- [x] 输入框（富文本、@提及、快捷命令 `/new`, `/help`）
- [x] 技能选择器（下拉菜单，支持搜索过滤）
- [x] 文件附件（拖拽上传、文件选择、随消息生效）
- [x] 快捷操作栏（4个任务模板卡片）
- [x] 模型切换下拉框（调用 Gateway API 获取可用模型）
- [x] 流式响应显示（SSE 实时渲染）
- [x] Mermaid 图表渲染（流程图、时序图、类图等）

##### 5.1.2 侧边栏
- [x] 新建任务按钮
- [x] 搜索任务入口（全局搜索，调用 Gateway API）
- [x] 菜单导航（定时任务、技能、设置）
- [x] 任务历史列表（从 Gateway 获取会话列表）
- [x] 会话删除/重命名

#### 5.2 任务系统模块

##### 5.2.1 普通任务（复用 Gateway Session）
- [x] 创建/删除/重命名对话（调用 `/api/sessions`）
- [x] 对话历史展示（从 Gateway 获取）
- [x] 消息搜索（Gateway 提供全文检索接口）

##### 5.2.2 定时任务（复用 Gateway Cron Service）
- [x] 任务创建表单
  - [x] 标题输入
  - [x] 提示词编辑器
  - [ ] Cron 表达式生成器（可视化选择器）⚠️ 文本输入已实现
  - [x] 工作目录选择（文件浏览器对话框）
  - [ ] 到期时间（可选）❌ 未实现
  - [ ] 通知渠道选择（多选）❌ 未实现
- [x] 任务列表管理（调用 Gateway Cron API）
- [x] 执行历史日志（从 Gateway 获取）⚠️ 仅显示上次执行时间

#### 5.3 技能系统模块

##### 5.3.1 技能管理（复用 Gateway Skills Loader）
- [x] 技能网格展示（读取 `<workspace>/skills` 目录）
- [x] 技能安装
  - [x] .zip 文件导入 → 解压到 skills 目录
  - [x] 文件夹导入
  - [x] GitHub URL 导入（支持子目录 sparse checkout）
- [x] 技能开关（通过 Gateway API 启用/禁用）
- [x] 技能配置文件解析（`SKILL.md` YAML frontmatter）

#### 5.4 模型配置模块

##### 5.4.1 提供商管理（复用 Gateway Provider 配置）
- [x] 预设提供商列表（DeepSeek、OpenAI、Anthropic、Moonshot、Groq、Gemini）
- [x] 添加自定义提供商
- [x] 启用/禁用切换
- [x] 配置字段编辑
- [x] **关键**：配置修改后调用 Gateway `/api/config` 更新并重启 Gateway

##### 5.4.2 连接测试
- [x] 一键测试 API 连通性（通过 Gateway 代理）
- [x] 显示延迟和状态

#### 5.5 集成设置模块

##### 5.5.1 邮箱配置（复用 Gateway Email Channel）
- [x] 服务商预设（Gmail、Outlook、QQ邮箱、163邮箱、自定义）
- [x] IMAP/SMTP 配置
- [x] 连接测试

##### 5.5.2 IM Bot 配置（复用 Gateway Channels）
- [x] 多平台配置面板：Telegram、Discord、WhatsApp、飞书、钉钉、Slack、QQ
- [x] 连接状态显示（从 Gateway `/api/status` 获取）
- [ ] 二维码登录（WhatsApp、Telegram）❌ 未实现

#### 5.6 系统设置模块

##### 5.6.1 通用设置
- [x] 语言切换（i18n 框架，完整中英文支持）
- [x] 开机自启动（electron-auto-launch）
- [x] 主题切换（Light/Dark/System，深色主题柔和色调）
- [x] 最小化到托盘
- [x] 系统通知（定时任务完成、新消息推送）

##### 5.6.2 快捷键
- [ ] 全局快捷键注册（呼出/隐藏窗口）❌ 未实现
- [ ] 应用内快捷键自定义❌ 未实现

##### 5.6.3 数据管理
- [ ] Gateway 配置导入/导出❌ 未实现
- [ ] 本地缓存清理❌ 未实现

### 6. 开发里程碑

#### Phase 1: 基础框架 (2周)
- Electron + React + TypeScript + Vite 项目搭建
- 与 maxclaw Gateway 集成（进程管理、健康检查）
- 基础窗口管理（托盘、自启动）
- 复用现有 webui 作为初始界面

#### Phase 2: 核心功能增强 (3周)
- 流式响应优化（SSE 客户端）
- 文件拖拽上传集成
- 技能管理界面
- 模型配置 UI

#### Phase 3: 桌面特性 (2周)
- 系统级通知（任务完成、新消息）
- 全局快捷键
- 离线消息队列（SQLite 缓存）
- 自动更新机制

#### Phase 4: 多平台适配 (2周)
- macOS Dock/菜单栏集成
- Windows 任务栏集成
- Linux 适配
- 签名与打包

### 7. 关键实现细节

#### 7.1 Gateway 启动流程

```typescript
// Main Process
async function startApp() {
  // 1. 检查 maxclaw 二进制文件
  const gatewayBinary = await findGatewayBinary();
  
  // 2. 确保配置文件存在
  await ensureConfigExists();
  
  // 3. 启动 Gateway 子进程
  gatewayProcess = spawn(gatewayBinary, ['gateway', '-p', '18890'], {
    stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, NANOBOT_ELECTRON: '1' }
  });
  
  // 4. 等待 Gateway 就绪
  await waitOn({ resources: ['tcp:18890'], timeout: 30000 });
  
  // 5. 创建 Electron 窗口
  createWindow();
}
```

#### 7.2 流式消息处理

```typescript
// Renderer Process
async function sendStreamMessage(content: string) {
  const response = await fetch('http://localhost:18890/api/message', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, stream: true })
  });
  
  const reader = response.body?.getReader();
  while (reader) {
    const { done, value } = await reader.read();
    if (done) break;
    
    // 解析 SSE 数据
    const text = new TextDecoder().decode(value);
    const lines = text.split('\n');
    
    for (const line of lines) {
      if (line.startsWith('data: ')) {
        const delta = line.slice(6);
        dispatch(updateMessage(delta)); // Redux action
      }
    }
  }
}
```

### 8. 参考资源

- **maxclaw 架构**: `ARCHITECTURE.md`
- **maxclaw Gateway**: `internal/cli/gateway.go`, `internal/webui/server.go`
- **maxclaw Agent**: `internal/agent/loop.go`
- **UI 参考**: 竞品截图设计系统（浅色主题、圆角、卡片式布局）
- **竞品参考**: DeepSeek Chat、Cursor、Claude Desktop、LobsterAI

---

## 附录：maxclaw Gateway API 清单

| 端点 | 方法 | 用途 |
|------|------|------|
| `/api/status` | GET | Gateway 状态、Channels 状态、Cron 状态 |
| `/api/sessions` | GET | 会话列表 |
| `/api/sessions/:key` | GET | 会话详情 |
| `/api/message` | POST | 发送消息（支持流式响应）|
| `/api/config` | GET/PUT | 获取/更新配置 |
| `/api/gateway/restart` | POST | 重启 Gateway |

**WebSocket 端点**: `ws://localhost:18890/ws`（用于实时消息推送）
