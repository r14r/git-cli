# git-cli

`git-cli` is a standalone Go CLI for Git workflow utilities. It currently provides two command groups:

- `security` — secret scanning and commit protection.
- `precommit` — application-aware pre-commit setup and staged-code validation.

Current version: **0.3.0**

## Security scanners

- **Gitleaks** — required/default; staged, repository and history scans.
- **detect-secrets** — required/default; staged and repository scans.
- **TruffleHog** — optional; deep/history scans.

Install scanner dependencies on macOS:

```bash
brew install gitleaks detect-secrets trufflehog
```

## Build and install

```bash
just check
just build
sudo just install
```

Default binary installation path: `/usr/local/bin/git-cli`.

## Application-aware pre-commit setup

Explicitly select a preset:

```bash
git-cli precommit --setup --for python
git-cli precommit --setup --for fastapi
git-cli precommit --setup --for django
git-cli precommit --setup --for laravel
```

`laracel` is accepted as an alias for `laravel`.

Or detect the application in the current Git repository:

```bash
git-cli precommit --setup --scan
```

Detection rules currently recognize:

- Laravel: `artisan` plus `laravel/framework` in `composer.json`
- Django: `manage.py`
- FastAPI: `fastapi` in common Python dependency files
- Python: common Python project/dependency files

The setup creates `.git-cli-precommit.yaml` and installs a managed `.git/hooks/pre-commit` hook.

Example configuration:

```yaml
preset: fastapi
```

The generated hook executes:

```bash
#!/usr/bin/env bash
set -e

# managed by git-cli precommit
git-cli security "check-staged"
exec git-cli precommit run
```

This means every commit performs secret scanning first and application checks second.

### Preset checks

| Preset | Checks |
|---|---|
| `python` | `ruff check` on staged `.py` files; falls back to `python -m py_compile` |
| `fastapi` | same staged Python checks |
| `django` | staged Python checks plus `python manage.py check` |
| `laravel` | `php -l` for staged `.php` files |

Additional precommit commands:

```bash
git-cli precommit run
git-cli precommit status
git-cli precommit list
```

Existing unrelated pre-commit hooks are not overwritten. A previous git-cli security-only hook can be upgraded to the combined application-aware hook.

## Security commands

```bash
git-cli security install

git-cli security check-staged
git-cli security check
git-cli security check --deep
git-cli security check-history

git-cli security scanner list
git-cli security scanner status
```

Additional management commands:

```bash
git-cli security status
git-cli security uninstall
git-cli version
git-cli help
git-cli security help
git-cli precommit help
```

Use `--json` with security scan commands for machine-readable output.

## Security configuration

`.git-cli.yaml`:

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

This repository includes `update-cli.yaml` for the Go build/install workflow. The update source itself is configured in `.updater-cli/config.json` as:

```text
https://github.com/r14r/git-cli.git
```

Equivalent initialization:

```bash
update-cli init git-cli --from repository --repository https://github.com/r14r/git-cli.git
```

## Exit codes

- `0`: command/check passed
- `1`: secret finding or application precommit check failed
- `2`: configuration/scanner/runtime/usage error

Git hooks can be bypassed with `git commit --no-verify`; CI or pre-push scanning should be used as a second enforcement layer where stronger enforcement is required.
