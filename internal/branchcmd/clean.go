package branchcmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type candidate struct {
	ref, sha, name string
	remote bool
}

func gitRun(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	b, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(b)))
	}
	return strings.TrimSpace(string(b)), nil
}

func ancestor(root, sha, base string) bool {
	_, err := gitRun(root, "merge-base", "--is-ancestor", sha, base)
	return err == nil
}

func protected(name, base string) bool {
	switch name {
	case base, "main", "master", "develop", "development", "dev", "production", "staging":
		return true
	}
	return strings.HasPrefix(name, "release/") || strings.HasPrefix(name, "hotfix/")
}

func mergedPRs(root, base string) map[string]bool {
	// Restrict lookup to the repository selected by origin, not gh's current default.
	result := map[string]bool{}
	origin, err := gitRun(root, "remote", "get-url", "origin")
	if err != nil {
		return result
	}
	var slug string
	for _, prefix := range []string{"https://github.com/", "git@github.com:", "ssh://git@github.com/"} {
		if strings.HasPrefix(origin, prefix) {
			slug = strings.TrimSuffix(strings.TrimPrefix(origin, prefix), ".git")
		}
	}
	if strings.Count(slug, "/") != 1 || strings.ContainsAny(slug, " \t\n?#") {
		return result
	}
	type pr struct {
		HeadRefName string
		HeadRefOid string
		IsCrossRepository bool
		MergeCommit struct{ Oid string }
	}
	cmd := exec.Command("gh", "pr", "list", "--repo", slug, "--state", "merged", "--base", base, "--limit", "1000", "--json", "headRefName,headRefOid,isCrossRepository,mergeCommit")
	cmd.Dir = root
	b, err := cmd.Output()
	if err != nil {
		return result
	}
	var prs []pr
	if json.Unmarshal(b, &prs) != nil {
		return result
	}
	for _, p := range prs {
		if !p.IsCrossRepository && p.HeadRefOid != "" && p.MergeCommit.Oid != "" && ancestor(root, p.MergeCommit.Oid, "refs/remotes/origin/"+base) {
			result[p.HeadRefName+" "+p.HeadRefOid] = true
		}
	}
	return result
}

func clean(root string, args []string, out, errOut io.Writer) int {
	yes, dry := false, false
	for _, arg := range args {
		switch arg {
		case "--yes":
			yes = true
		case "--dry-run":
			dry = true
		default:
			fmt.Fprintln(errOut, "usage: git-cli branch --clean [--yes|--dry-run]")
			return 2
		}
	}
	if yes && dry {
		fmt.Fprintln(errOut, "--yes and --dry-run are mutually exclusive")
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(errOut, err)
		return 2
	}
	// Fetch only origin; dry-run does not prune or delete branches.
	fetch := []string{"fetch", "--no-tags", "origin", "+refs/heads/*:refs/remotes/origin/*"}
	if _, err := gitRun(root, fetch...); err != nil {
		return fail(err)
	}
	head, err := gitRun(root, "ls-remote", "--symref", "origin", "HEAD")
	if err != nil {
		return fail(err)
	}
	base := ""
	for _, line := range strings.Split(head, "\n") {
		if strings.HasPrefix(line, "ref: refs/heads/") {
			base = strings.SplitN(strings.TrimPrefix(line, "ref: refs/heads/"), "\t", 2)[0]
		}
	}
	if base == "" {
		return fail(fmt.Errorf("cannot determine origin default branch"))
	}
	baseRef := "refs/remotes/origin/" + base
	baseSHA, err := gitRun(root, "rev-parse", "--verify", baseRef)
	if err != nil {
		return fail(err)
	}
	refs, err := gitRun(root, "for-each-ref", "--format=%(refname) %(objectname) %(symref)", "refs/heads", "refs/remotes/origin")
	if err != nil {
		return fail(err)
	}
	wt, err := gitRun(root, "worktree", "list", "--porcelain")
	if err != nil {
		return fail(err)
	}
	occupied := map[string]bool{}
	for _, line := range strings.Split(wt, "\n") {
		if strings.HasPrefix(line, "branch ") {
			occupied[strings.TrimPrefix(line, "branch ")] = true
		}
	}
	live, err := gitRun(root, "ls-remote", "--heads", "origin")
	if err != nil {
		return fail(err)
	}
	remoteHeads := map[string]string{}
	for _, line := range strings.Split(live, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			remoteHeads[strings.TrimPrefix(fields[1], "refs/heads/")] = fields[0]
		}
	}
	prs := mergedPRs(root, base)
	var candidates []candidate
	fmt.Fprintf(out, "Branch cleanup (origin/%s)\n\n", base)
	for _, line := range strings.Split(refs, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		ref, sha := fields[0], fields[1]
		remote := strings.HasPrefix(ref, "refs/remotes/origin/")
		name := strings.TrimPrefix(ref, "refs/heads/")
		if remote {
			name = strings.TrimPrefix(ref, "refs/remotes/origin/")
			if remoteHeads[name] != sha {
				continue
			}
		}
		if protected(name, base) || occupied["refs/heads/"+name] {
			continue
		}
		if !ancestor(root, sha, baseSHA) && !prs[name+" "+sha] {
			fmt.Fprintf(out, "KEEP    %s (not verified merged)\n", ref)
			continue
		}
		candidates = append(candidates, candidate{ref, sha, name, remote})
		fmt.Fprintf(out, "DELETE  %s\n", ref)
	}
	fmt.Fprintf(out, "\n%d eligible branch(es)\n", len(candidates))
	if dry || len(candidates) == 0 {
		return 0
	}
	if !yes {
		fmt.Fprint(out, "Delete listed branches? [y/N] ")
		answer, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil || strings.ToLower(strings.TrimSpace(answer)) != "y" {
			fmt.Fprintln(out, "Cancelled.")
			return 0
		}
	}
	failed := false
	for _, c := range candidates {
		var err error
		if c.remote {
			ref := "refs/heads/" + c.name
			_, err = gitRun(root, "push", "--force-with-lease="+ref+":"+c.sha, "origin", ":"+ref)
		} else {
			// git branch checks all worktrees. Recheck the tip before deletion.
			var tip string
			tip, err = gitRun(root, "rev-parse", "--verify", c.ref)
			if err == nil && tip != c.sha {
				err = fmt.Errorf("branch changed since preview")
			}
			if err == nil {
				// -D is needed for verified squash merges; no unverified tip is eligible.
				_, err = gitRun(root, "branch", "-D", "--", c.name)
			}
		}
		if err != nil {
			fmt.Fprintf(errOut, "FAILED  %s: %v\n", c.ref, err)
			failed = true
		} else {
			fmt.Fprintf(out, "DELETED %s\n", c.ref)
		}
	}
	if _, err := gitRun(root, "remote", "prune", "origin"); err != nil {
		fmt.Fprintln(errOut, err)
		failed = true
	}
	if failed {
		return 1
	}
	return 0
}
