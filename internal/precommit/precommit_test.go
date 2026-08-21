package precommit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizePreset(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{"python", "python"}, {"FastAPI", "fastapi"}, {"django", "django"}, {"laracel", "laravel"}} {
		got, err := NormalizePreset(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("NormalizePreset(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestDetectFrameworks(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"python", map[string]string{"pyproject.toml": "[project]\nname='demo'\n"}, "python"},
		{"fastapi", map[string]string{"pyproject.toml": "dependencies=['fastapi']\n"}, "fastapi"},
		{"django", map[string]string{"manage.py": "#!/usr/bin/env python3\n", "requirements.txt": "Django\n"}, "django"},
		{"laravel", map[string]string{"artisan": "#!/usr/bin/env php\n", "composer.json": `{"require":{"laravel/framework":"^12"}}`}, "laravel"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tc.files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Detect(root)
			if err != nil || got != tc.want {
				t.Fatalf("Detect() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestHookRunsSecurityBeforePreset(t *testing.T) {
	h := HookContent()
	security := strings.Index(h, "git-cli security check-staged")
	preset := strings.Index(h, "git-cli precommit run")
	if security < 0 || preset < 0 || security > preset {
		t.Fatalf("unexpected hook order: %s", h)
	}
}
