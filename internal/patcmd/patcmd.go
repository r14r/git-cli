package patcmd

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
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

const (
	configDirEnv = "GIT_CLI_CONFIG_DIR"
	encPrefix    = "enc:v1:"
	keyPrefix    = "v1:"
)

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
	case "secure":
		return secure(out, errOut)
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
  git-cli pat secure

PAT values are encrypted with AES-256-GCM in:
  <prefix>/etc/git-config/pat.yaml

The encryption key is stored separately in:
  <prefix>/etc/git-config/pat.key

For /usr/local/bin/git-cli:
  /usr/local/etc/git-config/pat.yaml
  /usr/local/etc/git-config/pat.key

Both files use mode 0600 and the directory mode 0700.
Set GIT_CLI_CONFIG_DIR to override the config directory.
Use pat secure once to migrate plaintext v0.7.2 entries.
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
	key, err := loadOrCreateKey()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	encrypted, err := encryptToken(key, value)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	if s.PATs == nil {
		s.PATs = map[string]string{}
	}
	s.PATs[n] = encrypted
	if err := saveStore(path, s); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	fmt.Fprintf(out, "Stored encrypted PAT %q in %s.\n", n, path)
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

	if !strings.HasPrefix(value, encPrefix) {
		fmt.Fprintf(errOut, "PAT %q is stored in plaintext; run 'git-cli pat secure' first\n", n)
		return 2
	}
	key, err := loadKey()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	plain, err := decryptToken(key, value)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintln(out, plain)
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

func secure(out, errOut io.Writer) int {
	s, path, err := loadStore()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	key, err := loadOrCreateKey()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}

	changed := 0
	for name, value := range s.PATs {
		if strings.HasPrefix(value, encPrefix) {
			continue
		}
		encrypted, err := encryptToken(key, value)
		if err != nil {
			fmt.Fprintf(errOut, "encrypt PAT %q: %v\n", name, err)
			return 2
		}
		s.PATs[name] = encrypted
		changed++
	}
	if changed == 0 {
		fmt.Fprintln(out, "All PATs are already encrypted.")
		return 0
	}
	if err := saveStore(path, s); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Encrypted %d PAT(s) in %s.\n", changed, path)
	return 0
}

func status(out, errOut io.Writer) int {
	path, err := configPath()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	keyPath, err := keyPath()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "PAT file: %s\n", path)
	fmt.Fprintf(out, "Key file: %s\n", keyPath)

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "Status: not created")
		return 0
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "PAT mode: %04o\n", info.Mode().Perm())

	keyInfo, keyErr := os.Stat(keyPath)
	if keyErr == nil {
		fmt.Fprintf(out, "Key mode: %04o\n", keyInfo.Mode().Perm())
	} else if !errors.Is(keyErr, os.ErrNotExist) {
		fmt.Fprintln(errOut, keyErr)
		return 2
	}

	s, _, err := loadStore()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	plain := 0
	for _, value := range s.PATs {
		if !strings.HasPrefix(value, encPrefix) {
			plain++
		}
	}
	fmt.Fprintf(out, "PATs: %d\n", len(s.PATs))
	fmt.Fprintf(out, "Encrypted: %d\n", len(s.PATs)-plain)
	fmt.Fprintf(out, "Plaintext: %d\n", plain)

	insecure := info.Mode().Perm()&0o077 != 0
	if keyErr == nil {
		insecure = insecure || keyInfo.Mode().Perm()&0o077 != 0
	}
	if plain > 0 {
		fmt.Fprintln(out, "Warning: plaintext PATs found; run 'git-cli pat secure'")
		return 1
	}
	if insecure {
		fmt.Fprintln(out, "Warning: PAT or key file is readable or writable by group/others")
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
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pat.yaml"), nil
}

func keyPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pat.key"), nil
}

func configDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(configDirEnv)); dir != "" {
		return dir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve git-cli executable: %w", err)
	}
	binDir := filepath.Dir(filepath.Clean(exe))
	prefix := filepath.Dir(binDir)
	return filepath.Join(prefix, "etc", "git-config"), nil
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
	if err := ensureSecureDir(dir); err != nil {
		return err
	}

	names := make([]string, 0, len(s.PATs))
	for name := range s.PATs {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("# git-cli encrypted PAT storage\n")
	b.WriteString("# Values use AES-256-GCM. Keep pat.key private.\n")
	b.WriteString("pats:\n")
	for _, name := range names {
		fmt.Fprintf(&b, "  %s: %s\n", strconv.Quote(name), strconv.Quote(s.PATs[name]))
	}
	return writePrivateFile(path, []byte(b.String()))
}

func ensureSecureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create PAT config directory %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure PAT config directory %s: %w", dir, err)
	}
	return nil
}

func writePrivateFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := ensureSecureDir(dir); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func loadOrCreateKey() ([]byte, error) {
	path, err := keyPath()
	if err != nil {
		return nil, err
	}
	key, err := loadKey()
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate encryption key: %w", err)
	}
	content := []byte(keyPrefix + base64.StdEncoding.EncodeToString(key) + "\n")
	if err := writePrivateFile(path, content); err != nil {
		return nil, fmt.Errorf("write encryption key: %w", err)
	}
	return key, nil
}

func loadKey() ([]byte, error) {
	path, err := keyPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(string(b))
	if !strings.HasPrefix(value, keyPrefix) {
		return nil, errors.New("unsupported PAT key format")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, keyPrefix))
	if err != nil {
		return nil, fmt.Errorf("decode PAT key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid PAT key length: %d", len(key))
	}
	return key, nil
}

func encryptToken(key []byte, token string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nil, nonce, []byte(token), nil)
	payload := append(nonce, sealed...)
	return encPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func decryptToken(key []byte, value string) (string, error) {
	if !strings.HasPrefix(value, encPrefix) {
		return "", errors.New("PAT value is not encrypted")
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encPrefix))
	if err != nil {
		return "", fmt.Errorf("decode encrypted PAT: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) < gcm.NonceSize() {
		return "", errors.New("encrypted PAT payload is too short")
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("decrypt PAT: authentication failed")
	}
	return string(plain), nil
}

func TokenNames() ([]string, error) {
	s, _, err := loadStore()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(s.PATs))
	for name := range s.PATs {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func ResolveToken(name string) (string, error) {
	s, _, err := loadStore()
	if err != nil {
		return "", err
	}
	value, ok := s.PATs[name]
	if !ok || value == "" {
		return "", fmt.Errorf("PAT %q not found", name)
	}
	if !strings.HasPrefix(value, encPrefix) {
		return "", fmt.Errorf("PAT %q is stored in plaintext; run 'git-cli pat secure' first", name)
	}
	key, err := loadKey()
	if err != nil {
		return "", err
	}
	return decryptToken(key, value)
}
