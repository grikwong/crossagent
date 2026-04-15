package state

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// RecoverableArtifacts is the ordered list of filenames that an agent is
// expected to produce inside the workflow directory but may instead emit
// into the repo (agent CWD) when an OS-level sandbox blocks writes outside
// the working tree.
//
// Phase outputs (plan/review/implement/verify.md) are recovered back into
// the workflow dir so downstream phase-advancement logic finds them.
// memory_updates.md is a gemini-specific fallback file: when the agent
// can't patch memory in place it stages pending changes into that file
// so the user or the next agent can apply them. We relocate it into the
// workflow dir so it is preserved and discoverable rather than leaking
// into the user's repo.
var RecoverableArtifacts = []string{
	"plan.md",
	"review.md",
	"implement.md",
	"verify.md",
	"memory_updates.md",
}

// RecoverMisplacedOutput relocates a single artifact from the repo root
// (agent CWD) into the workflow directory when the workflow copy is
// missing. It is a no-op if the workflow copy already exists, if the
// repo copy is missing, or if either path points outside a regular
// file. The move is atomic via os.Rename on the same filesystem, with
// a copy+remove fallback when Rename crosses filesystem boundaries.
//
// Returns:
//   - recovered: true if a file was actually moved.
//   - srcPath:   the location the file was recovered from (empty when
//     no recovery happened).
func RecoverMisplacedOutput(wfDir, repo, basename string) (recovered bool, srcPath string, err error) {
	if wfDir == "" || repo == "" || basename == "" {
		return false, "", nil
	}

	expected := filepath.Join(wfDir, basename)
	if fileExists(expected) {
		return false, "", nil
	}

	candidate := filepath.Join(repo, basename)
	if !fileExists(candidate) {
		return false, "", nil
	}

	// Guard against the degenerate case where the workflow directory is
	// nested inside the repo (or vice versa) and the two paths resolve
	// to the same file. Comparing absolute paths is defensive — even
	// though filepath.Join normalizes, symlinks could collapse them.
	absExpected, err1 := filepath.Abs(expected)
	absCandidate, err2 := filepath.Abs(candidate)
	if err1 == nil && err2 == nil && absExpected == absCandidate {
		return false, "", nil
	}

	if err := os.MkdirAll(filepath.Dir(expected), 0755); err != nil {
		return false, "", fmt.Errorf("recover %s: mkdir target: %w", basename, err)
	}

	if err := os.Rename(candidate, expected); err == nil {
		return true, candidate, nil
	}

	// Cross-device or permission edge case — fall back to copy+remove.
	if err := copyFile(candidate, expected); err != nil {
		return false, "", fmt.Errorf("recover %s: copy: %w", basename, err)
	}
	if err := os.Remove(candidate); err != nil {
		os.Remove(expected) // rollback the copy
		return false, "", fmt.Errorf("recover %s: remove source: %w", basename, err)
	}
	return true, candidate, nil
}

// RecoverWorkflowOutputs sweeps the known recoverable artifacts and
// relocates each one that is missing from the workflow dir but present
// in the repo root. Returns the source paths of every recovery that
// occurred, in the order the artifacts were checked.
//
// Errors are aggregated using errors.Join so a failure on one artifact does
// not hide failures on others.
func RecoverWorkflowOutputs(wfDir, repo string) (recoveredFrom []string, err error) {
	for _, name := range RecoverableArtifacts {
		moved, src, rerr := RecoverMisplacedOutput(wfDir, repo, name)
		if moved {
			recoveredFrom = append(recoveredFrom, src)
		}
		if rerr != nil {
			err = errors.Join(err, rerr)
		}
	}
	return recoveredFrom, err
}

// copyFile is a small helper used by RecoverMisplacedOutput's
// cross-filesystem fallback. It copies regular file contents and
// preserves mode bits; it does not attempt to preserve ownership or
// timestamps because these artifacts are regenerated on every run.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dst)
		return err
	}
	return out.Close()
}
