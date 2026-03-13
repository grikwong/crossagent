package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grikwong/crossagent/internal/state"
)

// ValidateName validates a workflow or project name.
// Delegates to state.ValidateName for consistency.
func ValidateName(name string) error {
	return state.ValidateName(name)
}

// SanitizeName sanitizes a workflow name to match bash behavior:
// lowercase, replace spaces with hyphens, strip non-allowed characters.
func SanitizeName(name string) string {
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ToLower(name)
	// Strip characters not matching [a-z0-9._-]
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SanitizeAgentName sanitizes an agent name: lowercase, strip non-alphanumeric except .- and _.
func SanitizeAgentName(name string) string {
	return SanitizeName(name)
}

// ValidatePhase validates a phase identifier.
// Must be 1-4 or plan/review/implement/verify.
func ValidatePhase(phase string) error {
	_, err := state.PhaseKey(phase)
	return err
}

// ValidatePath validates that a filesystem path exists and contains no commas (CSV safety).
// Returns the resolved absolute path.
func ValidatePath(path string) (string, error) {
	if strings.Contains(path, ",") {
		return "", fmt.Errorf("path cannot contain commas: %s", path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %s: %w", path, err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("path does not exist: %s", absPath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}

	return absPath, nil
}

// RequireWorkflow returns the current workflow name and directory.
// Delegates to state.RequireWorkflow.
func RequireWorkflow() (string, string, error) {
	return state.RequireWorkflow()
}
