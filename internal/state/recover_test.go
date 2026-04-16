package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// substantivePlan returns content that passes looksSubstantive:
// a markdown header plus enough padding to clear minSubstantiveSize.
func substantivePlan() []byte {
	return []byte("# plan\n\n" + strings.Repeat("real planning content.\n", 20))
}

func TestRecoverMisplacedOutput_MovesFromRepoRoot(t *testing.T) {
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "wf")
	repo := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(wfDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(repo, "plan.md")
	want := substantivePlan()
	if err := os.WriteFile(src, want, 0644); err != nil {
		t.Fatal(err)
	}

	moved, srcPath, err := RecoverMisplacedOutput(wfDir, repo, "plan.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !moved {
		t.Fatal("expected recovered=true")
	}
	if srcPath != src {
		t.Errorf("srcPath = %q, want %q", srcPath, src)
	}

	// Source is gone.
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source should have been removed, got err=%v", err)
	}

	// Target exists with original contents.
	dst := filepath.Join(wfDir, "plan.md")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("target missing: %v", err)
	}
	if string(data) != string(want) {
		t.Errorf("target contents = %q, want original", data)
	}
}

func TestRecoverMisplacedOutput_NoOpWhenTargetExists(t *testing.T) {
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "wf")
	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(wfDir, 0755)
	os.MkdirAll(repo, 0755)

	existing := filepath.Join(wfDir, "plan.md")
	misplaced := filepath.Join(repo, "plan.md")
	os.WriteFile(existing, []byte("authoritative"), 0644)
	os.WriteFile(misplaced, []byte("SHOULD NOT OVERWRITE"), 0644)

	moved, _, err := RecoverMisplacedOutput(wfDir, repo, "plan.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved {
		t.Error("expected no-op when target already exists")
	}

	data, _ := os.ReadFile(existing)
	if string(data) != "authoritative" {
		t.Errorf("target was clobbered: %q", data)
	}
	// Misplaced file left alone for operator inspection — recovery must
	// not silently delete data when no recovery happened.
	if _, err := os.Stat(misplaced); err != nil {
		t.Errorf("misplaced file should remain when target exists: %v", err)
	}
}

func TestRecoverMisplacedOutput_NoOpWhenNothingToRecover(t *testing.T) {
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "wf")
	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(wfDir, 0755)
	os.MkdirAll(repo, 0755)

	moved, src, err := RecoverMisplacedOutput(wfDir, repo, "plan.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved || src != "" {
		t.Errorf("expected (false, \"\"), got (%v, %q)", moved, src)
	}
}

func TestRecoverMisplacedOutput_EmptyInputs(t *testing.T) {
	// Guards against accidental recovery when callers can't supply a
	// workflow dir or repo path — we should silently decline rather
	// than touching the filesystem.
	if moved, _, err := RecoverMisplacedOutput("", "/tmp", "plan.md"); moved || err != nil {
		t.Errorf("empty wfDir: want no-op, got moved=%v err=%v", moved, err)
	}
	if moved, _, err := RecoverMisplacedOutput("/tmp", "", "plan.md"); moved || err != nil {
		t.Errorf("empty repo: want no-op, got moved=%v err=%v", moved, err)
	}
	if moved, _, err := RecoverMisplacedOutput("/tmp", "/tmp", ""); moved || err != nil {
		t.Errorf("empty basename: want no-op, got moved=%v err=%v", moved, err)
	}
}

func TestRecoverMisplacedOutput_SamePathNoOp(t *testing.T) {
	// When wfDir and repo are the same directory (unusual but possible
	// when a user points the workflow at the repo root), we must not
	// move a file onto itself.
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "plan.md"), []byte("x"), 0644)
	// Expected and candidate resolve to the same path → target exists,
	// which short-circuits before the self-move guard. This test pins
	// that invariant: no error, no-op, file preserved.
	moved, _, err := RecoverMisplacedOutput(tmp, tmp, "plan.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved {
		t.Error("expected no-op when wfDir == repo")
	}
	if _, err := os.Stat(filepath.Join(tmp, "plan.md")); err != nil {
		t.Errorf("file should still exist: %v", err)
	}
}

