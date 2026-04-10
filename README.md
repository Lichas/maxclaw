# maxclaw - Local-First AI Agent App in Go (Low Memory, Fully Local, Visual UI, Out-of-the-Box)

> A 24/7 local AI work assistant built with Go. Gateway, sessions, memory, and tool execution stay on your machine.

[![Go Version](https://img.shields.io/badge/Go-1.24%2B-blue)](https://golang.org)
[![License](https://img.shields.io/badge/License-Apache--2.0-green)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)]()

Language: [中文](README.zh.md) | **English**

**maxclaw** is a local AI agent for developers and operators.
Core value proposition: **low memory footprint**, **fully local workflow**, **visual desktop/web UI**, and **fast onboarding**.

- **Go backend, resource-efficient runtime**: single binary gateway + tool orchestration.
- **Fully local workflow**: sessions, memory, logs, and tool runs are stored locally.
- **Desktop UI + Web UI**: visual settings, streaming chat, file preview, and terminal integration.
- **Out-of-the-box setup**: one-command install and default workspace templates.

SEO keywords: `Go AI Agent`, `local AI assistant`, `self-hosted AI agent`, `private AI workflow`, `desktop AI app`, `low-memory AI`.

---

## Product Screenshot

![maxclaw app ui](screenshot/app_ui2.png)

---

## Highlights

- Go-native agent loop and tool system
- Fully local execution path with auditable artifacts
- Desktop UI + Web UI + API on the same port
- `executionMode=auto` for unattended long-running tasks
- `spawn` sub-sessions with independent context/model/source and status callbacks
- Automatic task titles that summarize sessions without overwriting message content
- Monorepo-aware recursive context discovery (`AGENTS.md` / `CLAUDE.md`)
- Multi-channel integrations: Telegram, WhatsApp (Bridge), Discord, WebSocket
- Cron/Once/Every scheduler + daily memory digest

## OpenClaw Concept Mapping

If you are familiar with OpenClaw, maxclaw follows similar local-first principles with a Go-first engineering focus:

- Local-first agent execution and private data boundaries
- Heartbeat context (`memory/heartbeat.md`)
- Memory layering (`MEMORY.md` + `HISTORY.md`)
- Autonomous mode (`executionMode=auto`)
- Sub-agent task split via `spawn`
- Monorepo context discovery for multi-module repositories

## Quick Start

1. Install Go 1.24+ and Node.js 18+
2. Build: `make build`
3. Initialize workspace: `./build/maxclaw onboard`
4. Configure: edit `~/.maxclaw/config.json`
5. Run gateway: `./build/maxclaw-gateway -p 18890`

Built binaries:
- `./build/maxclaw`: full CLI (`onboard`, `skills`, `telegram bind`, `gateway`, ...)
- `./build/maxclaw-gateway`: standalone backend for desktop packaging or headless use

All-in-one local dev startup:

```bash
make build && make restart-daemon && make electron-start
```

Common dev restart commands:

```bash
make dev-gateway
make backend-restart
make dev-electron
make electron-restart
```

## One-Command Install (Linux / macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/Lichas/maxclaw/main/install.sh | bash
```

## Minimal Config

Path: `~/.maxclaw/config.json`

```json
{
  "providers": {
    "anthropic": { "apiKey": "your-anthropic-key" }
  },
  "agents": {
    "defaults": {
      "model": "anthropic/claude-opus-4-5",
      "workspace": "/absolute/path/to/your/workspace",
      "executionMode": "auto"
    }
  }
}
```

OpenAI native models use the official `openai-go` SDK with default API base `https://api.openai.com/v1`.
Anthropic native models use the official `anthropic-sdk-go` SDK with default API base `https://api.anthropic.com`.

## Execution Modes

Set `agents.defaults.executionMode`:

- `safe`: conservative exploration mode
- `ask`: default mode
- `auto`: autonomous continuation (no manual "continue" approval for paused plans)

## Agent Lifecycle

MaxClaw implements a **six-layer adaptive system** that continuously learns from errors and user feedback:

| Layer | Purpose | Key Capabilities |
|-------|---------|------------------|
| **1. Verification** | Error handling | Auto-classify 15+ error types with recovery strategies (Auth, RateLimit, ContextOverflow, etc.) |
| **2. Reflection** | Context management | Smart compression with structured summaries, token usage insights, cost estimation |
| **3. Adaptation** | Dynamic adjustment | Multi-tier model fallback, adaptive context length, retry with exponential backoff |
| **4. Persistence** | Session recovery | Filesystem checkpoints, automatic recovery, resume from failures |
| **5. Evolution** | Pattern learning | Error pattern recognition, strategy effectiveness tracking, self-improvement metrics |
| **6. Feedback** | User learning | Three-tier feedback detection (Rules→Context→LLM), cross-session MEMORY.md persistence |

**Example - Automatic Error Recovery:**
```
Error: 400 Bad Request (context too large)
↓
[Verification] Classifies as ContextOverflowError
↓
[Reflection] Compresses conversation to structured summary
↓
[Adaptation] Reduces context length by 30%, falls back to model with larger window
↓
[Persistence] Saves checkpoint for potential recovery
↓
[Evolution] Logs pattern: "Repeated context overflow in Go refactoring tasks"
↓
[Feedback] If user corrects, learns: "Prefer explicit type annotations in Go"
```

The system is **cost-optimized**: 70% of feedback detection uses regex rules (zero cost), only 15% require LLM semantic analysis (~$0.1/day). See `docs/agent_lifecycle.md` for details.

## Web UI

1. Build: `make webui-install && make webui-build`
2. Start: `./build/maxclaw-gateway -p 18890`
3. Open: `http://localhost:18890`

## More Docs

- Architecture: `ARCHITECTURE.md`
- Operations: `MAINTENANCE.md`
- Browser runbook: `BROWSER_OPS.md`
- Full Chinese docs and all channel/config examples: [README.zh.md](README.zh.md)
