package gitignorecmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gitutil "git-cli/internal/git"
	"git-cli/internal/precommit"
)

const (
	githubRepo    = "github/gitignore"
	githubBranch  = "main"
	githubAPIBase = "https://api.github.com/repos/github/gitignore"
	githubRawBase = "https://raw.githubusercontent.com/github/gitignore/main"
)

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

var client httpClient = &http.Client{Timeout: 20 * time.Second}

type templateEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type treeResponse struct {
	Tree []templateEntry `json:"tree"`
}

type selection struct {
	Key          string
	TemplatePath string
	Extra        string
}

var presetTemplates = map[string]selection{
	"python":     {Key: "python", TemplatePath: "Python.gitignore"},
	"fastapi":    {Key: "fastapi", TemplatePath: "Python.gitignore"},
	"django":     {Key: "django", TemplatePath: "Python.gitignore"},
	"go":         {Key: "go", TemplatePath: "Go.gitignore"},
	"node":       {Key: "node", TemplatePath: "Node.gitignore"},
	"javascript": {Key: "javascript", TemplatePath: "Node.gitignore"},
	"typescript": {Key: "typescript", TemplatePath: "Node.gitignore"},
	"react":      {Key: "react", TemplatePath: "Node.gitignore"},
	"nextjs":     {Key: "nextjs", TemplatePath: "Node.gitignore", Extra: "\n# Next.js\n.next/\nout/\n"},
	"vue":        {Key: "vue", TemplatePath: "Node.gitignore"},
	"nuxt":       {Key: "nuxt", TemplatePath: "Node.gitignore", Extra: "\n# Nuxt\n.nuxt/\n.output/\n"},
	"laravel":    {Key: "laravel", Extra: "# Laravel\n/vendor/\n/node_modules/\n/public/build/\n/public/hot\n/storage/*.key\n/bootstrap/cache/*\n!.gitignore\n.env\n.env.backup\n.env.production\n.phpunit.result.cache\nHomestead.json\nHomestead.yaml\n"},
}

func Run(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		help(out)
		return 0
	}

	switch args[0] {
	case "list":
		return list(args[1:], out, errOut)
	case "add", "create":
		return add(args[1:], out, errOut)
	case "download":
		return download(args[1:], out, errOut)
	case "show":
		return show(args[1:], out, errOut)
	case "status":
		return status(out, errOut)
	case "remove":
		return remove(args[1:], out, errOut)
	case "presets":
		return presets(out)
	default:
		fmt.Fprintf(errOut, "unknown gitignore command: %s\n", args[0])
		help(errOut)
		return 2
	}
}

func help(w io.Writer) {
	fmt.Fprintln(w, `git-cli gitignore - create and manage .gitignore rules

Usage:
  git-cli gitignore add --for django
  git-cli gitignore add --scan
  git-cli gitignore add --template Python
  git-cli gitignore add --template Global/macOS
  git-cli gitignore list [--filter TERM]
  git-cli gitignore presets
  git-cli gitignore show --for django
  git-cli gitignore show --template Python
  git-cli gitignore download --template Python [--output FILE]
  git-cli gitignore status
  git-cli gitignore remove --for django

Templates are loaded from GitHub's github/gitignore repository. Existing user rules are preserved; git-cli manages only marked sections.`)
}

func presets(out io.Writer) int {
	keys := make([]string, 0, len(presetTemplates))
	for k := range presetTemplates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := presetTemplates[k]
		remote := s.TemplatePath
		if remote == "" {
			remote = "built-in"
		}
		fmt.Fprintf(out, "%-12s %s\n", k, remote)
	}
	return 0
}

func list(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("gitignore list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	filter := fs.String("filter", "", "filter template names/paths")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	entries, err := remoteTemplates()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	needle := strings.ToLower(strings.TrimSpace(*filter))
	for _, p := range entries {
		if needle != "" && !strings.Contains(strings.ToLower(p), needle) {
			continue
		}
		fmt.Fprintln(out, strings.TrimSuffix(p, ".gitignore"))
	}
	return 0
}

func add(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("gitignore add", flag.ContinueOnError)
	fs.SetOutput(errOut)
	preset := fs.String("for", "", "preset/group, e.g. django")
	tmpl := fs.String("template", "", "GitHub template name/path")
	scan := fs.Bool("scan", false, "detect application from current repository")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	selectedPreset := strings.TrimSpace(*preset)
	if *scan {
		if selectedPreset != "" || strings.TrimSpace(*tmpl) != "" {
			fmt.Fprintln(errOut, "--scan cannot be combined with --for or --template")
			return 2
		}
		selectedPreset, err = precommit.Detect(root)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
		fmt.Fprintf(out, "Detected project: %s\n", selectedPreset)
	}
	sel, err := resolveSelection(selectedPreset, *tmpl)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	content, err := selectionContent(sel)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	path := filepath.Join(root, ".gitignore")
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(errOut, err)
		return 2
	}
	merged := upsertBlock(string(current), sel.Key, content)
	if err := os.WriteFile(path, []byte(merged), 0o644); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Updated: %s\nSection: %s\n", path, sel.Key)
	if sel.TemplatePath != "" {
		fmt.Fprintf(out, "Source: %s/%s@%s:%s\n", "https://github.com", githubRepo, githubBranch, sel.TemplatePath)
	}
	return 0
}

