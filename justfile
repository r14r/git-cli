set shell := ["bash", "-uc"]

version := `cat VERSION`
bin := "git-cli"

build:
    mkdir -p dist
    go build -trimpath -ldflags "-s -w -X main.version={{version}}" -o dist/{{bin}} ./cmd/git-cli

install: build
    install -m 0755 dist/{{bin}} /usr/local/bin/{{bin}}

check:
    gofmt -w cmd internal
    go vet ./...
    go test ./...

run *args:
    go run -ldflags "-X main.version={{version}}" ./cmd/git-cli {{args}}

clean:
    rm -rf dist release

release: clean check build
    mkdir -p release/git-cli-v{{version}}
    cp -R cmd internal go.mod VERSION README.md LICENSE .git-cli.yaml .gitignore update-cli.yaml .updater-cli justfile release/git-cli-v{{version}}/
    (cd release && zip -qr git-cli-v{{version}}.zip git-cli-v{{version}})
    shasum -a 256 release/git-cli-v{{version}}.zip
