# 多平台支持文档

本文档描述 Maxclaw Desktop 的多平台支持实现，包括 Windows、macOS 和 Linux。

## 支持的平台

| 平台 | 最低版本 | 安装包类型 | 自动更新 |
|------|---------|-----------|---------|
| Windows | Windows 10 | NSIS (.exe), Portable | ✅ |
| macOS | macOS 11 (Big Sur) | DMG, ZIP | ✅ |
| Linux | Ubuntu 20.04+, Fedora 35+ | AppImage, DEB, RPM | ✅ (AppImage) |

## 平台特定功能

### Windows

#### 任务栏集成
- **跳转列表 (Jump List)**: 右键任务栏图标显示快捷操作
  - New Chat
  - Settings
- **任务栏进度条**: 显示操作进度
- **缩略图工具栏**: 窗口预览时的快捷按钮

#### 安装程序 (NSIS)
- 自定义安装向导
- 选择安装目录
- 创建桌面和开始菜单快捷方式
- 注册 `maxclaw://` 协议
- 卸载时可选删除用户数据

#### 可执行文件名
- 安装版: `Maxclaw-${version}-Setup.exe`
- 便携版: `Maxclaw-${version}-Portable.exe`

### macOS

#### Dock 集成
- 自定义 Dock 图标
- 应用菜单
- 状态栏托盘图标（支持模板图片）

#### 安装包
- DMG: 拖拽安装
- ZIP: 直接解压

#### 代码签名
- 支持 Apple 代码签名
- 支持公证 (Notarization)

### Linux

#### 桌面集成
- Desktop 文件注册
- 图标主题集成
- MIME 类型关联

#### 安装包
- AppImage: 免安装，双击运行
- DEB: Debian/Ubuntu 系
- RPM: Fedora/RHEL 系
- Snap: 通用 Linux 包

## 开发注意事项

### 路径处理
使用 Node.js 的 `path` 模块确保跨平台兼容：

```typescript
import path from 'path';

// ✅ 正确：使用 path.join
const configPath = path.join(app.getPath('userData'), 'config.json');

// ❌ 错误：硬编码路径分隔符
const configPath = `${app.getPath('userData')}/config.json`;
```

### 平台检测
```typescript
if (process.platform === 'win32') {
  // Windows 特定代码
} else if (process.platform === 'darwin') {
  // macOS 特定代码
} else if (process.platform === 'linux') {
  // Linux 特定代码
}
```

### 自动启动
使用 `auto-launch` 库处理跨平台自动启动：

```typescript
import AutoLaunch from 'auto-launch';

const autoLauncher = new AutoLaunch({
  name: 'Maxclaw',
  path: app.getPath('exe')
});

// 启用
autoLauncher.enable();

// 禁用
autoLauncher.disable();
```

### 文件路径协议
- **Windows**: `file:///C:/Users/...`
- **macOS/Linux**: `file:///home/user/...`

使用 `pathToFileURL` 转换：

```typescript
import { pathToFileURL } from 'node:url';

const fileUrl = pathToFileURL(filePath).toString();
```

## 构建配置

### electron-builder.yml

```yaml
win:
  icon: assets/icon.ico
  target:
    - nsis      # 安装程序
    - portable  # 便携版

nsis:
  oneClick: false                      # 显示安装向导
  allowToChangeInstallationDirectory: true
  createDesktopShortcut: always
  createStartMenuShortcut: true
  shortcutName: 'Maxclaw AI'
  artifactName: 'Maxclaw-${version}-Setup.${ext}'
  include: build/installer.nsh         # 自定义脚本

mac:
  icon: assets/icon.icns
  category: public.app-category.productivity
  target:
    - dmg
    - zip

linux:
  icon: assets/icon.png
  category: Office
  target:
    - AppImage
    - deb
    - rpm
```

## CI/CD 构建

GitHub Actions 工作流自动构建多平台安装包：

- **触发条件**: Push 到 main 分支、打 tag、手动触发
- **构建矩阵**:
  - macOS (macos-latest)
  - Windows (windows-latest)
  - Linux (ubuntu-latest)
- **产物**: 自动上传到 Artifacts，打 tag 时发布 Release

### 手动触发构建

1. 进入 Actions 页面
2. 选择 "Build Desktop Apps"
3. 点击 "Run workflow"
4. 选择构建类型 (all/macos/windows/linux)

## 发布流程

1. 更新版本号 (`electron/package.json`)
2. 打 tag: `git tag v1.x.x`
3. 推送 tag: `git push origin v1.x.x`
4. GitHub Actions 自动构建并创建 Draft Release
5. 编辑 Release 说明
6. 发布 Release

## 常见问题

### Windows 安装警告
Windows Defender 或 SmartScreen 可能显示未知发布者警告。解决方案：
- 购买代码签名证书
- 使用 EV 证书获得即时信任
- 等待 SmartScreen 积累声誉

### macOS 无法打开
如果显示 "无法打开，因为无法验证开发者"：
- 右键点击应用 → 打开
- 或在系统设置 → 隐私与安全中允许

### Linux 权限
AppImage 需要执行权限：
```bash
chmod +x Maxclaw-*.AppImage
```

## 参考文档

- [Electron 跨平台文档](https://www.electronjs.org/docs/latest/tutorial/supported-platforms)
- [electron-builder 配置](https://www.electron.build/configuration/configuration)
- [auto-launch 文档](https://github.com/Teamwork/node-auto-launch)
