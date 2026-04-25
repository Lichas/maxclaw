# AGENTS.md

This file provides guidance to coding agents (Codex, Claude, and similar) when working in this repository.

## Required Workflow

- Complete the user request end-to-end: implement, verify, then report.
- Run relevant validation (tests/build/smoke) yourself; do not ask the user to verify in your place.
- If a request changes repository files, run the relevant E2E/regression test script(s) before finishing. If any E2E test fails, investigate the failure and either fix it or clearly report the blocker; do not ignore the failure.
- After successfully completing a request that changes repository files, run `make build` and then create a `git commit` for that request.
- If a user request results in a repository change (feature, bug fix, behavior change, config change, or docs change), you must append an entry to `CHANGELOG.md` under `## [Unreleased]`.
- Changelog entries should be concise and include:
  - what changed
  - where (key files)
  - how it was verified (test/build command)
- If no repository files were changed, explicitly state that no changelog update is needed.
- When changing stateful UI or session-management logic, think through the full interaction loop before editing:
  - identify every effect, auto-redirect, default selection, and persistence path that can overwrite the new state
  - verify the intended empty/initial state, transition state, and restored state separately
  - avoid "local fix, global regression" changes where one handler is updated but linked sync logic still forces old behavior back
- Core flows must gain regression coverage when they change:
  - for session creation, chat routing, interrupt/resume, spawn/callback, and other user-visible control flow, add or extend an automated E2E/regression test in `e2e_test/`
  - do not treat manual clicking as sufficient verification for these paths
  - if full UI automation is not available yet, still add the closest end-to-end regression that exercises the real persistence/API boundary and document the remaining gap

## Parallel Sessions (Git Worktree)

- For concurrent sessions/tasks, use one dedicated Git worktree and one dedicated branch per task.
- Branch names must use the `codex/` prefix.
- Before creating a task worktree, sync main with fast-forward only.
- Suggested setup:
  - `git fetch origin`
  - `git switch main`
  - `git pull --ff-only`
  - `git worktree add ../maxclaw-wt-<task> -b codex/<task> main`
- Do all edits, validation, and commit in that task worktree.
- Merge back to `main` only after verification passes (relevant tests + `make build`).
- After merge, clean up the worktree and merged task branch.

## UI Icon Search Rule

- For future UI icon updates, use this source first:
  - `https://igoutu.cn/icons/set/{keyworkd}--os-macos`
- Replace `{keyworkd}` with the icon keyword (for example: `hammer`, `search`, `settings`) and prefer macOS-style icons from that result page.
