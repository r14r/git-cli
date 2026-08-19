# git-cli

`git-cli` is a standalone Go CLI for Git workflow utilities. The initial command group is `security`, which protects repositories from committing secrets.

## Security scanners

- **Gitleaks** — required/default; staged, repository and history scans.
- **detect-secrets** — required/default; staged and repository scans.
- **TruffleHog** — optional; deep/history scans.

`git-cli security` orchestrates these external scanners; it does not copy their detection engines.

## Install scanner dependencies on macOS

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

## Update with update-cli

This repository includes `update-cli.yaml` for the Go build/install workflow. The update source itself is configured in `.updater-cli/config.json` as the Git repository:

```text
https://github.com/r14r/git-cli.git
```

Equivalent initialization command:

```bash
update-cli init git-cli --from repository --repository https://github.com/r14r/git-cli.git
```

The normal update check is then:

```bash
update-cli check
```

## Repository setup

From a Git repository:

```bash
git-cli security install
```

This installs `.git/hooks/pre-commit` containing:

```bash
#!/usr/bin/env bash
set -e

exec git-cli security check-staged
```

It also creates `.git-cli.yaml` if missing. Existing unrelated pre-commit hooks are never overwritten.

## Commands

```text
git-cli security install

git-cli security check-staged
git-cli security check
git-cli security check --deep
git-cli security check-history

git-cli security scanner list
git-cli security scanner status
```

Additional management commands:

```text
git-cli security status
git-cli security uninstall
git-cli version
git-cli help
git-cli security help
```

Use `--json` with scan commands for machine-readable output.

## Scanner commands

`git-cli security scanner list` lists the scanners supported by this release and the scan modes each scanner supports.

`git-cli security scanner status` reports whether each scanner is enabled, required and installed. Inside a Git repository it uses `.git-cli.yaml`; outside a repository it displays the default configuration.

## Exit codes

- `0`: scan passed or informational command succeeded
- `1`: secret finding(s), commit should be blocked
- `2`: configuration/scanner/runtime/usage error

## Configuration

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

## detect-secrets baseline

Optional:

```bash
detect-secrets scan > .secrets.baseline
```

When that file exists, staged scans pass it to `detect-secrets-hook`.

## Security model

The pre-commit hook calls `git-cli security check-staged`. Gitleaks scans staged Git changes and detect-secrets scans the staged file paths. TruffleHog is reserved for deep/history scans so normal commits remain fast.

Git hooks can be bypassed with `git commit --no-verify`, so CI or pre-push scanning should be added as a second enforcement layer when stronger enforcement is required.