func TestRecoverWorkflowOutputs_SweepsAllArtifacts(t *testing.T) {
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "wf")
	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(wfDir, 0755)
	os.MkdirAll(repo, 0755)

	// Misplace three of the four phase outputs plus memory_updates.md
	// and leave one (verify.md) entirely absent. Each misplaced file
	// must be substantive for the recovery to succeed.
	for _, name := range []string{"plan.md", "review.md", "implement.md"} {
		os.WriteFile(filepath.Join(repo, name), substantivePlan(), 0644)
	}
	os.WriteFile(filepath.Join(repo, "memory_updates.md"),
		[]byte(strings.Repeat("- memory update entry\n", 20)), 0644)

	recoveredFrom, err := RecoverWorkflowOutputs(wfDir, repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recoveredFrom) != 4 {
		t.Fatalf("recoveredFrom = %d, want 4 (got %v)", len(recoveredFrom), recoveredFrom)
	}

	for _, name := range []string{"plan.md", "review.md", "implement.md", "memory_updates.md"} {
		if _, err := os.Stat(filepath.Join(wfDir, name)); err != nil {
			t.Errorf("expected %s in wfDir: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(repo, name)); !os.IsNotExist(err) {
			t.Errorf("expected %s removed from repo, err=%v", name, err)
		}
	}
	// verify.md was never produced — it should not magically appear.
	if _, err := os.Stat(filepath.Join(wfDir, "verify.md")); !os.IsNotExist(err) {
		t.Errorf("verify.md should not exist, err=%v", err)
	}
}

// Regression guard for the crossagent-meta-orchestrator retry-loop
// bug: a 4-byte "test" file written by gemini's sandbox probe was
// being promoted to the canonical plan.md slot, causing the reviewer
// to reject every retry with REQUEST REWORK.
func TestRecoverMisplacedOutput_RejectsTinyProbe(t *testing.T) {
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "wf")
	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(wfDir, 0755)
	os.MkdirAll(repo, 0755)

	probe := filepath.Join(repo, "plan.md")
	os.WriteFile(probe, []byte("test"), 0644)

	moved, src, err := RecoverMisplacedOutput(wfDir, repo, "plan.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved || src != "" {
		t.Errorf("expected probe to be rejected, got moved=%v src=%q", moved, src)
	}
	if _, err := os.Stat(filepath.Join(wfDir, "plan.md")); !os.IsNotExist(err) {
		t.Errorf("workflow dir must not contain the probe, err=%v", err)
	}
	// Probe must be quarantined (renamed) so the next sweep does not
	// re-pick it up. The original location must be empty.
	if _, err := os.Stat(probe); !os.IsNotExist(err) {
		t.Errorf("probe source should be renamed away, err=%v", err)
	}
	quarantined := probe + probeQuarantineSuffix
	data, err := os.ReadFile(quarantined)
	if err != nil {
		t.Fatalf("expected quarantined probe at %s: %v", quarantined, err)
	}
	if string(data) != "test" {
		t.Errorf("quarantined content = %q, want preserved", data)
	}
}

func TestRecoverMisplacedOutput_RejectsHeaderlessLargeFile(t *testing.T) {
	// Even a large file is rejected if it lacks any markdown header —
	// this catches the "# test write to project root" style probe
	// once agents start padding their probes. memory_updates.md is
	// exempt (see looksSubstantive); phase outputs are not.
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "wf")
	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(wfDir, 0755)
	os.MkdirAll(repo, 0755)

	os.WriteFile(filepath.Join(repo, "plan.md"),
		[]byte(strings.Repeat("no header here, just filler bytes. ", 20)), 0644)

	moved, _, err := RecoverMisplacedOutput(wfDir, repo, "plan.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if moved {
		t.Error("expected headerless file to be rejected")
	}
}

func TestRecoverMisplacedOutput_AcceptsSubstantiveReview(t *testing.T) {
	tmp := t.TempDir()
	wfDir := filepath.Join(tmp, "wf")
	repo := filepath.Join(tmp, "repo")
	os.MkdirAll(wfDir, 0755)
	os.MkdirAll(repo, 0755)

	body := []byte("## Review\n\n" + strings.Repeat("Finding: something worth noting.\n", 20))
	os.WriteFile(filepath.Join(repo, "review.md"), body, 0644)

	moved, _, err := RecoverMisplacedOutput(wfDir, repo, "review.md")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !moved {
		t.Fatal("expected substantive review.md to be recovered")
	}
}

func TestLooksSubstantive_MemoryUpdatesExemptFromHeader(t *testing.T) {
	// memory_updates.md may use adapter-specific formatting without
	// a top-level `#` header; the size gate alone must be sufficient.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "memory_updates.md")
	os.WriteFile(path, []byte(strings.Repeat("- entry\n", 40)), 0644)
	if !looksSubstantive(path, "memory_updates.md") {
		t.Error("memory_updates.md with no header should still be substantive when large enough")
	}
}
