package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withTestHome(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	origHome := os.Getenv("CROSSAGENT_HOME")
	os.Setenv("CROSSAGENT_HOME", dir)
	return dir, func() {
		if origHome == "" {
			os.Unsetenv("CROSSAGENT_HOME")
		} else {
			os.Setenv("CROSSAGENT_HOME", origHome)
		}
	}
}

func TestHome(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("CROSSAGENT_HOME")
	os.Setenv("CROSSAGENT_HOME", dir)
	defer func() {
		if origHome == "" {
			os.Unsetenv("CROSSAGENT_HOME")
		} else {
			os.Setenv("CROSSAGENT_HOME", origHome)
		}
	}()

	if got := Home(); got != dir {
		t.Errorf("Home() = %q, want %q", got, dir)
	}
}

func TestGetSetPhase(t *testing.T) {
	dir := t.TempDir()

	// No phase file -> "0"
	phase, err := GetPhase(dir)
	if err != nil {
		t.Fatal(err)
	}
	if phase != "0" {
		t.Errorf("expected '0', got %q", phase)
	}

	// Set and read back
	if err := SetPhase(dir, "2"); err != nil {
		t.Fatal(err)
	}
	phase, err = GetPhase(dir)
	if err != nil {
		t.Fatal(err)
	}
	if phase != "2" {
		t.Errorf("expected '2', got %q", phase)
	}

	// Set done
	if err := SetPhase(dir, "done"); err != nil {
		t.Fatal(err)
	}
	phase, _ = GetPhase(dir)
	if phase != "done" {
		t.Errorf("expected 'done', got %q", phase)
	}
}

func TestGetDescription(t *testing.T) {
	dir := t.TempDir()

	// No file -> empty
	desc, err := GetDescription(dir)
	if err != nil {
		t.Fatal(err)
	}
	if desc != "" {
		t.Errorf("expected empty, got %q", desc)
	}

	// Write and read
	os.WriteFile(filepath.Join(dir, "description"), []byte("My feature\nSecond line"), 0644)
	desc, _ = GetDescription(dir)
	if desc != "My feature\nSecond line" {
		t.Errorf("unexpected description: %q", desc)
	}
}

func TestGetSetCurrent(t *testing.T) {
	_, cleanup := withTestHome(t)
	defer cleanup()

	if err := os.MkdirAll(Home(), 0755); err != nil {
		t.Fatal(err)
	}

	// No current -> empty
	name, err := GetCurrent()
	if err != nil {
		t.Fatal(err)
	}
	if name != "" {
		t.Errorf("expected empty, got %q", name)
	}

	// Set and read
	if err := SetCurrent("my-workflow"); err != nil {
		t.Fatal(err)
	}
	name, _ = GetCurrent()
	if name != "my-workflow" {
		t.Errorf("expected 'my-workflow', got %q", name)
	}
}

func TestListWorkflows(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	wfDir := filepath.Join(home, "workflows")
	os.MkdirAll(filepath.Join(wfDir, "wf-a"), 0755)
	os.MkdirAll(filepath.Join(wfDir, "wf-b"), 0755)

	names, err := ListWorkflows()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 workflows, got %d", len(names))
	}
}

func TestWorkflowExists(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(home, "workflows", "exists"), 0755)

	if !WorkflowExists("exists") {
		t.Error("expected workflow to exist")
	}
	if WorkflowExists("nonexistent") {
		t.Error("expected workflow to not exist")
	}
}

func TestEnsureDirs(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	// Override time for deterministic output
	oldTime := currentTime
	currentTime = func() time.Time { return time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC) }
	defer func() { currentTime = oldTime }()

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Check directories exist
	for _, d := range []string{"workflows", "agents", "memory", "projects/default/memory"} {
		path := filepath.Join(home, d)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Errorf("directory %s should exist", d)
		}
	}

	// Check global memory files
	if _, err := os.Stat(filepath.Join(home, "memory", "global-context.md")); err != nil {
		t.Error("global-context.md should exist")
	}
	if _, err := os.Stat(filepath.Join(home, "memory", "lessons-learned.md")); err != nil {
		t.Error("lessons-learned.md should exist")
	}

	// Check default project memory
	if _, err := os.Stat(filepath.Join(home, "projects", "default", "memory", "project-context.md")); err != nil {
		t.Error("default project-context.md should exist")
	}
}

