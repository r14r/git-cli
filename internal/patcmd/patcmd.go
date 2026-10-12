package patcmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	credentialProtocol = "https"
	credentialHost     = "github.com"
	credentialUsername = "x-access-token"
)

type metadata struct {
	Names []string `json:"names"`
}

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(out)
		return 0
	}

	switch args[0] {
	case "create":
		return create(args[1:], out, errOut)
	case "display":
		return display(args[1:], out, errOut)
	case "list":
		return list(out, errOut)
	case "delete":
		return deletePAT(args[1:], out, errOut)
	case "status":
		return status(out, errOut)
	default:
		fmt.Fprintf(errOut, "unknown pat command: %s\n", args[0])
		help(errOut)
		return 2
	}
}

func help(w io.Writer) {
	io.WriteString(w, `git-cli pat - local Git PAT management

Usage:
  git-cli pat create --name <NAME> --token <TOKEN>
  printf '%s' "$GITHUB_TOKEN" | git-cli pat create --name <NAME> --token-stdin
  git-cli pat display <NAME>
  git-cli pat display --name <NAME>
  git-cli pat list
  git-cli pat delete <NAME>
  git-cli pat delete --name <NAME>
  git-cli pat status

PAT values are stored through Git's configured credential helper. git-cli stores
only PAT names in local metadata. Using --token-stdin avoids putting a token in
shell history or process arguments.`)
}

func create(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("pat create", flag.ContinueOnError)
	fs.SetOutput(errOut)
	name := fs.String("name", "", "PAT name")
	token := fs.String("token", "", "PAT value")
	tokenStdin := fs.Bool("token-stdin", false, "read PAT value from stdin")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	n, err := normalizeName(*name)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if (*token != "" && *tokenStdin) || (*token == "" && !*tokenStdin) {
		fmt.Fprintln(errOut, "use exactly one of --token <TOKEN> or --token-stdin")
		return 2
	}
	if err := requireCredentialHelper(); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	value := *token
	if *tokenStdin {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
		value = strings.TrimSpace(string(b))
	}
	if value == "" {
		fmt.Fprintln(errOut, "token must not be empty")
		return 2
	}

	if err := credential("approve", n, value); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	stored, err := fillCredential(n)
	if err != nil || stored != value {
		_ = credential("reject", n, "")
		if err != nil {
			fmt.Fprintf(errOut, "credential helper did not persist PAT %q: %v\n", n, err)
		} else {
			fmt.Fprintf(errOut, "credential helper roundtrip failed for PAT %q\n", n)
		}
		return 2
	}

	if err := addName(n); err != nil {
		_ = credential("reject", n, "")
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Stored PAT %q using Git credential helper.\n", n)
	return 0
}

func display(args []string, out, errOut io.Writer) int {
	n, code := parseNameArgs("pat display", args, errOut)
	if code != 0 {
		return code
	}
	value, err := fillCredential(n)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintln(out, value)
	return 0
}

func list(out, errOut io.Writer) int {
	m, err := loadMetadata()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if len(m.Names) == 0 {
		fmt.Fprintln(out, "No PATs registered.")
		return 0
	}
	sort.Strings(m.Names)
	for _, name := range m.Names {
		fmt.Fprintln(out, name)
	}
	return 0
}

func deletePAT(args []string, out, errOut io.Writer) int {
	n, code := parseNameArgs("pat delete", args, errOut)
	if code != 0 {
		return code
	}
	if err := credential("reject", n, ""); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if err := removeName(n); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Deleted PAT %q.\n", n)
	return 0
}

func status(out, errOut io.Writer) int {
	helpers, err := credentialHelpers()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	path, err := metadataPath()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Credential host: %s\n", credentialHost)
	fmt.Fprintf(out, "Credential namespace: %s\n", credentialPath("<NAME>"))
	fmt.Fprintf(out, "Metadata: %s\n", path)
	if len(helpers) == 0 {
		fmt.Fprintln(out, "Credential helper: not configured")
		return 1
	}
	fmt.Fprintln(out, "Credential helper(s):")
	for _, helper := range helpers {
		fmt.Fprintf(out, "  - %s\n", helper)
	}
	return 0
}

func parseNameArgs(command string, args []string, errOut io.Writer) (string, int) {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(errOut)
	name := fs.String("name", "", "PAT name")
	if err := fs.Parse(args); err != nil {
		return "", 2
	}
	value := strings.TrimSpace(*name)
	if value == "" && fs.NArg() == 1 {
		value = fs.Arg(0)
	} else if value != "" && fs.NArg() != 0 {
		fmt.Fprintln(errOut, "use either positional NAME or --name <NAME>, not both")
		return "", 2
	} else if value == "" && fs.NArg() > 1 {
		fmt.Fprintln(errOut, "too many positional arguments")
		return "", 2
	}
	n, err := normalizeName(value)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return "", 2
	}
	return n, 0
}

func normalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("PAT name is required")
	}
	if strings.ContainsAny(name, "\r\n\x00") {
		return "", errors.New("PAT name contains invalid characters")
	}
	return name, nil
}

func credentialPath(name string) string {
	return "git-cli/pat/" + url.PathEscape(name)
}

func credentialInput(name, token string) string {
	var input strings.Builder
	fmt.Fprintf(&input, "protocol=%s\nhost=%s\npath=%s\nusername=%s\n", credentialProtocol, credentialHost, credentialPath(name), credentialUsername)
	if token != "" {
		fmt.Fprintf(&input, "password=%s\n", token)
	}
	input.WriteString("\n")
	return input.String()
}

func credential(action, name, token string) error {
	cmd := exec.Command("git", "-c", "credential.useHttpPath=true", "credential", action)
	cmd.Stdin = strings.NewReader(credentialInput(name, token))
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("git credential %s failed: %s", action, msg)
	}
	return nil
}

func fillCredential(name string) (string, error) {
	cmd := exec.Command("git", "-c", "credential.useHttpPath=true", "credential", "fill")
	cmd.Stdin = strings.NewReader(credentialInput(name, ""))
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("PAT %q not found: %s", name, msg)
	}

	s := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "password=") {
			value := strings.TrimPrefix(line, "password=")
			if value != "" {
				return value, nil
			}
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("PAT %q not found", name)
}

func credentialHelpers() ([]string, error) {
	cmd := exec.Command("git", "config", "--get-all", "credential.helper")
	b, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	var helpers []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			helpers = append(helpers, line)
		}
	}
	return helpers, nil
}

func requireCredentialHelper() error {
	helpers, err := credentialHelpers()
	if err != nil {
		return err
	}
	if len(helpers) == 0 {
		return errors.New("no Git credential helper is configured; configure a secure helper before storing PATs")
	}
	return nil
}

func metadataPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "git-cli", "pats.json"), nil
}

func loadMetadata() (metadata, error) {
	path, err := metadataPath()
	if err != nil {
		return metadata{}, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return metadata{}, nil
	}
	if err != nil {
		return metadata{}, err
	}
	var m metadata
	if err := json.Unmarshal(b, &m); err != nil {
		return metadata{}, fmt.Errorf("read PAT metadata: %w", err)
	}
	return m, nil
}

func saveMetadata(m metadata) error {
	path, err := metadataPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	sort.Strings(m.Names)
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}

func addName(name string) error {
	m, err := loadMetadata()
	if err != nil {
		return err
	}
	for _, existing := range m.Names {
		if existing == name {
			return saveMetadata(m)
		}
	}
	m.Names = append(m.Names, name)
	return saveMetadata(m)
}

func removeName(name string) error {
	m, err := loadMetadata()
	if err != nil {
		return err
	}
	out := m.Names[:0]
	for _, existing := range m.Names {
		if existing != name {
			out = append(out, existing)
		}
	}
	m.Names = out
	return saveMetadata(m)
}
