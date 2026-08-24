package largefilescmd

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	gitutil "git-cli/internal/git"
)

type item struct{ size int64; path string }

func Run(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("large-files scan", flag.ContinueOnError)
	fs.SetOutput(errOut)
	mb := fs.Int64("threshold-mb", 25, "minimum size in MB")
	if len(args) == 0 || args[0] != "scan" { fmt.Fprintln(errOut, "usage: git-cli large-files scan [--threshold-mb 25]"); return 2 }
	if err := fs.Parse(args[1:]); err != nil { return 2 }
	root, err := gitutil.Root(); if err != nil { fmt.Fprintln(errOut, err); return 2 }
	cmd := exec.Command("git", "ls-files", "-s"); cmd.Dir = root
	b, err := cmd.Output(); if err != nil { fmt.Fprintln(errOut, err); return 2 }
	var items []item
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line); if len(f) < 4 { continue }
		sha := f[1]; path := strings.Join(f[3:], " ")
		c := exec.Command("git", "cat-file", "-s", sha); c.Dir = root
		szB, e := c.Output(); if e != nil { continue }
		sz, e := strconv.ParseInt(strings.TrimSpace(string(szB)), 10, 64); if e == nil && sz >= *mb*1024*1024 { items = append(items, item{sz, path}) }
	}
	sort.Slice(items, func(i,j int) bool { return items[i].size > items[j].size })
	if len(items)==0 { fmt.Fprintf(out, "No tracked files >= %d MB.\n", *mb); return 0 }
	for _, it := range items { fmt.Fprintf(out, "%8.1f MB  %s\n", float64(it.size)/(1024*1024), it.path) }
	return 1
}