func TestEnsureDirsLegacyMigration(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	// Create legacy features directory
	legacyFeatures := filepath.Join(home, "memory", "features")
	os.MkdirAll(legacyFeatures, 0755)
	os.WriteFile(filepath.Join(legacyFeatures, "feature1.md"), []byte("# Feature 1"), 0644)

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs with migration: %v", err)
	}

	// Features should be migrated
	dest := filepath.Join(home, "projects", "default", "memory", "features", "feature1.md")
	if _, err := os.Stat(dest); err != nil {
		t.Error("feature1.md should be migrated to default project")
	}

	// Source should be gone
	if _, err := os.Stat(legacyFeatures); !os.IsNotExist(err) {
		t.Error("legacy features dir should be removed after migration")
	}

	// Migration note should exist
	note := filepath.Join(home, "memory", "features-migrated.txt")
	if _, err := os.Stat(note); err != nil {
		t.Error("features-migrated.txt should exist")
	}
}

func TestEnsureDirsBackfillProject(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	// Create a workflow without a project key
	wfDir := filepath.Join(home, "workflows", "old-wf")
	os.MkdirAll(wfDir, 0755)
	os.WriteFile(filepath.Join(wfDir, "config"), []byte("repo=/tmp/test\n"), 0644)

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Should have backfilled project=default
	proj, _ := GetConf(wfDir, "project")
	if proj != "default" {
		t.Errorf("expected project=default, got %q", proj)
	}
}

func TestPhaseLabel(t *testing.T) {
	tests := map[string]string{
		"1": "plan", "2": "review", "3": "implement", "4": "verify", "done": "done",
	}
	for phase, want := range tests {
		if got := PhaseLabel(phase); got != want {
			t.Errorf("PhaseLabel(%q) = %q, want %q", phase, got, want)
		}
	}
}

func TestPhaseOutputFile(t *testing.T) {
	tests := map[string]string{
		"1": "plan.md", "2": "review.md", "3": "implement.md", "4": "verify.md", "done": "",
	}
	for phase, want := range tests {
		if got := PhaseOutputFile(phase); got != want {
			t.Errorf("PhaseOutputFile(%q) = %q, want %q", phase, got, want)
		}
	}
}

func TestPhaseNum(t *testing.T) {
	if got := PhaseNum("done"); got != 5 {
		t.Errorf("PhaseNum('done') = %d, want 5", got)
	}
	if got := PhaseNum("3"); got != 3 {
		t.Errorf("PhaseNum('3') = %d, want 3", got)
	}
}