func show(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("gitignore show", flag.ContinueOnError)
	fs.SetOutput(errOut)
	preset := fs.String("for", "", "preset/group")
	tmpl := fs.String("template", "", "GitHub template name/path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sel, err := resolveSelection(*preset, *tmpl)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	content, err := selectionContent(sel)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprint(out, managedBlock(sel.Key, content))
	return 0
}

func download(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("gitignore download", flag.ContinueOnError)
	fs.SetOutput(errOut)
	tmpl := fs.String("template", "", "GitHub template name/path")
	output := fs.String("output", "", "output file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*tmpl) == "" {
		fmt.Fprintln(errOut, "--template is required")
		return 2
	}
	path := normalizeTemplatePath(*tmpl)
	content, err := fetchTemplate(path)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	dest := strings.TrimSpace(*output)
	if dest == "" {
		dest = filepath.Base(path)
	}
	if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Downloaded %s -> %s\n", path, dest)
	return 0
}

func status(out, errOut io.Writer) int {
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	path := filepath.Join(root, ".gitignore")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, ".gitignore: not found")
		return 0
	}
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	sections := managedSections(string(b))
	fmt.Fprintf(out, ".gitignore: %s\n", path)
	if len(sections) == 0 {
		fmt.Fprintln(out, "Managed sections: none")
		return 0
	}
	fmt.Fprintln(out, "Managed sections:")
	for _, s := range sections {
		fmt.Fprintf(out, "  - %s\n", s)
	}
	return 0
}

func remove(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("gitignore remove", flag.ContinueOnError)
	fs.SetOutput(errOut)
	preset := fs.String("for", "", "preset/group")
	tmpl := fs.String("template", "", "GitHub template name/path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	sel, err := resolveSelection(*preset, *tmpl)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	root, err := gitutil.Root()
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	path := filepath.Join(root, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	updated, removed := removeBlock(string(b), sel.Key)
	if !removed {
		fmt.Fprintf(out, "Section not found: %s\n", sel.Key)
		return 0
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	fmt.Fprintf(out, "Removed section: %s\n", sel.Key)
	return 0
}

func resolveSelection(preset, tmpl string) (selection, error) {
	preset = strings.ToLower(strings.TrimSpace(preset))
	tmpl = strings.TrimSpace(tmpl)
	if (preset == "" && tmpl == "") || (preset != "" && tmpl != "") {
		return selection{}, errors.New("use exactly one of --for <preset> or --template <name/path>")
	}
	if preset != "" {
		if preset == "laracel" {
			preset = "laravel"
		}
		s, ok := presetTemplates[preset]
		if !ok {
			return selection{}, fmt.Errorf("unknown preset %q; use `git-cli gitignore presets`", preset)
		}
		return s, nil
	}
	path := normalizeTemplatePath(tmpl)
	return selection{Key: templateKey(path), TemplatePath: path}, nil
}

func selectionContent(sel selection) (string, error) {
	var parts []string
	if sel.TemplatePath != "" {
		content, err := fetchTemplate(sel.TemplatePath)
		if err != nil {
			return "", err
		}
		parts = append(parts, strings.TrimSpace(content))
	}
	if strings.TrimSpace(sel.Extra) != "" {
		parts = append(parts, strings.TrimSpace(sel.Extra))
	}
	return strings.Join(parts, "\n\n") + "\n", nil
}

func normalizeTemplatePath(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "/"))
	if !strings.HasSuffix(strings.ToLower(value), ".gitignore") {
		value += ".gitignore"
	}
	return value
}

func templateKey(path string) string {
	key := strings.TrimSuffix(path, ".gitignore")
	key = strings.ReplaceAll(key, "/", "-")
	key = strings.ReplaceAll(key, " ", "-")
	return strings.ToLower(key)
}

func fetchTemplate(path string) (string, error) {
	u := githubRawBase + "/" + escapePath(path)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "git-cli")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("template %q not found in %s (HTTP %d)", path, githubRepo, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func remoteTemplates() ([]string, error) {
	u := githubAPIBase + "/git/trees/" + githubBranch + "?recursive=1"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "git-cli")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list GitHub templates: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list GitHub templates: HTTP %d", resp.StatusCode)
	}
	var doc treeResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&doc); err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range doc.Tree {
		if e.Type == "blob" && strings.HasSuffix(strings.ToLower(e.Path), ".gitignore") {
			paths = append(paths, e.Path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func managedBlock(key, content string) string {
	content = strings.TrimSpace(content)
	return fmt.Sprintf("# >>> git-cli gitignore: %s\n%s\n# <<< git-cli gitignore: %s\n", key, content, key)
}

func upsertBlock(existing, key, content string) string {
	block := managedBlock(key, content)
	without, _ := removeBlock(existing, key)
	without = strings.TrimRight(without, "\n")
	if without == "" {
		return block
	}
	return without + "\n\n" + block
}

func removeBlock(existing, key string) (string, bool) {
	startMarker := "# >>> git-cli gitignore: " + key
	endMarker := "# <<< git-cli gitignore: " + key
	start := strings.Index(existing, startMarker)
	if start < 0 {
		return existing, false
	}
	endRel := strings.Index(existing[start:], endMarker)
	if endRel < 0 {
		return existing, false
	}
	end := start + endRel + len(endMarker)
	if end < len(existing) && existing[end] == '\r' {
		end++
	}
	if end < len(existing) && existing[end] == '\n' {
		end++
	}
	result := existing[:start] + existing[end:]
	return strings.TrimRight(result, "\n") + conditionalNewline(result), true
}

func conditionalNewline(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return "\n"
}

func managedSections(content string) []string {
	const prefix = "# >>> git-cli gitignore: "
	var out []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, prefix)))
		}
	}
	sort.Strings(out)
	return out
}
