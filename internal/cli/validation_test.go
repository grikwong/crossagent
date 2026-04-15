package cli

import (
	"os"
	"strings"
	"testing"
)

func TestValidatePathRejectsDoubleQuote(t *testing.T) {
	// Double quotes would break the codex trust-override TOML key.
	_, err := ValidatePath(`/tmp/weird"dir`)
	if err == nil {
		t.Fatal("expected error for path containing double quote")
	}
	if !strings.Contains(err.Error(), "double-quote") && !strings.Contains(err.Error(), "backslash") {
		t.Errorf("error should mention double-quote/backslash, got: %v", err)
	}
}

func TestValidatePathRejectsBackslash(t *testing.T) {
	// Backslashes would also break the codex trust-override TOML key escape.
	_, err := ValidatePath(`/tmp/weird\dir`)
	if err == nil {
		t.Fatal("expected error for path containing backslash")
	}
}

func TestValidatePathRejectsComma(t *testing.T) {
	_, err := ValidatePath("/tmp/a,b")
	if err == nil {
		t.Fatal("expected error for path containing comma")
	}
}

func TestValidatePathAcceptsSpaces(t *testing.T) {
	// Paths with spaces are fine: codexTrustArgs %q-escapes them into a
	// valid TOML key segment.
	dir := t.TempDir()
	target := dir + "/my dir"
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatal(err)
	}
	got, err := ValidatePath(target)
	if err != nil {
		t.Fatalf("expected success for path with space, got: %v", err)
	}
	if got != target {
		t.Errorf("got %q, want %q", got, target)
	}
}
