package branchcmd

import "testing"

func TestProtectedBranches(t *testing.T) {
	for _, name := range []string{"main", "master", "develop", "trunk", "release/1", "hotfix/urgent"} {
		if !protected(name, "trunk") {
			t.Errorf("not protected: %s", name)
		}
	}
	if protected("feature/test", "trunk") {
		t.Fatal("feature incorrectly protected")
	}
}
