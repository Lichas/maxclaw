# Electron Gateway 生命周期增强设计

## 背景与目标

当前 Electron Desktop App 在启动时会启动 maxclaw Gateway 子进程，但存在以下问题：

1. **端口冲突未覆盖所有场景**：`terminateExistingGatewayProcesses()` 只匹配特定命令行模式的 maxclaw 进程，如果 18890 被其他程序占用，Gateway 启动会失败且无法自动恢复。
2. **启动同步不可靠**：当前靠超时（10s）或 stdout 关键字猜测 Gateway 是否就绪，没有明确的"ready"信号。
3. **窗口显示时机不对**：窗口在 Gateway 启动失败后仍然显示，用户看到的是一个无法连接的界面。
4. **崩溃重启不清理端口**：`attemptRestart()` 直接重启，如果端口被其他进程抢占，重启仍然失败。

**目标**：实现一个鲁棒的 Gateway 生命周期管理系统，确保 Electron 启动时 Gateway 一定 ready，且在任何冲突场景下自动恢复。

---

## 方案选择

采用**端口强制独占模式（方案 A）**：

- 启动前无条件清理 18890 端口占用者（无论是否 maxclaw 进程）
- Go Gateway 输出 READY 协议确认就绪
- Electron 等待 READY 后再显示主窗口
- 崩溃重启前同样清理端口

不选方案 B（动态端口）是因为 renderer 层大量代码硬编码了 `localhost:18890`，全量改造成本过高。不选方案 C（混合）是因为增加了维护复杂度，而固定端口是更可控的方案。

---

## 详细设计

### 1. 端口占用检测与进程清理

新增 `cleanupPortLock(port: number): Promise<void>` 方法，封装跨平台端口→PID 查询与清理：

**macOS**
```
lsof -iTCP:<port> -sTCP:LISTEN -t -P
```
返回监听该端口的 PID 列表（排除当前进程）。

**Linux**
```
fuser <port>/tcp 2>/dev/null
```
或 fallback 到：
```
ss -tlnp 'sport = :<port>'
```

**Windows**
```
netstat -ano | findstr :<port>
```
解析最后一列 PID。

**清理流程**
1. 查询占用目标端口的所有 PID
2. 过滤掉当前 Electron 进程和当前管理的 Gateway 子进程
3. 对每个 PID 发送 `SIGTERM`（Windows: `taskkill /PID <pid>`）
4. 等待 1000ms，再次检测
5. 若仍有残留，发送 `SIGKILL`（Windows: `taskkill /F /PID <pid>`）
6. 最多尝试 3 轮，之后放弃并抛错

在 `start()` 的 spawn 前调用，在 `attemptRestart()` 的重启前调用。

### 2. READY 协议

**Go 侧**

在 Gateway HTTP server 成功启动后（`ListenAndServe` 返回前或 goroutine 中），向 stdout 输出：

```
READY:127.0.0.1:18890
```

格式严格为 `READY:<host>:<port>`，单行，无多余内容。

修改位置：`cmd/maxclaw-gateway/main.go` 或 `internal/cli/gateway.go` 中 server 启动后的位置。

**Electron 侧**

`GatewayManager.start()` 中：
1. `spawn()` 后监听 `stdout`
2. 用正则 `/^READY:(.+):(\d+)$/` 匹配每一行输出
3. 匹配成功后解析 host 和 port，更新 `this.status = { state: 'running', port: parsedPort }`
4. `resolve()`
5. 若 30 秒内未收到 READY，则 `reject(new Error('Gateway startup timeout'))`

原有的超时逻辑（10s 后 fallback health check）可以保留作为第二层保险，但主逻辑以 READY 为准。

### 3. 窗口生命周期集成

修改 `electron/src/main/index.ts` 的 `initializeApp()`：

```
initializeApp()
  ├── gatewayManager = new GatewayManager()
  ├── 先创建窗口（加载 loading.html）
  ├── 尝试 gatewayManager.start()
  │     ├── cleanupPortLock(18890)
  │     ├── spawn + 等 READY
  │     └── 返回 success/failure
  ├── 如果成功
  │     └── 窗口导航到主界面
  ├── 如果失败
  │     └── 窗口显示错误页面（含重试按钮）
```

