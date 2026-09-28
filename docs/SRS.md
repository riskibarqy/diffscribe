# SRS: Portable `cmg`

## 1. Interfaces and installation

- `cmg` SHALL be an executable available on `PATH`, independent of a shell alias. GitHub Releases SHALL contain builds for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, and windows/amd64; source build instructions SHALL also be provided.
- `cmg` SHALL operate on the Git repository containing the current working directory. It SHALL NOT depend on files or services unique to the original developer's machine, apart from the explicitly configured AI endpoint.
- Supported configuration SHALL include `COMMITGEN_ENDPOINT`, `COMMITGEN_MODEL`, and optional `COMMITGEN_API_KEY`. `--endpoint` and `--model` SHALL override their corresponding environment values. An API key SHALL NOT be required for endpoints that need no authentication.
- Missing endpoint or model SHALL cause a clear nonzero error before any network request or commit. The program SHALL NOT fall back to a hard-coded personal endpoint, model, or key.
- Retain `--commit=false` (preview), `--review` (opt-in), `--hook <path>` (write message instead of committing), `--max-bytes`, and `--timeout`. Add `--dry-run` (preview) and `--no-verify` (explicit Git hook bypass). Defaults: commit enabled, review disabled, hooks enabled, diff cap 32,000 bytes, total timeout 30 seconds.

## 2. Functional requirements

- FR-1: Read the staged diff only. If it is empty, exit nonzero without contacting the AI or committing. Do not stage or modify working-tree files.
- FR-2: Read the current branch. Extract a ticket matching letters, `-`, digits from the final branch segment when present. A missing ticket SHALL NOT fail generation or commit.
- FR-3: Limit the diff sent to the AI to `--max-bytes`. Generate a commit message from that diff and branch context using the configured OpenAI-compatible endpoint/model.
- FR-4: Accept only a usable generated message. A missing/invalid response or AI request failure SHALL exit nonzero without committing. Do not silently replace failures with an unrelated generic message.
- FR-5: Prefix the generated headline with the extracted ticket when present. On `main` or another no-ticket branch, omit the prefix; keep the generated message.
- FR-6: With `--commit=false`, print the generated message and exit successfully without committing.
- FR-7: By default, use normal `git commit` with the generated headline and optional body, allowing repository Git hooks to run. `--no-verify` SHALL explicitly pass that flag to Git. A failed hook or Git commit SHALL exit nonzero and SHALL NOT report success. Unstaged changes SHALL remain unstaged.
- FR-8: With `--review`, request and print review findings before the message. A review-only failure SHALL warn and allow message generation to proceed, matching current behavior.
- FR-9: With `--hook <path>`, write the generated message to that path and do not commit, regardless of the default commit setting. A write failure SHALL exit nonzero.

## 3. Security and failure behavior

- Credentials SHALL come from runtime configuration, not source, binary defaults, documentation examples, or Git history. They SHALL NOT be printed by help, normal output, or errors.
- Documentation SHALL state that staged diff content is transmitted to the configured endpoint. No request SHALL be sent without an explicit endpoint.
- Flag and environment inputs affecting request size or timeout SHALL reject nonpositive values. Network, parsing, and Git failures SHALL yield nonzero exit status and a useful error without claiming a commit succeeded.
- Invalid AI output SHALL not be passed unvalidated to `git commit`. Do not log the full staged diff or authorization header.

## 4. Verification scenarios

| Setup | Command | Required result |
| --- | --- | --- |
| Staged change; branch `feature/ABC-123-login`; AI configured | `cmg` | One commit; headline includes `ABC-123` |
| Staged change; branch `main`; AI configured | `cmg` | One commit; headline has no ticket prefix |
| Staged change; AI configured | `cmg --commit=false` or `cmg --dry-run` | Message printed; no commit |
| Failing pre-commit hook | `cmg` | Git error shown; no commit |
| Failing pre-commit hook | `cmg --no-verify` | Commit succeeds; hook explicitly bypassed |
| No staged change | `cmg` | Nonzero error; no AI call; no commit |
| Endpoint or model missing | `cmg` | Nonzero configuration error; no AI call; no commit |
| AI unreachable or response invalid | `cmg` | Nonzero error; no commit |
| Review request fails; generation succeeds | `cmg --review --commit=false` | Warning and generated message; no commit |
| Hook path provided | `cmg --hook <path>` | Message written; no commit |
| Git commit fails | `cmg` | Nonzero error; no successful commit reported |

## 5. Decisions

- Use `COMMITGEN_API_KEY` for credentials; omit `--api-key` to avoid shell-history/process-list exposure.
- Do not run hooks manually or bypass them by default. Normal Git behavior applies; `--no-verify` is explicit.
- `--dry-run` and `--commit=false` both generate without committing. `--amend` and `--yes` are out of scope for v1.
