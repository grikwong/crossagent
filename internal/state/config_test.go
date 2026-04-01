package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetConfSetConf(t *testing.T) {
	dir := t.TempDir()

	// GetConf on non-existent file returns empty
	val, err := GetConf(dir, "repo")
	if err != nil {
		t.Fatalf("GetConf on missing file: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty, got %q", val)
	}

	// SetConf creates the file
	if err := SetConf(dir, "repo", "/tmp/test"); err != nil {
		t.Fatalf("SetConf: %v", err)
	}

	val, err = GetConf(dir, "repo")
	if err != nil {
		t.Fatalf("GetConf: %v", err)
	}
	if val != "/tmp/test" {
		t.Fatalf("expected /tmp/test, got %q", val)
	}

	// SetConf updates existing key
	if err := SetConf(dir, "repo", "/tmp/new"); err != nil {
		t.Fatalf("SetConf update: %v", err)
	}
	val, _ = GetConf(dir, "repo")
	if val != "/tmp/new" {
		t.Fatalf("expected /tmp/new, got %q", val)
	}

	// Multiple keys
	if err := SetConf(dir, "project", "default"); err != nil {
		t.Fatalf("SetConf project: %v", err)
	}
	if err := SetConf(dir, "created", "2026-03-13"); err != nil {
		t.Fatalf("SetConf created: %v", err)
	}

	val, _ = GetConf(dir, "project")
	if val != "default" {
		t.Fatalf("expected default, got %q", val)
	}
	val, _ = GetConf(dir, "repo")
	if val != "/tmp/new" {
		t.Fatalf("repo should be preserved, got %q", val)
	}
}

func TestReadWriteConfig(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Repo:    "/tmp/test-repo",
		AddDirs: []string{"/dir1", "/dir2"},
		Created: "2026-03-13",
		Project: "default",
	}

	if err := WriteConfig(dir, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	got, err := ReadConfig(dir)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if got.Repo != cfg.Repo {
		t.Errorf("Repo: got %q, want %q", got.Repo, cfg.Repo)
	}
	if len(got.AddDirs) != 2 || got.AddDirs[0] != "/dir1" || got.AddDirs[1] != "/dir2" {
		t.Errorf("AddDirs: got %v, want %v", got.AddDirs, cfg.AddDirs)
	}
	if got.Created != cfg.Created {
		t.Errorf("Created: got %q, want %q", got.Created, cfg.Created)
	}
	if got.Project != cfg.Project {
		t.Errorf("Project: got %q, want %q", got.Project, cfg.Project)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()

	cfg := &Config{
		Repo:           "/tmp/repo",
		AddDirs:        []string{"/a", "/b", "/c"},
		Created:        "2026-01-01",
		Project:        "myproject",
		PlanAgent:      "claude",
		ReviewAgent:    "codex",
		ImplementAgent: "claude",
		VerifyAgent:    "codex",
		RetryCount:     2,
		MaxRetries:     3,
	}

	if err := WriteConfig(dir, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	got, err := ReadConfig(dir)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if got.PlanAgent != "claude" {
		t.Errorf("PlanAgent: got %q, want %q", got.PlanAgent, "claude")
	}
	if got.ReviewAgent != "codex" {
		t.Errorf("ReviewAgent: got %q, want %q", got.ReviewAgent, "codex")
	}
	if got.RetryCount != 2 {
		t.Errorf("RetryCount: got %d, want 2", got.RetryCount)
	}
	if got.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", got.MaxRetries)
	}
}

func TestConfigFollowupRoundRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// FollowupRound > 0 round-trips
	cfg := &Config{
		Repo:          "/tmp/repo",
		Created:       "2026-04-02",
		Project:       "default",
		FollowupRound: 2,
	}
	if err := WriteConfig(dir, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	got, err := ReadConfig(dir)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if got.FollowupRound != 2 {
		t.Errorf("FollowupRound: got %d, want 2", got.FollowupRound)
	}

	// Verify followup_round appears in file
	data, _ := os.ReadFile(filepath.Join(dir, "config"))
	if !strings.Contains(string(data), "followup_round=2") {
		t.Errorf("config file should contain followup_round=2, got:\n%s", data)
	}

	// FollowupRound == 0 is omitted from file
	cfg.FollowupRound = 0
	if err := WriteConfig(dir, cfg); err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}
	data, _ = os.ReadFile(filepath.Join(dir, "config"))
	if strings.Contains(string(data), "followup_round") {
		t.Errorf("config file should not contain followup_round when 0, got:\n%s", data)
	}
	got, _ = ReadConfig(dir)
	if got.FollowupRound != 0 {
		t.Errorf("FollowupRound: got %d, want 0", got.FollowupRound)
	}
}

func TestGetConfMissingKey(t *testing.T) {
	dir := t.TempDir()

	// Write a config with one key
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("repo=/tmp/x\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Missing key returns empty (matches bash behavior)
	val, err := GetConf(dir, "nonexistent")
	if err != nil {
		t.Fatalf("GetConf missing key: %v", err)
	}
	if val != "" {
		t.Fatalf("expected empty for missing key, got %q", val)
	}
}

func TestSetConfPreservesOrder(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config")

	// Write initial config
	initial := "repo=/tmp/a\nproject=default\ncreated=2026-01-01\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0644); err != nil {
		t.Fatal(err)
	}

	// Update middle key
	if err := SetConf(dir, "project", "newproject"); err != nil {
		t.Fatalf("SetConf: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	expected := "repo=/tmp/a\nproject=newproject\ncreated=2026-01-01\n"
	if string(data) != expected {
		t.Errorf("config order not preserved:\ngot:  %q\nwant: %q", string(data), expected)
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		in  string
		out int
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"", 0},
		{"abc", 0},
		{"3x", 3},
	}
	for _, tt := range tests {
		if got := atoi(tt.in); got != tt.out {
			t.Errorf("atoi(%q) = %d, want %d", tt.in, got, tt.out)
		}
	}
}
