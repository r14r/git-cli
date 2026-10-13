set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

version := `cat VERSION`
bin := "git-cli"
repo := "r14r/git-cli"
install_prefix := "/usr/local"
install_bin := install_prefix + "/bin/" + bin

# Show available commands.
default:
    @just --list --unsorted

# Show version metadata.
version:
    @echo "{{bin}} {{version}}"
    @echo "install: {{install_bin}}"

# Check required local tools.
check-tools:
    @command -v go >/dev/null || { echo "go is required"; exit 1; }
    @command -v git >/dev/null || { echo "git is required"; exit 1; }
    @command -v just >/dev/null || { echo "just is required"; exit 1; }
    @go version
    @git --version
    @just --version

# Format Go sources in place.
fmt:
    gofmt -w cmd internal

# Verify formatting without modifying files.
fmt-check:
    @files="$$(gofmt -l cmd internal)";     if [ -n "$$files" ]; then       echo "Files require gofmt:";       echo "$$files";       exit 1;     fi

# Run go vet.
vet:
    go vet ./...

# Run all tests.
test:
    go test ./...

# Run tests verbosely.
test-v:
    go test -v ./...

# Run tests with race detection.
test-race:
    go test -race ./...

# Run formatting, vet and tests.
check: fmt-check vet test

# Build the versioned binary.
build:
    mkdir -p dist
    go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/{{bin}} ./cmd/git-cli

# Build and install to /usr/local/bin.
install: build
    install -m 0755 dist/{{bin}} {{install_bin}}

# Reinstall the current version.
reinstall: clean-build install

# Run git-cli from source with version injection.
run *args:
    go run -ldflags "-X main.version={{version}}" ./cmd/git-cli {{args}}

# Run the built binary.
exec *args: build
    ./dist/{{bin}} {{args}}

# Display CLI help.
cli-help:
    just run help

# Run git-cli doctor.
doctor:
    just run doctor

# Show repository health.
repo-health:
    just run repo health

# Detect project type.
project-detect:
    just run project detect

# Show detailed project information.
project-info:
    just run project info

# List configured PAT names.
pat-list:
    just run pat list

# Show PAT storage status.
pat-status:
    just run pat status

# Migrate plaintext PAT entries to encrypted storage.
pat-secure:
    just run pat secure

# Display a named PAT. Usage: just pat-display NAME
pat-display name:
    just run pat display "{{name}}"

# Delete a named PAT. Usage: just pat-delete NAME
pat-delete name:
    just run pat delete "{{name}}"

# Show gitignore status.
gitignore-status:
    just run gitignore status

# List available gitignore presets.
gitignore-presets:
    just run gitignore presets

# List GitHub gitignore templates.
gitignore-list:
    just run gitignore list

# Add gitignore entries by detected project type.
gitignore-scan:
    just run gitignore add --scan

# Explain why a path is ignored. Usage: just gitignore-explain PATH
gitignore-explain path:
    just run gitignore explain "{{path}}"

# Show tracked files that match ignore rules.
gitignore-tracked:
    just run gitignore tracked

# Run configured pre-commit checks.
precommit-run:
    just run precommit run

# Show pre-commit status.
precommit-status:
    just run precommit status

# List pre-commit presets/checks.
precommit-list:
    just run precommit list

# Show security scanner status.
security-status:
    just run security status

# Run staged security scan.
security-staged:
    just run security check-staged

# Run repository security scan.
security-check:
    just run security check

# Run deep security scan.
security-deep:
    just run security check --deep

# Scan Git history for secrets.
security-history:
    just run security check-history

# Preview untracked cleanup.
clean-preview:
    just run clean preview

# Preview ignored-file cleanup.
clean-ignored-preview:
    just run clean ignored --preview

# Remove ignored files explicitly.
clean-ignored-apply:
    just run clean ignored --apply

# Scan for large tracked files.
large-files:
    just run large-files scan

# Scan for large tracked files with a threshold. Usage: just large-files-mb 50
large-files-mb mb:
    just run large-files scan --threshold-mb "{{mb}}"

# List merged local branches.
branch-merged:
    just run branch merged

# List local branches ordered by age.
branch-stale:
    just run branch stale

# List GitHub Actions runs for the default repository.
actions-list:
    just run actions list --repository "{{repo}}"

# List GitHub Actions runs by state. Usage: just actions-state failure
actions-state state:
    just run actions list --repository "{{repo}}" --state "{{state}}"

# List GitHub Actions runs verbosely.
actions-verbose:
    just run actions list --repository "{{repo}}" --verbose

# List GitHub Actions runs verbosely by state.
actions-state-verbose state:
    just run actions list --repository "{{repo}}" --state "{{state}}" --verbose

# Delete GitHub Actions runs by state.
actions-delete state:
    just run actions delete --repository "{{repo}}" --state "{{state}}"

# Delete GitHub Actions runs by state with verbose output.
actions-delete-verbose state:
    just run actions delete --repository "{{repo}}" --state "{{state}}" --verbose

# Show Git status.
git-status:
    git status --short --branch

# Show recent commits.
git-log:
    git log --oneline --decorate -20

# Show local and remote branches.
git-branches:
    git branch -a -vv

# Fetch and prune remotes.
git-fetch:
    git fetch --all --prune

# Pull main with fast-forward only.
git-pull:
    git pull --ff-only

# Show changes.
diff:
    git diff

# Show staged changes.
diff-staged:
    git diff --cached

# Remove build/release artifacts only.
clean: clean-build

# Remove build/release artifacts only.
clean-build:
    rm -rf dist release

# Build a release ZIP and checksum.
release: clean-build check build
    mkdir -p release/git-cli-v{{version}}
    cp -R cmd internal go.mod VERSION README.md LICENSE .git-cli.yaml .gitignore update-cli.yaml .updater-cli justfile release/git-cli-v{{version}}/
    (cd release && zip -qr git-cli-v{{version}}.zip git-cli-v{{version}})
    shasum -a 256 release/git-cli-v{{version}}.zip

# Show release artifact files.
release-list:
    @find release -maxdepth 2 -type f -print 2>/dev/null || true
