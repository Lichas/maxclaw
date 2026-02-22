# AGENTS.md

This file provides guidance to coding agents (Codex, Claude, and similar) when working in this repository.

## Required Workflow

- Complete the user request end-to-end: implement, verify, then report.
- Run relevant validation (tests/build/smoke) yourself; do not ask the user to verify in your place.
- After successfully completing a request that changes repository files, run `make build` and then create a `git commit` for that request.
- If a user request results in a repository change (feature, bug fix, behavior change, config change, or docs change), you must append an entry to `CHANGELOG.md` under `## [Unreleased]`.
- Changelog entries should be concise and include:
  - what changed
  - where (key files)
  - how it was verified (test/build command)
- If no repository files were changed, explicitly state that no changelog update is needed.

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
