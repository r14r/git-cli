# git-cli

`git-cli` is a standalone Go CLI for Git workflow utilities.

Current version: **0.6.0**

## Command groups

- `security` — secret scanning and commit protection.
- `precommit` — application-aware pre-commit setup and staged-code validation.
- `gitignore` — create, maintain and diagnose `.gitignore` rules.
- `project` — detect and inspect the current application type.
- `repo` — repository health diagnostics.
- `clean` — safe Git cleanup previews and explicit ignored-file cleanup.
- `large-files` — detect oversized tracked files.
- `branch` — inspect merged and stale local branches.
- `doctor` — diagnose Git hooks, scanners and application tooling.

## Build and install

```bash
just check
just build
sudo just install
```

Default binary installation path: `/usr/local/bin/git-cli`.

## Repository intelligence

### Repository health

```bash
git-cli repo health
```

The health report checks the current branch, working-tree state, upstream configuration, ahead/behind counts, `.gitignore`, and tracked files that now match ignore rules. Exit code `1` means actionable repository issues were found; exit code `2` means the command itself could not run.

### Explain ignore rules

```bash
git-cli gitignore explain .env
git-cli gitignore explain .env dist/output.json
```

This wraps `git check-ignore -v --no-index`, so the output shows the matching ignore source, line and pattern. Tracked files can also be inspected.

Find files which are already tracked even though current ignore rules match them:

```bash
git-cli gitignore tracked
```

The command is read-only. It suggests `git rm --cached <path>` but never modifies the index automatically.

### Safe cleanup

Preview untracked files that Git could clean:

```bash
git-cli clean preview
```

Preview only ignored files:

```bash
git-cli clean ignored --preview
```

Explicitly remove ignored files/directories:

```bash
git-cli clean ignored --apply
```

The destructive path requires the explicit `--apply` option. It maps to Git's ignored-only clean mode (`git clean -fdX`).

### Large tracked files

```bash
git-cli large-files scan
git-cli large-files scan --threshold-mb 50
```

The default threshold is 25 MB. The scanner reads Git index blob sizes and reports large tracked files without modifying the repository.

### Branch inspection

List local branches already merged into the current `HEAD`, excluding the current branch and conventional `main`/`master` branches:

```bash
git-cli branch merged
```

List local branches ordered by commit date:

```bash
git-cli branch stale
```

These commands are read-only. Branch deletion is intentionally not automatic in v0.6.0.

## Gitignore management

`git-cli` can create or extend `.gitignore` without overwriting hand-written rules. Managed template content is stored in marked sections and can be refreshed or removed independently.

### Add a project preset

```bash
git-cli gitignore add --for python
git-cli gitignore add --for fastapi
git-cli gitignore add --for django
git-cli gitignore add --for laravel
git-cli gitignore add --for go
git-cli gitignore add --for node
```

Automatically detect the current project and apply the matching preset:

```bash
git-cli gitignore add --scan
```

### Browse and use GitHub templates

The template catalog is loaded from the public `github/gitignore` repository.

```bash
git-cli gitignore list
git-cli gitignore list --filter python
git-cli gitignore list --filter jetbrains

git-cli gitignore add --template Python
git-cli gitignore add --template Global/macOS
git-cli gitignore add --template Global/JetBrains

git-cli gitignore show --for django
git-cli gitignore show --template Python

git-cli gitignore download --template Python
git-cli gitignore download --template Global/macOS --output macOS.gitignore

git-cli gitignore status
git-cli gitignore remove --for django
git-cli gitignore presets
```

A managed section looks like:

```text
# >>> git-cli gitignore: django
...
# <<< git-cli gitignore: django
```

Running the same `add` command again refreshes that section while leaving other `.gitignore` content untouched.

## Security

The default scanners are:

- **Gitleaks** — staged, repository and history scans.
- **detect-secrets** — staged and repository scans.
- **TruffleHog** — optional deep/history scanner.

On macOS:

```bash
brew install gitleaks detect-secrets trufflehog
```

Commands:

```bash
git-cli security install
git-cli security check-staged
git-cli security check
git-cli security check --deep
git-cli security check-history
git-cli security scanner list
git-cli security scanner status
git-cli security status
git-cli security uninstall
```

Security configuration is stored in `.git-cli.yaml`.

## Application-aware pre-commit

Explicit setup:

```bash
git-cli precommit --setup --for python
git-cli precommit --setup --for fastapi
git-cli precommit --setup --for django
git-cli precommit --setup --for laravel
```

Automatic detection:

```bash
git-cli precommit --setup --scan
```

Management:

```bash
git-cli precommit run
git-cli precommit status
git-cli precommit list
git-cli precommit uninstall
```

The selected preset is stored in `.git-cli-precommit.yaml`. Staged Python/PHP checks operate on materialized Git-index content rather than later unstaged worktree edits.

## Project detection

```bash
git-cli project detect
git-cli project detect --json
git-cli project info
git-cli project info --json
```

Current pre-commit project types include Python, FastAPI, Django and Laravel.

## Hook handling

`git-cli precommit --setup ...` respects `core.hooksPath`. If unset, `.git/hooks` is used. The managed pre-commit hook dispatches to security scanning first and application validation second.

## Doctor

```bash
git-cli doctor
```

`doctor` diagnoses environment/configuration correctness: repository access, hooks, secret scanners, security config, detected application preset and required runtimes.

`repo health` is complementary: it reports the quality/state of the repository itself.

## Update with update-cli

The repository contains `update-cli.yaml`, and `.updater-cli/config.json` uses:

```text
https://github.com/r14r/git-cli.git
```

Equivalent initialization:

```bash
update-cli init git-cli --from repository --repository https://github.com/r14r/git-cli.git
```

## Exit codes

- `0`: command/check passed or no finding.
- `1`: finding or actionable repository issue.
- `2`: configuration, runtime or usage error.

Git hooks can be bypassed with `git commit --no-verify`; CI or pre-push enforcement remains appropriate when stronger policy enforcement is required.
