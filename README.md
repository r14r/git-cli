# git-cli

`git-cli` is a standalone Go CLI for Git workflow utilities.

Current version: **0.5.0**

## Command groups

- `security` — secret scanning and commit protection.
- `precommit` — application-aware pre-commit setup and staged-code validation.
- `gitignore` — create and maintain `.gitignore` files from GitHub templates.
- `project` — detect and inspect the current application type.
- `doctor` — diagnose Git hooks, scanners and application tooling.

## Build and install

```bash
just check
just build
sudo just install
```

Default binary installation path: `/usr/local/bin/git-cli`.

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

Django, FastAPI and Python use GitHub's maintained `Python.gitignore`; the Python template already contains Django-specific ignore rules. Laravel adds a curated framework block because the GitHub template repository currently has no dedicated Laravel template.

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
```

Add any template by its catalog name or path:

```bash
git-cli gitignore add --template Python
git-cli gitignore add --template Global/macOS
git-cli gitignore add --template Global/JetBrains
```

Preview before changing `.gitignore`:

```bash
git-cli gitignore show --for django
git-cli gitignore show --template Python
```

Download an official template without modifying the repository:

```bash
git-cli gitignore download --template Python
git-cli gitignore download --template Global/macOS --output macOS.gitignore
```

Inspect or remove git-cli-managed sections:

```bash
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

Running the same `add` command again refreshes that section from the current upstream template while leaving all other `.gitignore` content untouched.

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

`laracel` is accepted as a compatibility alias for `laravel`.

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

The selected preset is stored in `.git-cli-precommit.yaml`.

### Project detection

Current detection rules:

| Preset | Detection |
|---|---|
| `laravel` | `artisan` plus `laravel/framework` in `composer.json` |
| `django` | `manage.py` |
| `fastapi` | `fastapi` in common Python dependency files |
| `python` | common Python project/dependency files |

Inspect detection without changing the repository:

```bash
git-cli project detect
git-cli project detect --json
git-cli project info
git-cli project info --json
```

### Preset checks

| Preset | Checks |
|---|---|
| `python` | `ruff check` on staged `.py` content; fallback to `python -m py_compile` |
| `fastapi` | same staged Python checks |
| `django` | staged Python checks plus `python manage.py check` |
| `laravel` | `php -l` against staged `.php` content |

The validator materializes files from the Git index using `git show :<path>`. Therefore checks operate on the exact content being committed, not on later unstaged worktree edits.

## Hook handling

`git-cli precommit --setup ...` respects Git's configured hook directory:

```bash
git config core.hooksPath .githooks
```

If `core.hooksPath` is unset, `.git/hooks` is used.

The managed hook is intentionally stable:

```bash
#!/usr/bin/env bash
set -e

# managed by git-cli precommit
exec git-cli hook run pre-commit
```

`git-cli hook run pre-commit` runs security checks first and application checks second.

Existing unrelated hooks are never overwritten. `git-cli precommit uninstall` removes the application-specific configuration and downgrades the hook to security-only scanning.

## Doctor

Run:

```bash
git-cli doctor
```

It checks:

- whether the current directory belongs to a Git repository
- effective Git hook path, including `core.hooksPath`
- git-cli pre-commit hook state
- Gitleaks, detect-secrets and optional TruffleHog availability
- `.git-cli.yaml`
- detected application preset
- required runtime such as Python or PHP
- `.git-cli-precommit.yaml`

A non-zero exit code indicates a required dependency or configuration problem.

## Security configuration

Example `.git-cli.yaml`:

```yaml
fail_on: high
scanners:
  gitleaks:
    enabled: true
    required: true
  detect-secrets:
    enabled: true
    required: true
  trufflehog:
    enabled: true
    required: false
detect_secrets_baseline: .secrets.baseline
# gitleaks_config: .gitleaks.toml
```

Optional detect-secrets baseline:

```bash
detect-secrets scan > .secrets.baseline
```

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

- `0`: command/check passed
- `1`: finding, application validation failure or doctor problem
- `2`: configuration/runtime/usage error

Git hooks can be bypassed with `git commit --no-verify`; CI or pre-push enforcement remains appropriate when stronger policy enforcement is required.
