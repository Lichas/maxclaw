# Browser Tool Operations Runbook

本手册用于 nanobot-go 的浏览器能力落地，目标是稳定处理 X/Twitter 这类强 JS、需要登录的站点。

## 1. 适用范围

- 读取需要登录态的页面内容
- 多步骤页面交互（点击、输入、切换标签页）
- 产出可追溯证据（截图路径）

## 2. 前置条件

1. 安装 Playwright 依赖：
   ```bash
   make webfetch-install
   ```
2. 配置 `~/.nanobot/config.json`：
   - `tools.web.fetch.mode = "chrome"`
   - `tools.web.fetch.scriptPath = "/absolute/path/to/nanobot-go/webfetcher/fetch.mjs"`
   - `tools.web.fetch.nodePath = "node"`
3. 启动网关：
   ```bash
   ./build/nanobot-go gateway
   ```

## 3. 登录态初始化（一次性）

首次使用某个 profile 前，必须手动登录一次：

```bash
./build/nanobot-go browser login https://x.com
```

执行后会打开受管 profile（默认目录：`~/.nanobot/browser/chrome/user-data`）：
- 在弹出的浏览器中完成登录
- 回到终端按 Enter 结束登录会话

说明：
- 后续 `web_fetch(mode=chrome)` 和 `browser` 工具都会复用同一 profile。
- 需要隔离不同账号时，使用 `--profile` 或 `--user-data-dir`。

## 4. 实战流程（聊天里调用）

### 4.1 快速读取页面（单步）

适合只读页面正文：
- 让 agent 调用 `web_fetch`，URL 指向目标页面。

### 4.2 多步骤交互（推荐 `browser`）

适合搜索、点按钮、翻页、截图：

1. 导航
   - `action="navigate", url="https://x.com/home"`
2. 抓快照
   - `action="snapshot"`
   - 返回内容会包含 `[ref]` 引用（可点击/可输入元素）
3. 执行动作
   - 点击：`action="act", act="click", ref=12`
   - 输入：`action="act", act="type", ref=5, text="OpenAI"`
   - 回车：`action="act", act="press", ref=5, key="Enter"`
4. 标签页管理（可选）
   - 列表：`action="tabs", tab_action="list"`
   - 切换：`action="tabs", tab_action="switch", tab_index=1`
5. 保存截图
   - `action="screenshot"`（自动路径）
   - 或 `action="screenshot", path="/absolute/path/result.png"`

## 5. 会话与状态机制

- 浏览器状态按 `channel + chat_id` 维度隔离。
- 每个会话保存：
  - 当前活动标签页索引
  - 最近一次 `snapshot` 的引用表（`ref -> selector`）
- 状态文件路径：
  - `~/.nanobot/browser/sessions/<session>.json`

## 6. 常见问题排查

### 6.1 页面仍显示未登录

排查顺序：
1. 是否先执行过 `browser login` 并在该 profile 中登录成功
2. `config.json` 的 `profileName/userDataDir` 是否与登录时一致
3. 是否误切回了 `mode=http`（应使用 `mode=chrome`）

### 6.2 CDP 连接失败

现象：
- 错误包含 `CDP connect failed`

处理：
1. 若要接管本机已有 Chrome，会话需以 `--remote-debugging-port=9222` 启动
2. 或者清空 `cdpEndpoint`，直接走受管 profile（推荐稳定方案）

### 6.3 `act` 失败（找不到元素）

处理：
1. 先重新执行一次 `snapshot`
2. 使用新的 `ref`
3. 或直接提供更精确的 `selector`

### 6.4 截图找不到文件

默认输出目录：
- `~/.nanobot/browser/screenshots/`

建议：
- 在 `screenshot` 时显式传 `path` 到你可控目录

## 7. 推荐生产策略

1. 默认使用受管 profile（稳定、可复现）
2. 只有确实需要共享当前桌面会话时才用 CDP endpoint
3. 对关键任务强制要求 `screenshot` 作为回执证据