**loading.html**：简单的静态页面，显示"正在启动服务..."和 spinner。

**错误页面**：显示具体的错误信息（二进制缺失 / 端口被系统进程占用 / 启动超时）和"查看日志"、"重试"按钮。

### 4. 崩溃自动重启增强

当前 `attemptRestart()` 逻辑：

```typescript
private async attemptRestart(): Promise<void> {
  if (this.restartAttempts >= this.maxRestartAttempts) {
    this.status = { state: 'error', port: 18890, error: 'Max restart attempts reached' };
    return;
  }

  this.restartAttempts++;
  const delay = this.restartDelay * Math.pow(2, this.restartAttempts - 1);

  // 新增：清理端口
  try {
    await this.cleanupPortLock(this.status.port);
  } catch (error) {
    log.error('Failed to cleanup port before restart:', error);
  }

  setTimeout(() => {
    this.start().catch(error => log.error('Restart failed:', error));
  }, delay);
}
```

确保每次重启前端口是干净的。

### 5. 错误处理与用户提示

| 错误场景 | 检测方式 | 用户提示 |
|----------|----------|----------|
| Gateway 二进制不存在 | `fs.existsSync(binaryPath)` | "未找到 Gateway 程序，请运行 `make build` 或重新安装应用" |
| 端口被系统进程占用且清理失败 | `cleanupPortLock()` 3 轮后仍有残留 | "端口 18890 被系统进程占用，请关闭占用程序后重试" |
| 启动超时（READY 未收到） | 30 秒超时 | "Gateway 启动超时，请查看日志排查问题" |
| 崩溃超过最大重试次数 | `restartAttempts >= maxRestartAttempts` | "Gateway 多次启动失败，已停止自动恢复。点击重试手动启动" |
| 端口被占用但可清理 | `cleanupPortLock()` 成功 | 无需提示，静默恢复 |

错误信息通过 IPC 发送到 renderer，由错误页面展示。

---

## 文件改动清单

| 文件 | 改动内容 |
|------|----------|
| `electron/src/main/gateway.ts` | 新增 `cleanupPortLock()`、改造 `start()` 支持 READY 协议、改造 `attemptRestart()` 先清理端口 |
| `electron/src/main/index.ts` | `initializeApp()` 先创建 loading 窗口，等 gateway ready 后再加载主界面；失败时显示错误 |
| `electron/src/main/ipc.ts` | 新增 IPC 通道：`gateway:retry`（用户点击重试） |
| `cmd/maxclaw-gateway/main.go` 或 `internal/cli/gateway.go` | Gateway server 启动成功后输出 `READY:127.0.0.1:18890` 到 stdout |
| 新增 `electron/static/loading.html` | 启动加载页面 |
| 新增 `electron/static/error.html` | 启动错误页面 |

---

## 测试策略

1. **端口冲突测试**：手动用 `python3 -m http.server 18890` 占用端口，启动 Electron，验证旧进程被清理、Gateway 成功启动
2. **READY 超时测试**：临时注释 Go 侧的 READY 输出，验证 Electron 30 秒后显示超时错误
3. **崩溃重启测试**：启动后手动 `kill -9` Gateway 进程，验证 Electron 自动清理端口并重启
4. **多平台测试**：在 macOS（arm64/x64）、Linux、Windows 上验证 `cleanupPortLock()` 的 PID 查询和清理逻辑

---

## 风险与回退

- **风险**：`cleanupPortLock()` 如果误杀系统关键进程（概率极低，因为只杀监听 18890 的进程），可能影响系统稳定性。
- **缓解**：严格过滤当前 Electron PID 和当前 Gateway 子进程 PID；清理前打日志；只清理监听端口（不是连接端口）。
- **回退**：如果 `cleanupPortLock()` 在某平台失败，fallback 到原有逻辑（直接 spawn，靠超时判断）。
