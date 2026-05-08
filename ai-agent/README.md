# agent-team-monitor Agent README

## Project Reality

- Project: agent-team-monitor
- Repository path: /home/vkleo/.openclaw/workspace/development/agent-team-monitor
- Current branch at migration: master
- Compatible old agent alias: (none recorded)
- This file is Hermes-readable project context. Keep stable facts here; put temporary task state in `ai-agent/PROGRESS.md`.

## Tech Stack Signals

- Go module
- Repository README.md present

## Commands Discovered

These are discovered from repository marker files during OpenClaw → Hermes migration. Verify before use if the project has unusual setup.

- `go test ./...`
- `go build ./...`

## Directory Boundaries

- Main source paths: inspect repository root and README before editing.
- Allowed edit paths: project source, tests, docs, and `ai-agent/` when relevant to the task.
- Do-not-edit paths: secrets, auth profiles, generated vendor/cache directories, and unrelated legacy OpenClaw bootstrap files unless explicitly migrating them.
- Generated files: do not commit build/cache artifacts unless the repo explicitly tracks them.

## Workflow Notes

1. Start by reading `ai-agent/README.md` and `ai-agent/PROGRESS.md`.
2. Check `git status --short` before edits.
3. Preserve existing uncommitted changes; do not mix unrelated historical changes into a new commit.
4. Make the smallest necessary change and run the smallest relevant verification.
5. Review diff/status before reporting.
6. Do not read, print, or send secrets/tokens/auth profiles.
7. Do not run destructive or privileged commands without explicit scope confirmation.

## Migration Notes

- Created by Hermes during OpenClaw → Hermes migration on 2026-05-08.
- Existing OpenClaw files such as `AGENTS.md`, `.openclaw/`, `BOOTSTRAP.md`, `HEARTBEAT.md`, or `IDENTITY.md` may still be present. Treat them as legacy inputs, not the new canonical project-agent home.
