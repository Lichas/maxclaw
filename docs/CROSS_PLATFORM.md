# 多平台支持文档

本文档描述当前仓库里 Electron Desktop 的实际跨平台支持范围，以现有代码、打包配置和 CI 为准。

## 当前支持范围

| 平台 | 安装包 / 产物 | 当前状态 | 说明 |
|------|---------------|----------|------|
| Windows | NSIS `.exe`、Portable `.exe` | 已配置 | 通过 `electron-builder` 构建 |
| macOS | `.dmg`、`.zip` | 已配置 | 通过 `electron-builder` 构建 |
| Linux | `AppImage`、`.deb` | 已配置 | 通过 `electron-builder` 构建 |

当前 `electron-builder` 配置见 [electron/electron-builder.yml](/Users/lua/git/maxclaw/electron/electron-builder.yml:1)。

不应再视为当前已支持的能力：

- Linux `rpm`
- Linux `snap`
- `maxclaw://` 自定义协议注册
- Windows 缩略图工具栏

这些能力如果未来要补，应以代码和打包配置落地后再更新本文档。

## 已实现的平台特性

### 通用

- Electron 主进程负责启动和守护内嵌 Go Gateway
- Gateway 启动前会清理目标端口占用
- Gateway 通过 `READY:host:port` stdout 协议向 Electron 报告就绪
- Gateway 异常退出后会指数退避重启
- 支持自动更新检查与下载通知（生产环境）
- 支持全局快捷键
- 支持系统托盘 / 菜单栏驻留

### Windows

- Jump List 快捷任务
- User Tasks
- Taskbar progress bar
- NSIS 安装向导
- Portable 便携包

实现位置：
- [electron/src/main/windows-integration.ts](/Users/lua/git/maxclaw/electron/src/main/windows-integration.ts:1)
- [electron/electron-builder.yml](/Users/lua/git/maxclaw/electron/electron-builder.yml:26)

### macOS

- Dock 图标与应用菜单
- 托盘图标
- `.dmg` / `.zip` 打包目标

实现位置：
- [electron/src/main/window.ts](/Users/lua/git/maxclaw/electron/src/main/window.ts:1)
- [electron/src/main/tray.ts](/Users/lua/git/maxclaw/electron/src/main/tray.ts:1)
- [electron/electron-builder.yml](/Users/lua/git/maxclaw/electron/electron-builder.yml:20)

### Linux

- `AppImage` / `.deb` 打包目标
- 托盘、快捷键、Gateway 管理等通用能力沿用 Electron 通用实现

实现位置：
- [electron/electron-builder.yml](/Users/lua/git/maxclaw/electron/electron-builder.yml:52)

## 开发注意事项

### 路径与平台判断

- 使用 Node.js `path` 和 `url` 工具处理跨平台路径
- 使用 `process.platform` 做平台分支
- 不要在文档或代码中假设所有平台拥有相同的系统集成能力

### 自动启动

桌面端通过 `auto-launch` 处理开机自启动。是否启用由设置页中的本地配置控制。

相关代码：
- [electron/src/main/index.ts](/Users/lua/git/maxclaw/electron/src/main/index.ts:1)
- [electron/src/renderer/views/SettingsView.tsx](/Users/lua/git/maxclaw/electron/src/renderer/views/SettingsView.tsx:1)

### 自动更新

- 仅在生产环境启用
- 发布源配置为 GitHub Releases
- 渲染进程通过 preload 暴露的 `update` API 获取更新状态

相关代码：
- [electron/src/main/index.ts](/Users/lua/git/maxclaw/electron/src/main/index.ts:115)
- [electron/src/preload/index.ts](/Users/lua/git/maxclaw/electron/src/preload/index.ts:101)
- [electron/electron-builder.yml](/Users/lua/git/maxclaw/electron/electron-builder.yml:58)

## 构建与 CI

### 本地打包

```bash
make build
cd electron
npm install
npm run dist:mac
npm run dist:win
npm run dist:linux
```

### CI 工作流

仓库当前包含两个相关工作流：

- `build-desktop.yml`
  - 构建桌面端多平台安装包
  - 上传 macOS / Windows / Linux 桌面产物
- `release-artifacts.yml`
  - 构建 CLI 发布包并发布 GitHub Release 附件

对应文件：
- [build-desktop.yml](/Users/lua/git/maxclaw/.github/workflows/build-desktop.yml:1)
- [release-artifacts.yml](/Users/lua/git/maxclaw/.github/workflows/release-artifacts.yml:1)

## 文档维护约定

更新本文档时，请只记录“仓库中已经存在并可验证”的能力：

- 以 `electron-builder.yml` 为准描述安装包类型
- 以 `electron/src/main/` 下实际实现为准描述平台集成
- 以 GitHub Actions 工作流为准描述 CI/CD

如果某项能力只是计划、草案或注释，请放到设计文档里，不要写进这里。
