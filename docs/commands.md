# Command guide

| Command | Purpose |
|---|---|
| git-cli branch merged | List local merged branches |
| git-cli branch stale | List old branches |
| git-cli branch --clean --dry-run | Fetch origin and preview eligible branches |
| git-cli branch --clean | Confirm and delete eligible branches |
| git-cli branch --clean --yes | Delete eligible branches without prompt |
| git-cl-doctor | Diagnose problems and show suggestions |
| git-cli doctor --fix | Create missing configuration and hooks safely |
| git-cli wizard | Select a command interactively |

Use a dry-run first. Squash merge recognition requires authenticated gh and exact PR head and merge commit verification. Remote deletion uses a SHA lease. Local deletion preserves checked-out branches. External packages remain manual.
