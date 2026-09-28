# PRD: Portable `cmg`

## Problem

`cmg` works on one machine because its alias, compiled program, and AI configuration live there. Reusing it elsewhere requires reconstructing that setup, and machine-specific AI defaults are not portable.

## Goal

Provide an installable `cmg` command that preserves the existing commit-generation workflow. A new machine should need only the executable on `PATH`, Git, and access to a user-configured OpenAI-compatible AI endpoint.

## User stories

1. As a developer, I can install `cmg` on another machine without copying a local alias or private configuration.
2. As a developer, I can configure the AI endpoint and model with environment variables and override them with flags.
3. As a developer, I can keep my API key out of the program and repository.
4. As a developer, I can stage changes and run `cmg` to generate a message and commit them by default.
5. As a developer, I can run `cmg --commit=false` to inspect a proposed message without committing.
6. As a developer, I get a branch ticket in the message when one exists; `main` and other no-ticket branches still work.
7. As a developer, I get a clear error without a commit when prerequisites, AI generation, or Git commit fail.
8. As a developer, I can opt into the existing review and hook-output workflows.

## Scope and behavior

- Reuse only code needed for the `cmg` workflow: staged diff, current branch, optional ticket extraction, AI request/response, message formatting, optional review, hook output, and commit.
- Use a directly runnable `cmg` executable instead of requiring a shell alias. Publish binaries for macOS arm64/amd64, Linux arm64/amd64, and Windows amd64 on GitHub Releases; provide source/build instructions.
- Require an explicit AI endpoint and model. Support environment-based configuration, with command-line overrides. Never embed a personal endpoint or credential.
- Commit staged changes unless `--commit=false` or `--dry-run` is supplied. Never stage files automatically. Run normal Git hooks by default; `--no-verify` explicitly opts out.
- Ticket extraction is optional, not branch validation. For example, `feature/ABC-123-login` includes `ABC-123`; `main` generates a message without a ticket.
- Send only the bounded staged diff to the configured AI endpoint. Make this data transfer explicit in installation documentation.

## Acceptance criteria

- A fresh machine can build or install `cmg`, configure its endpoint/model, and run it in any Git repository with staged changes.
- Ticket and no-ticket branches both generate valid messages; only the former gets a ticket prefix.
- Preview mode creates no commit; default mode creates exactly one commit on success.
- Missing configuration, no staged changes, invalid AI output, and failed Git operations produce nonzero exits without a new commit.
- No API key or machine-specific endpoint is committed or compiled as a default.

## Out of scope

- Hosting or provisioning an AI service; offline generation; automatic installation of Git/Go; automatically staging files.
- Enforcing ticket naming, changing the conventional message format, or creating a cross-platform installer.
- Publishing binaries, synchronizing credentials, or redesigning the review workflow.

## Implementation decisions

- Publish five prebuilt targets: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64.
- Invoke standard `git commit` by default so repository hooks run. Add `cmg --no-verify` as an explicit opt-out. A failed hook is a failed command, not a successful commit.
- Retain `--commit=false`; add `--dry-run` as an equivalent preview mode. Defer `--amend` and `--yes`.
- Keep credentials in environment variables, not command-line flags or help defaults.