func TestPhaseKey(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  bool
	}{
		{"1", "plan", false},
		{"plan", "plan", false},
		{"3", "implement", false},
		{"impl", "implement", false},
		{"bad", "", true},
	}
	for _, tt := range tests {
		got, err := PhaseKey(tt.in)
		if tt.err && err == nil {
			t.Errorf("PhaseKey(%q) expected error", tt.in)
		}
		if !tt.err && got != tt.want {
			t.Errorf("PhaseKey(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPhaseID(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  bool
	}{
		{"1", "1", false},
		{"plan", "1", false},
		{"review", "2", false},
		{"implement", "3", false},
		{"impl", "3", false},
		{"verify", "4", false},
		{"bad", "", true},
	}
	for _, tt := range tests {
		got, err := PhaseID(tt.in)
		if tt.err && err == nil {
			t.Errorf("PhaseID(%q) expected error", tt.in)
		}
		if !tt.err && got != tt.want {
			t.Errorf("PhaseID(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCreateWorkflow(t *testing.T) {
	_, cleanup := withTestHome(t)
	defer cleanup()

	oldTime := currentTime
	currentTime = func() time.Time { return time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC) }
	defer func() { currentTime = oldTime }()

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	// Create a temp repo dir
	repoDir := t.TempDir()

	err := CreateWorkflow("test-wf", repoDir, "default", "Test description", nil)
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	// Verify workflow exists
	if !WorkflowExists("test-wf") {
		t.Error("workflow should exist after creation")
	}

	// Verify current is set
	cur, _ := GetCurrent()
	if cur != "test-wf" {
		t.Errorf("expected current='test-wf', got %q", cur)
	}

	// Verify phase is 1
	d := WorkflowDir("test-wf")
	phase, _ := GetPhase(d)
	if phase != "1" {
		t.Errorf("expected phase='1', got %q", phase)
	}

	// Verify description
	desc, _ := GetDescription(d)
	if desc != "Test description" {
		t.Errorf("expected description='Test description', got %q", desc)
	}

	// Verify config
	cfg, _ := ReadConfig(d)
	if cfg.Repo != repoDir {
		t.Errorf("expected repo=%q, got %q", repoDir, cfg.Repo)
	}
	if cfg.Project != "default" {
		t.Errorf("expected project='default', got %q", cfg.Project)
	}

	// Verify memory file exists
	if _, err := os.Stat(filepath.Join(d, "memory.md")); err != nil {
		t.Error("memory.md should exist")
	}

	// Verify duplicate creation fails
	err = CreateWorkflow("test-wf", repoDir, "default", "Dup", nil)
	if err == nil {
		t.Error("expected error for duplicate workflow")
	}

	// Verify bad project fails
	err = CreateWorkflow("test-wf-2", repoDir, "nonexistent", "Test", nil)
	if err == nil {
		t.Error("expected error for nonexistent project")
	}
}

func TestCreateWorkflowWithAddDirs(t *testing.T) {
	_, cleanup := withTestHome(t)
	defer cleanup()

	oldTime := currentTime
	currentTime = func() time.Time { return time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC) }
	defer func() { currentTime = oldTime }()

	if err := EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	repoDir := t.TempDir()
	addDir1 := t.TempDir()
	addDir2 := t.TempDir()

	err := CreateWorkflow("test-dirs", repoDir, "", "With dirs", []string{addDir1, addDir2})
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}

	d := WorkflowDir("test-dirs")
	cfg, _ := ReadConfig(d)
	if len(cfg.AddDirs) != 2 {
		t.Errorf("expected 2 add_dirs, got %d", len(cfg.AddDirs))
	}
}

func TestGlobSort(t *testing.T) {
	// Verify GlobSort matches macOS bash glob ordering where hyphens sort
	// before end-of-string (opposite of byte comparison).
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			name:  "hyphen prefix",
			input: []string{"a", "a-b"},
			want:  []string{"a-b", "a"},
		},
		{
			name:  "real workflow names",
			input: []string{"crossagent-bash-golang-rewrite", "crossagent-bash-golang-rewrite-phase2", "crossagent-bash-golang-rewrite-phase3"},
			want:  []string{"crossagent-bash-golang-rewrite-phase2", "crossagent-bash-golang-rewrite-phase3", "crossagent-bash-golang-rewrite"},
		},
		{
			name:  "migration names",
			input: []string{"esign-migration", "esign-migration-2"},
			want:  []string{"esign-migration-2", "esign-migration"},
		},
		{
			name:  "no hyphens",
			input: []string{"c", "a", "b"},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "identical prefixes letters",
			input: []string{"ab", "a"},
			want:  []string{"a", "ab"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]string, len(tt.input))
			copy(got, tt.input)
			GlobSort(got)
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("GlobSort(%v) = %v, want %v", tt.input, got, tt.want)
					break
				}
			}
		})
	}
}
