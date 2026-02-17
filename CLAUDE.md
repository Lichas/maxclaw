# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**nanobot-go** is an ultra-lightweight personal AI assistant framework written in Go (~3,500 lines of core code). It provides core agent functionality including tool use, multi-channel chat integrations, and scheduled tasks.

- **Language**: Go 1.21+
- **Module**: `github.com/Lichas/nanobot-go`
- **Entry point**: `cmd/nanobot/main.go`
- **Binary name**: `nanobot-go`

## Operating Rules (Must Follow)

- **Do not ask the user to verify**. You are responsible for running tests, validating behavior, and reporting results.
- **Always verify changes**: run relevant tests or smoke checks, and report what passed/failed.
- **After successfully completing a request with repository changes**, run `make build` and then create a `git commit` for that request.
- **If you fix a bug**, append a concise entry to `BUGFIX.md` (what broke, root cause, fix, verification).
- **If a request changes repository files**, append a concise entry to `CHANGELOG.md` under `## [Unreleased]` (what changed, key files, verification command).
- **When adding features**, update the docs (README / architecture) and validate the path end-to-end.
- **Assume ownership of outcomes**: if something could be flaky, add logging/guards and document the caveats.
- **For runtime incidents**, follow `MAINTENANCE.md` first (process/port/proxy/log order) before ad-hoc debugging.

## Development Commands

### Build & Run
```bash
make build          # Build to build/nanobot-go
make install        # Install to GOPATH/bin
make run            # Build and run
make dev            # Hot reload (requires air)
```

### Testing
```bash
make test           # Run all tests (go test -v ./...)
make coverage       # Generate coverage report
# OR
go test ./...       # Direct go test
go test -v ./...    # Verbose output
go test -cover ./... # With coverage
```

### Code Quality
```bash
make fmt            # Format code (go fmt ./...)
make vet            # Run go vet
make lint           # Run fmt and vet
make mod            # Tidy and verify modules
```

### E2E Testing
```bash
cd e2e_test
./run.sh            # Run all E2E tests
./tools_test.sh     # Tool-specific tests
```

## Architecture

### Core Components

**Agent Loop** (`internal/agent/loop.go`)
- Central processing engine that receives messages, builds context with history, calls LLM, executes tools
- Key struct: `AgentLoop` with message bus, provider, workspace, tools
- Max 20 iterations for tool calls (configurable via `MaxToolIterations`)
- Integrates: `ContextBuilder`, `SessionManager`, `ToolRegistry`

**Tool System** (`pkg/tools/`)
- Interface: `Tool` in `base.go` - defines `Name()`, `Description()`, `Parameters()`, `Execute()`
- Registry: `ToolRegistry` in `registry.go` - manages tool registration and execution
- Built-in tools:
  - `filesystem.go`: ReadFileTool, WriteFileTool, EditFileTool, ListDirTool
  - `shell.go`: ExecTool (shell command execution with timeout)
  - `web.go`: WebSearchTool (Brave API), WebFetchTool
  - `message.go`: MessageTool (send messages back to channels)
  - `spawn.go`: SpawnTool (background subagents)
  - `cron.go`: CronTool (manage scheduled jobs)

**Message Bus** (`internal/bus/queue.go`)
- Routes messages between components
- Handles `InboundMessage` and `OutboundMessage` events
- Channel-based implementation with 100-message buffer

**Configuration** (`internal/config/`)
- Schema: `schema.go` - structs with mapstructure tags
- Loader: `loader.go` - JSON config at `~/.nanobot/config.json`
- Key config sections: `providers`, `agents`, `channels`, `tools`, `gateway`

**LLM Provider** (`internal/providers/openai.go`)
- OpenAI-compatible API (works with OpenRouter, DeepSeek, Anthropic, etc.)
- Supports tool calling via function definitions

**Cron Service** (`internal/cron/`)
- Three schedule types: Every (ms), Cron (expression), Once (datetime)
- Persistent JSON storage at `~/.nanobot/workspace/.cron/jobs.json`
- Thread-safe job management

**Channel System** (`internal/channels/`)
- Integrations: Telegram (Bot API polling), Discord (HTTP API), WhatsApp (bridge URL)
- Allowlist for access control (`allowFrom`)

### Key Design Patterns

1. **Tool Registration**: Tools self-describe via JSON Schema parameters. The `ToolRegistry` validates and executes them.

2. **Workspace Restriction**: When `tools.restrictToWorkspace` is true in config, file tools and shell execution are sandboxed to the workspace directory via `tools.SetAllowedDir()`.

3. **Session Management**: `SessionManager` persists conversation history per `session_key` (format: `channel:chat_id`) as JSON files in the workspace.

4. **Provider Selection**: Config uses keyword matching to select the right API key based on model name (e.g., "claude" -> Anthropic, "deepseek" -> DeepSeek).

## Configuration File

Location: `~/.nanobot/config.json`

Key structure:
```json
{
  "providers": {
    "openrouter": { "apiKey": "..." },
    "anthropic": { "apiKey": "..." }
  },
  "agents": {
    "defaults": {
      "model": "anthropic/claude-opus-4-5",
      "workspace": "~/.nanobot/workspace",
      "maxToolIterations": 20
    }
  },
  "channels": {
    "telegram": { "enabled": true, "token": "...", "allowFrom": [] },
    "discord": { "enabled": false, "token": "..." }
  },
  "tools": {
    "web": { "search": { "apiKey": "..." } },
    "exec": { "timeout": 60 },
    "restrictToWorkspace": false
  },
  "gateway": {
    "host": "0.0.0.0",
    "port": 18890
  }
}
```

## CLI Commands

```bash
nanobot-go onboard                    # Initialize config and workspace
nanobot-go agent [-m "message"]       # Run agent (interactive or single message)
nanobot-go gateway                    # Start gateway server
nanobot-go status                     # Show configuration status
nanobot-go cron add [flags]           # Add scheduled job
nanobot-go cron list                  # List all jobs
nanobot-go cron remove <job-id>       # Remove job
nanobot-go cron run                   # Start cron scheduler daemon
```
