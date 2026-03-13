package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitGlobalMemory(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()
	os.MkdirAll(filepath.Join(home, "memory"), 0755)

	if err := InitGlobalMemory(); err != nil {
		t.Fatalf("InitGlobalMemory: %v", err)
	}

	// Check files exist
	ctx := filepath.Join(home, "memory", "global-context.md")
	data, err := os.ReadFile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Crossagent Global Context") {
		t.Error("global-context.md should contain header")
	}

	lessons := filepath.Join(home, "memory", "lessons-learned.md")
	data, err = os.ReadFile(lessons)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Lessons Learned") {
		t.Error("lessons-learned.md should contain header")
	}

	// Idempotent: calling again should not overwrite
	os.WriteFile(ctx, []byte("custom content"), 0644)
	if err := InitGlobalMemory(); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(ctx)
	if string(data) != "custom content" {
		t.Error("InitGlobalMemory should not overwrite existing file")
	}
}

func TestInitProjectMemory(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()
	os.MkdirAll(ProjectsDir(), 0755)

	if err := InitProjectMemory("testproj"); err != nil {
		t.Fatalf("InitProjectMemory: %v", err)
	}

	ctx := filepath.Join(home, "projects", "testproj", "memory", "project-context.md")
	data, err := os.ReadFile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Project Context: testproj") {
		t.Error("project-context.md should contain project name")
	}

	lessons := filepath.Join(home, "projects", "testproj", "memory", "lessons-learned.md")
	data, err = os.ReadFile(lessons)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Project Lessons Learned: testproj") {
		t.Error("lessons-learned.md should contain project name")
	}
}

func TestInitWorkflowMemory(t *testing.T) {
	dir := t.TempDir()

	oldTime := currentTime
	currentTime = func() time.Time { return time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC) }
	defer func() { currentTime = oldTime }()

	if err := InitWorkflowMemory(dir, "test-wf", "Build a feature", "/tmp/repo"); err != nil {
		t.Fatalf("InitWorkflowMemory: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "memory.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"Workflow Memory: test-wf",
		"| Workflow | test-wf |",
		"| Repository | /tmp/repo |",
		"| Created | 2026-03-13 |",
		"Build a feature",
		"## Decisions Log",
		"## Key Findings",
		"## Session Notes",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("memory.md should contain %q", check)
		}
	}

	// Idempotent
	if err := InitWorkflowMemory(dir, "test-wf", "different", "/tmp/other"); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(filepath.Join(dir, "memory.md"))
	if string(data2) != content {
		t.Error("InitWorkflowMemory should not overwrite existing file")
	}
}

func TestReadMemoryContent(t *testing.T) {
	dir := t.TempDir()

	// Non-existent file
	content, err := ReadMemoryContent(filepath.Join(dir, "nope.md"))
	if err != nil {
		t.Fatal(err)
	}
	if content != "" {
		t.Error("non-existent file should return empty")
	}

	// File with fewer than 5 non-empty lines
	sparse := filepath.Join(dir, "sparse.md")
	os.WriteFile(sparse, []byte("# Title\n\nLine 1\nLine 2\n\n"), 0644)
	content, _ = ReadMemoryContent(sparse)
	if content != "" {
		t.Error("sparse file should return empty (not substantive)")
	}

	// File with enough content
	rich := filepath.Join(dir, "rich.md")
	lines := "# Title\nLine 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\n"
	os.WriteFile(rich, []byte(lines), 0644)
	content, _ = ReadMemoryContent(rich)
	if content == "" {
		t.Error("rich file should return content")
	}
	if !strings.Contains(content, "# Title") {
		t.Error("content should contain file content")
	}
}

func TestListProjectMemoryFiles(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	memDir := filepath.Join(home, "projects", "testproj", "memory")
	featDir := filepath.Join(memDir, "features")
	os.MkdirAll(featDir, 0755)

	os.WriteFile(filepath.Join(memDir, "project-context.md"), []byte("ctx"), 0644)
	os.WriteFile(filepath.Join(memDir, "lessons-learned.md"), []byte("lessons"), 0644)
	os.WriteFile(filepath.Join(featDir, "feat1.md"), []byte("feat1"), 0644)
	os.WriteFile(filepath.Join(memDir, "notes.txt"), []byte("not md"), 0644) // should be excluded

	files, err := ListProjectMemoryFiles("testproj")
	if err != nil {
		t.Fatalf("ListProjectMemoryFiles: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("expected 3 .md files, got %d: %v", len(files), files)
	}

	// Check that all are .md files
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			t.Errorf("expected .md file, got %s", f)
		}
	}
}

func TestGlobalMemoryDir(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	expected := filepath.Join(home, "memory")
	if got := GlobalMemoryDir(); got != expected {
		t.Errorf("GlobalMemoryDir() = %q, want %q", got, expected)
	}
}

func TestDeleteDefaultFails2(t *testing.T) {
	// Ensure deleting default project fails
	err := DeleteProject("default")
	if err == nil {
		t.Error("deleting default should fail")
	}
}
