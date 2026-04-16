package judge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestExtractAffectedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	repo := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}

	planContent := "" +
		"# Implementation Plan\n\n" +
		"## 1. Overview\nThis is a test plan.\n\n" +
		"## 2. Affected Files\n" +
		"- `internal/agent/launcher.go`: update launch logic\n" +
		"- `internal/agent/adapter.go`: update context\n" +
		"- `" + filepath.Join(repo, "abs/path/to/file.go") + "`: absolute in-repo\n" +
		"- `README.md`: update docs\n" +
		"- Skip this: `not a file` (maybe?)\n" +
		"- `src/main.go`\n\n" +
		"## 3. Implementation Phases\n- Phase 1: Do things\n"

	planFile := filepath.Join(tmpDir, "plan.md")
	if err := os.WriteFile(planFile, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		filepath.Join(repo, "internal/agent/launcher.go"),
		filepath.Join(repo, "internal/agent/adapter.go"),
		filepath.Join(repo, "abs/path/to/file.go"),
		filepath.Join(repo, "README.md"),
		filepath.Join(repo, "src/main.go"),
	}

	got, err := ExtractAffectedFiles(planFile, repo)
	if err != nil {
		t.Errorf("ExtractAffectedFiles failed: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestExtractAffectedFilesDifferentHeader(t *testing.T) {
	tmpDir := t.TempDir()
	repo := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repo, 0755)

	planContent := "#### Affected Files\n" +
		"* `pkg/foo.go`\n" +
		"* `pkg/bar.go`\n\n" +
		"#### Next Section\n"
	planFile := filepath.Join(tmpDir, "plan.md")
	if err := os.WriteFile(planFile, []byte(planContent), 0644); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		filepath.Join(repo, "pkg/foo.go"),
		filepath.Join(repo, "pkg/bar.go"),
	}
	got, err := ExtractAffectedFiles(planFile, repo)
	if err != nil {
		t.Errorf("ExtractAffectedFiles failed: %v", err)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestExtractAffectedFilesRejectsEscape(t *testing.T) {
	tmpDir := t.TempDir()
	repo := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repo, 0755)

	// Relative traversal, absolute out-of-repo, and legitimate entries
	// mixed together. Only the in-repo path should survive.
	planContent := "## Affected Files\n" +
		"- `../../.ssh/id_rsa`\n" +
		"- `/etc/passwd`\n" +
		"- `good/file.go`\n"
	planFile := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(planFile, []byte(planContent), 0644)

	got, err := ExtractAffectedFiles(planFile, repo)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{filepath.Join(repo, "good/file.go")}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestExtractAffectedFilesNonBacktickedList(t *testing.T) {
	// Plans that forget backticks must still yield file paths when the
	// list entries look path-like. Prose bullets with spaces remain
	// rejected.
	tmpDir := t.TempDir()
	repo := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repo, 0755)

	planContent := "## Affected Files\n" +
		"- internal/agent/launcher.go: update launch logic\n" +
		"- cmd/crossagent/main.go\n" +
		"- This is a prose bullet with many words\n" +
		"* pkg/foo.go\n"
	planFile := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(planFile, []byte(planContent), 0644)

	got, err := ExtractAffectedFiles(planFile, repo)
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{
		filepath.Join(repo, "internal/agent/launcher.go"),
		filepath.Join(repo, "cmd/crossagent/main.go"),
		filepath.Join(repo, "pkg/foo.go"),
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestValidatePath(t *testing.T) {
	repo := "/tmp/repo"
	cases := []struct {
		name      string
		candidate string
		want      string
		ok        bool
	}{
		{"relative in-repo", "internal/foo.go", "/tmp/repo/internal/foo.go", true},
		{"absolute in-repo", "/tmp/repo/x.go", "/tmp/repo/x.go", true},
		{"relative escape", "../secret", "", false},
		{"absolute out-of-repo", "/etc/passwd", "", false},
		{"prose label", "file", "", false},
		{"space token", "has space", "", false},
		{"empty", "", "", false},
		{"repo root dot", ".", "", false},
		{"repo root absolute", "/tmp/repo", "", false},
		{"repo root trailing slash", "/tmp/repo/", "", false},
	}
	for _, c := range cases {
		got, ok := ValidatePath(repo, c.candidate)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: ValidatePath(%q, %q) = (%q, %v); want (%q, %v)",
				c.name, repo, c.candidate, got, ok, c.want, c.ok)
		}
	}
}
