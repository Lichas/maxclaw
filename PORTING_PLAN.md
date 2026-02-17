# nanobot-go Feature Porting Plan

This plan tracks parity work against the Python `nanobot` milestones.

## Scope

- Port Python features to Go incrementally.
- Every delivered item must include tests and verification.

## Milestones

- [x] 2026-02-14: MCP support
  - [x] MCP config support (`tools.mcpServers`)
  - [x] Claude Desktop / Cursor style top-level `mcpServers` compatibility
  - [x] MCP tool discovery + registration as native tools
  - [x] Agent lifecycle integration and cleanup
  - [x] Unit/integration tests and full `go test ./...` pass

- [x] 2026-02-13: v0.1.3.post7
  - [x] Security hardening parity
  - [x] WhatsApp bridge shared-secret auth (`bridgeToken` / `BRIDGE_TOKEN`)
  - [x] Telegram `allowFrom` enforcement (ID/username allow-list)
  - [x] Bug-fix parity and regression tests
  - [x] CLI session default/key compatibility (`cli:direct`, explicit session key honored)
  - [x] Slash commands parity baseline (`/new`, `/help`)
  - [x] Iteration limit fallback message parity
  - [x] `maxTokens` / `temperature` wired into provider requests (`max_tokens` clamped to >=1)
  - [x] Session retention window raised to 500

- [x] 2026-02-12: Memory system refactor parity
  - [x] Two-layer memory architecture (`MEMORY.md` + `HISTORY.md`)
  - [x] Session consolidation pipeline (`/new` archive-all + threshold-based auto consolidate)
  - [x] Append-only session semantics with windowed history loading (`GetHistory(500)`)
  - [x] Reliability-focused tests

- [x] 2026-02-11: CLI experience enhancements + MiniMax provider
  - [x] CLI UX parity updates
  - [x] CLI interactive baseline improvements (`exit/quit`, EOF graceful exit, robust input trim)
  - [x] Advanced CLI parity (prompt history/editing)
  - [x] CLI compatibility switches (`--markdown/--no-markdown`, `--logs/--no-logs`)
  - [x] MiniMax provider support + tests

- [x] 2026-02-10: v0.1.3.post6 parity
  - [x] Feature improvements and fixes from Python milestone

- [x] 2026-02-09: Multi-platform chat support
  - [x] Slack (Socket Mode)
  - [x] Email (IMAP/SMTP)
  - [x] QQ (private chat)

- [x] 2026-02-08: Provider architecture refactor
  - [x] Add-new-provider in two steps parity (`ProviderSpec + ProvidersConfig` equivalent)
  - [x] Spec-driven API key/base routing (registry order + default API base)

- [x] 2026-02-07: v0.1.3.post5 parity
  - [x] Qwen provider support (DashScope)
  - [x] Key improvements parity

- [x] 2026-02-06: Moonshot/Kimi + Discord + security parity check
  - [x] Verify full parity and tests

- [x] 2026-02-05: Feishu + DeepSeek + scheduler enhancements parity
  - [x] Feishu channel
  - [x] DeepSeek parity verification
  - [x] Enhanced scheduler parity

- [x] 2026-02-04: v0.1.3.post4 parity
  - [x] Multi-provider support verification
  - [x] Docker support parity

- [x] 2026-02-03: vLLM local model parity
  - [x] vLLM support verification
  - [x] Natural-language scheduling improvements parity
