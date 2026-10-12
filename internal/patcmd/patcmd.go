package patcmd

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const configDirEnv = "GIT_CLI_CONFIG_DIR"

type store struct {
	PATs map[string]string
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

PAT values are stored in pat.yaml below the installation prefix:
  <prefix>/etc/git-config/pat.yaml

For /usr/local/bin/git-cli this is:
  /usr/local/etc/git-config/pat.yaml

Set GIT_CLI_CONFIG_DIR to override the config directory.
The directory is created with mode 0700 and pat.yaml with mode 0600.
Using --token-stdin avoids putting a token in shell history or process arguments.`)
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

	s, path, err := loadStore()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if s.PATs == nil {
		s.PATs = map[string]string{}
	}
	s.PATs[n] = value
	if err := saveStore(path, s); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	fmt.Fprintf(out, "Stored PAT %q in %s.\n", n, path)
	return 0
}

func display(args []string, out, errOut io.Writer) int {
	n, code := parseNameArgs("pat display", args, errOut)
	if code != 0 {
		return code
	}
	s, _, err := loadStore()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	value, ok := s.PATs[n]
	if !ok || value == "" {
		fmt.Fprintf(errOut, "PAT %q not found\n", n)
		return 2
	}
	fmt.Fprintln(out, value)
	return 0
}

func list(out, errOut io.Writer) int {
	s, _, err := loadStore()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if len(s.PATs) == 0 {
		fmt.Fprintln(out, "No PATs registered.")
		return 0
	}
	names := make([]string, 0, len(s.PATs))
	for name := range s.PATs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintln(out, name)
	}
	return 0
}

func deletePAT(args []string, out, errOut io.Writer) int {
	n, code := parseNameArgs("pat delete", args, errOut)
	if code != 0 {
		return code
	}
	s, path, err := loadStore()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if _, ok := s.PATs[n]; !ok {
		fmt.Fprintf(out, "PAT %q not found.\n", n)
		return 0
	}
	delete(s.PATs, n)
	if err := saveStore(path, s); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Deleted PAT %q.\n", n)
	return 0
}

func status(out, errOut io.Writer) int {
	path, err := configPath()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "PAT file: %s\n", path)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "Status: not created")
		return 0
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Mode: %04o\n", info.Mode().Perm())
	s, _, err := loadStore()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "PATs: %d\n", len(s.PATs))
	if info.Mode().Perm()&0o077 != 0 {
		fmt.Fprintln(out, "Warning: PAT file is readable or writable by group/others")
		return 1
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

func configPath() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(configDirEnv)); dir != "" {
		return filepath.Join(dir, "pat.yaml"), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve git-cli executable: %w", err)
	}
	return configPathForExecutable(exe), nil
}

func configPathForExecutable(exe string) string {
	binDir := filepath.Dir(filepath.Clean(exe))
	prefix := filepath.Dir(binDir)
	return filepath.Join(prefix, "etc", "git-config", "pat.yaml")
}

func loadStore() (store, string, error) {
	path, err := configPath()
	if err != nil {
		return store{}, "", err
	}
	s := store{PATs: map[string]string{}}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, path, nil
	}
	if err != nil {
		return store{}, path, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNo := 0
	seenHeader := false
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !seenHeader {
			if line != "pats:" {
				return store{}, path, fmt.Errorf("%s:%d: expected pats:", path, lineNo)
			}
			seenHeader = true
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return store{}, path, fmt.Errorf("%s:%d: invalid PAT entry", path, lineNo)
		}
		name, err := strconv.Unquote(strings.TrimSpace(key))
		if err != nil {
			return store{}, path, fmt.Errorf("%s:%d: invalid PAT name: %w", path, lineNo, err)
		}
		token, err := strconv.Unquote(strings.TrimSpace(value))
		if err != nil {
			return store{}, path, fmt.Errorf("%s:%d: invalid PAT token: %w", path, lineNo, err)
		}
		s.PATs[name] = token
	}
	if err := scanner.Err(); err != nil {
		return store{}, path, err
	}
	return s, path, nil
}

func saveStore(path string, s store) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create PAT config directory %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure PAT config directory %s: %w", dir, err)
	}

	names := make([]string, 0, len(s.PATs))
	for name := range s.PATs {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("# git-cli PAT storage\n")
	b.WriteString("# Keep this file private.\n")
	b.WriteString("pats:\n")
	for _, name := range names {
		fmt.Fprintf(&b, "  %s: %s\n", strconv.Quote(name), strconv.Quote(s.PATs[name]))
	}

	tmp, err := os.CreateTemp(dir, ".pat.yaml-*")
	if err != nil {
		return fmt.Errorf("create temporary PAT file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := io.WriteString(tmp, b.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("install PAT file %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure PAT file %s: %w", path, err)
	}
	return nil
}
