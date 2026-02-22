# MAINTENANCE.md

maxclaw 维护与排障手册（供 code agent / 运维使用）。

## 1. 目标
- 快速判断“服务是否活着”。
- 快速定位“为什么频道无回复（尤其 Telegram/WhatsApp）”。
- 用统一命令恢复到可用状态。

## 2. 标准运行模型
- Bridge: `3001`（Node）
- Gateway: `18890`（Go）
- 日志目录: `~/.maxclaw/logs`
- PID 文件: `<repo>/.pids/bridge.pid` / `<repo>/.pids/gateway.pid`

## 3. 30 秒健康检查
在仓库根目录执行：

```bash
lsof -nP -iTCP:3001 -sTCP:LISTEN
lsof -nP -iTCP:18890 -sTCP:LISTEN
curl -sS http://127.0.0.1:18890/api/status
```

判定：
- 两个端口都监听 + `/api/status` 可返回 JSON = 基础存活正常。
- 任一失败 = 进入第 4 节。

## 4. 启动/重启标准动作
```bash
make restart-daemon
```

如果仍失败：
```bash
make down-daemon
make up-daemon
```

说明：
- 现版本 `make up`/`make up-daemon` 会自动清理占用 `3001/18890` 的旧进程。
- `start_daemon.sh` 已包含启动后健康检查，防止“假启动”。

## 5. Telegram 无回复排障

### 5.1 快速判断是否“消息积压在 Telegram 服务器”
```bash
TOKEN=$(jq -r '.channels.telegram.token' ~/.maxclaw/config.json)
curl -sS "https://api.telegram.org/bot${TOKEN}/getWebhookInfo"
```

关键字段：
- `pending_update_count` 持续增长：说明本地轮询没有消费。

### 5.2 检查本地是否收到与发送
```bash
tail -n 200 /Users/lua/.maxclaw/logs/channels.log
```

关键日志：
- 入站：`telegram inbound ...`
- 出站：`telegram send ...`

### 5.3 代理相关（高频根因）
如果网络必须代理，请同时满足：
- 配置文件有 Telegram 代理：
```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "token": "...",
      "proxy": "http://127.0.0.1:7897"
    }
  }
}
```
- 环境变量（脚本已支持大小写）：
```bash
export http_proxy=http://127.0.0.1:7897
export https_proxy=http://127.0.0.1:7897
export all_proxy=http://127.0.0.1:7897
export NO_PROXY=localhost,127.0.0.1,0.0.0.0,::1
```
- 重启：`make restart-daemon`

## 6. WhatsApp 无回复排障
```bash
tail -n 200 /Users/lua/.maxclaw/logs/bridge.log
tail -n 200 /Users/lua/.maxclaw/logs/channels.log
```

检查：
- bridge 是否打印 `Bridge server listening`。
- `channels.log` 是否有 `whatsapp inbound/send`。
- `api/status` 中 `whatsapp.connected` 是否为 `true`。

## 7. 常见误区
- `0.0.0.0:18890` 在代理环境可能返回 502；访问请用 `localhost:18890`。
- PID 文件存在不等于进程还活着；必须以端口监听和 `/api/status` 为准。
- `ps aux | grep nano` 里的 `git-remote-https` 不是 maxclaw 运行进程。

## 8. 事故记录要求（必须）
每次修复生产可见问题后，必须在 `BUGFIX.md` 追加：
- 现象
- 根因
- 修复
- 验证命令与结果

## 9. 给 code agent 的执行顺序
1. 先看端口和 `/api/status`。
2. 再看 `channels.log` 是否有 inbound/send。
3. 再看 `gateway.log` / `bridge.log` 退出或错误。
4. 最后检查代理与配置一致性。
5. 修复后必须 `make restart-daemon` 并复测。
