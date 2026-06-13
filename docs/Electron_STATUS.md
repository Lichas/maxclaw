# Electron Desktop Status

> Last refreshed: 2026-06-13
> Scope: current implementation snapshot

本文档记录桌面端当前已经落地的能力、明确未落地的能力，以及文档维护口径。

## 总体状态

| 领域 | 当前状态 | 备注 |
|------|----------|------|
| Gateway 生命周期管理 | 已完成 | READY 协议、端口清理、自动重启 |
| 核心聊天体验 | 已完成 | 流式响应、Mermaid、会话管理、技能选择 |
| 桌面增强能力 | 已完成 | 托盘、通知、快捷键、自动更新、终端 |
| 配置与集成页 | 已完成 | Providers、Channels、Gateway 设置 |
| 多平台打包 | 已配置 | macOS / Windows / Linux 构建目标已存在 |
| 高级平台特性 | 部分实现 | 仅保留已落地项，不再按计划项算完成 |

## 主要已实现能力

### Chat / Sessions

- 消息流与流式响应
- Markdown + 代码高亮
- Mermaid 图表渲染
- 技能多选、`@` 提及、快捷命令
- 会话列表、删除、重命名、搜索
- 文件附件
- 文件预览侧边栏
- 内嵌终端面板

### Settings / Config

- Provider 配置编辑
- Gateway 配置读取与保存
- execution mode / max tool iterations 编辑
- 渠道配置面板
- 语言 / 主题 / think tag 渲染配置
- 自动启动、最小化到托盘、通知开关
- 快捷键配置

### Desktop integrations

- Gateway child process 管理
- 系统托盘
- 系统通知
- 自动更新
- 全局快捷键
- 数据导入导出

## 与代码一致的技术栈

| 类别 | 当前实现 |
|------|----------|
| Shell | Electron 28 |
| Frontend | React 18、Redux Toolkit、Vite |
| Local settings | electron-store |
| Markdown | react-markdown、remark-gfm |
| Diagram | mermaid |
| Terminal | node-pty、xterm |
| Updater | electron-updater |

来源：
- [electron/package.json](/Users/lua/git/maxclaw/electron/package.json:1)

## 明确未实现 / 不应再声称已支持

下面这些内容以前在设计稿或旧状态文档里出现过，但当前仓库不能把它们视作“已支持”：

- `better-sqlite3` 本地缓存层
- Headless UI
- Heroicons 作为实际依赖
- `remark-math` / `rehype-katex`
- DOMPurify 作为显式集成依赖
- Linux `rpm`
- Linux `snap`
- `maxclaw://` 自定义协议注册
- Windows 缩略图工具栏

## 平台特性现状

### Windows

已实现：
- Jump List
- User Tasks
- Taskbar progress bar
- NSIS / Portable 打包目标

未发现当前实现：
- 协议注册
- 缩略图工具栏

### macOS

已实现：
- Dock / 主窗口集成
- 托盘图标
- `.dmg` / `.zip` 打包目标

### Linux

已实现：
- `AppImage` / `.deb` 打包目标

未配置：
- `rpm`
- `snap`

## 仍待补充的可选能力

- 本地缓存清理入口
- 更细粒度的应用内快捷键自定义
- 二维码登录类渠道辅助流程

这些项在代码落地前，不应写成完成态。

## 文档维护规则

更新本文件时请遵守：

1. 只写仓库中已经存在并可验证的能力
2. 技术栈以 `package.json` / 实际源码为准
3. 平台支持以 `electron-builder.yml` 为准
4. 计划中的能力放到计划文档，不放到 status 文档

## 相关文件

- [electron/src/main/index.ts](/Users/lua/git/maxclaw/electron/src/main/index.ts:1)
- [electron/src/main/gateway.ts](/Users/lua/git/maxclaw/electron/src/main/gateway.ts:1)
- [electron/src/main/windows-integration.ts](/Users/lua/git/maxclaw/electron/src/main/windows-integration.ts:1)
- [electron/src/renderer/views/ChatView.tsx](/Users/lua/git/maxclaw/electron/src/renderer/views/ChatView.tsx:1)
- [electron/src/renderer/views/SettingsView.tsx](/Users/lua/git/maxclaw/electron/src/renderer/views/SettingsView.tsx:1)
- [electron/electron-builder.yml](/Users/lua/git/maxclaw/electron/electron-builder.yml:1)
