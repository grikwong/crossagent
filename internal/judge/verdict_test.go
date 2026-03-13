package judge

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestJudgeVerifyShipIt(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "verify.md", `# Verification Report

## Status
**PASS**

## Plan Drift
None.

## Issues Found
None.

## Positive Notes
Everything looks good.

## Recommendation
**Ship it**
`)

	verdict, status, rec, err := JudgeVerify(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Pass {
		t.Errorf("verdict = %q, want %q", verdict, Pass)
	}
	if status != "PASS" {
		t.Errorf("status = %q, want %q", status, "PASS")
	}
	if rec != "Ship it" {
		t.Errorf("recommendation = %q, want %q", rec, "Ship it")
	}
}

func TestJudgeVerifyFixIssues(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "verify.md", `## Status
**FAIL**

## Recommendation
**Fix issues first**
`)

	verdict, status, rec, err := JudgeVerify(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Fix {
		t.Errorf("verdict = %q, want %q", verdict, Fix)
	}
	if status != "FAIL" {
		t.Errorf("status = %q, want %q", status, "FAIL")
	}
	if rec != "Fix issues first" {
		t.Errorf("recommendation = %q, want %q", rec, "Fix issues first")
	}
}

func TestJudgeVerifyNeedsRework(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "verify.md", `## Status
**FAIL**

## Recommendation
**Needs rework**
`)

	verdict, _, _, err := JudgeVerify(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Rework {
		t.Errorf("verdict = %q, want %q", verdict, Rework)
	}
}

func TestJudgeVerifyPassFromStatus(t *testing.T) {
	dir := t.TempDir()
	// No recommendation section, just status with PASS
	path := writeTestFile(t, dir, "verify.md", `## Status
PASS

## Issues Found
None.
`)

	verdict, _, _, err := JudgeVerify(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Pass {
		t.Errorf("verdict = %q, want %q", verdict, Pass)
	}
}

func TestJudgeVerifyFailFromStatus(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "verify.md", `## Status
FAIL
`)

	verdict, _, _, err := JudgeVerify(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Fix {
		t.Errorf("verdict = %q, want %q", verdict, Fix)
	}
}

func TestJudgeVerifyMissingFile(t *testing.T) {
	verdict, status, rec, err := JudgeVerify("/nonexistent/verify.md")
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Missing {
		t.Errorf("verdict = %q, want %q", verdict, Missing)
	}
	if status != "" {
		t.Errorf("status should be empty, got %q", status)
	}
	if rec != "" {
		t.Errorf("recommendation should be empty, got %q", rec)
	}
}

func TestJudgeVerifyVariousHeadingLevels(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "verify.md", `### Status
**PASS WITH NOTES**

#### Recommendation
Ship it
`)

	verdict, status, _, err := JudgeVerify(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Pass {
		t.Errorf("verdict = %q, want %q", verdict, Pass)
	}
	if status != "PASS WITH NOTES" {
		t.Errorf("status = %q, want %q", status, "PASS WITH NOTES")
	}
}

func TestJudgeVerifyCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "verify.md", `## Status
pass

## Recommendation
SHIP IT
`)

	verdict, _, _, err := JudgeVerify(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Pass {
		t.Errorf("verdict = %q, want %q", verdict, Pass)
	}
}

func TestJudgeReviewApprove(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "review.md", `# Plan Review

## Issues
None.

## Verdict
**APPROVE**
`)

	verdict, raw, err := JudgeReview(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Approve {
		t.Errorf("verdict = %q, want %q", verdict, Approve)
	}
	if raw != "APPROVE" {
		t.Errorf("raw = %q, want %q", raw, "APPROVE")
	}
}

func TestJudgeReviewApproveWithChanges(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "review.md", `## Verdict

**APPROVE WITH CHANGES**
`)

	verdict, _, err := JudgeReview(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != ApproveWithChanges {
		t.Errorf("verdict = %q, want %q", verdict, ApproveWithChanges)
	}
}

func TestJudgeReviewRequestRework(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "review.md", `## Verdict

**REQUEST REWORK**
`)

	verdict, _, err := JudgeReview(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Rework {
		t.Errorf("verdict = %q, want %q", verdict, Rework)
	}
}

func TestJudgeReviewMissingFile(t *testing.T) {
	verdict, raw, err := JudgeReview("/nonexistent/review.md")
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Missing {
		t.Errorf("verdict = %q, want %q", verdict, Missing)
	}
	if raw != "" {
		t.Errorf("raw should be empty, got %q", raw)
	}
}

func TestJudgeReviewUnknown(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "review.md", `## Verdict

Something unexpected
`)

	verdict, _, err := JudgeReview(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != Unknown {
		t.Errorf("verdict = %q, want %q", verdict, Unknown)
	}
}

func TestJudgeReviewVariousHeadingLevels(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "review.md", `#### Verdict
APPROVE WITH CHANGES
`)

	verdict, _, err := JudgeReview(path)
	if err != nil {
		t.Fatal(err)
	}
	if verdict != ApproveWithChanges {
		t.Errorf("verdict = %q, want %q", verdict, ApproveWithChanges)
	}
}
