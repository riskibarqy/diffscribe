# diffscribe

`cmg` generates a commit message from the staged Git diff, then commits by default. It sends that diff to the OpenAI-compatible endpoint **you configure**. Review staged changes for secrets before running it.

## Install

Install Git first. Once a GitHub release is published, use its installer (macOS/Linux selects your OS and CPU automatically):

```sh
curl -fL https://github.com/riskibarqy/diffscribe/releases/latest/download/install.sh -o install.sh
sh install.sh
```

Windows 64-bit (PowerShell):

```powershell
Invoke-WebRequest 'https://github.com/riskibarqy/diffscribe/releases/latest/download/install.ps1' -OutFile install.ps1
.\install.ps1
```

Inspect the downloaded script before running it; installers download an executable from GitHub Releases. On macOS/Linux, follow the printed `PATH` instruction if needed. On Windows, open a new terminal after installation. Run `cmg --help` to verify.

Alternatively, [download a binary manually](https://github.com/riskibarqy/diffscribe/releases), rename it `cmg` (`cmg.exe` on Windows), and put it on `PATH`. To build from source with Go 1.21+, clone this repository and run `go build -o cmg .`.

## Configure

```sh
export COMMITGEN_ENDPOINT='http://localhost:11434/v1'
export COMMITGEN_MODEL='<model-name>'
# export COMMITGEN_API_KEY='<key>'  # only if your endpoint needs one
```

`--endpoint` and `--model` override the corresponding environment variables. There are no built-in endpoint, model, or API-key defaults. The endpoint must support OpenAI-compatible `/chat/completions` with non-streaming responses. The staged diff (up to 32,000 bytes by default) is sent to that endpoint; use a trusted service.

## Use

```sh
git add -p
cmg                   # generate, then commit with normal Git hooks
cmg --dry-run         # show message, no commit
cmg --commit=false    # same preview behavior as the original command
cmg --no-verify       # explicitly bypass pre-commit/commit-msg hooks
cmg --review          # optional AI review before generation
cmg --hook <path>     # write message to a file instead of committing
```

A ticket in the final branch segment, such as `feature/ABC-123-login`, prefixes the headline with `ABC-123`. Branches such as `main` work without a ticket. Unstaged changes are never added automatically. Failed hooks and Git errors prevent success and leave the generated message uncommitted; fix the issue and rerun `cmg`.

`--max-bytes` changes the staged diff limit; `--timeout` changes the total command timeout (default `30s`). Missing configuration, no staged changes, AI errors, or invalid responses fail without committing. Keep API keys in your local environment, not command-line arguments or repository files.
