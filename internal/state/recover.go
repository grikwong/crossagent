package state

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// substantiveHeaderRE matches any markdown header at levels 1–4, mirroring
// judge.affectedFilesRE which accepts the same depth range. Keeping the
// two in lockstep ensures a plan.md written with a deeply-nested
// "#### 2. Affected Files" header (the template's default) isn't
// rejected as non-substantive after a sandbox-fallback recovery.
var substantiveHeaderRE = regexp.MustCompile(`(?m)^\s{0,3}#{1,4}\s+\S`)

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

// minSubstantiveSize is the byte threshold below which a recovery
// candidate is treated as a sandbox probe (e.g. gemini writing a
// 4-byte "test" or a placeholder file to its CWD to verify write
// access) rather than a real phase artifact. Real plan/review/verify
// outputs from every adapter consistently exceed 1 KiB; the smallest
// legitimate memory_updates.md we have observed is ~400 B. We pick
// 256 to leave a wide margin for terse-but-valid artifacts while
// still rejecting the probe files that were stalling retry loops.
const minSubstantiveSize = 256

// probeQuarantineSuffix is appended to repo-root candidates that fail
// the substantive-content check. Keeping the file (rather than
// deleting it) preserves evidence for debugging, while the renamed
// form prevents the next RecoverWorkflowOutputs sweep from picking
// the same probe up and re-triggering the loop.
const probeQuarantineSuffix = ".sandbox-probe"

// RecoverMisplacedOutput relocates a single artifact from the repo root
// (agent CWD) into the workflow directory when the workflow copy is
// missing. It is a no-op if the workflow copy already exists, if the
// repo copy is missing, or if either path points outside a regular
// file. The move is atomic via os.Rename on the same filesystem, with
// a copy+remove fallback when Rename crosses filesystem boundaries.
//
// To prevent sandbox probe files from being promoted to canonical
// artifacts (which previously stalled workflows in a plan↔review retry
// loop), candidates that fail looksSubstantive() are quarantined
// next to their source with a `.sandbox-probe` suffix instead of
// being moved into the workflow dir.
//
// Returns:
//   - recovered: true if a substantive file was actually moved.
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

	// Substantive-content gate: reject obvious sandbox probes before
	// they can overwrite the canonical phase artifact slot. The
	// failing candidate is quarantined with a `.sandbox-probe`
	// suffix so (a) the next sweep does not pick it up again and
	// (b) the operator can still inspect what the agent actually
	// wrote. Errors from the rename are surfaced so callers can log
	// them, but the recovery itself is still a no-op either way.
	if !looksSubstantive(candidate, basename) {
		quarantine := candidate + probeQuarantineSuffix
		// Use a timestamp-free suffix so repeated probes collapse
		// onto the same file instead of cluttering the repo root.
		if rerr := os.Rename(candidate, quarantine); rerr != nil {
			return false, "", fmt.Errorf("recover %s: quarantine probe: %w", basename, rerr)
		}
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

// looksSubstantive reports whether path points to a file large and
// structured enough to plausibly be a real phase artifact rather than
// a sandbox probe. Checks (in order, any failure disqualifies):
//
//  1. Size ≥ minSubstantiveSize.
//  2. Content is not whitespace-only.
//  3. For phase outputs (plan/review/implement/verify.md) the body
//     contains at least one markdown section header (`# ` or `## `).
//     memory_updates.md is exempt from the header rule because its
//     structure is agent-specific.
//
// Unreadable files are treated as non-substantive — if we cannot
// inspect the content, we must not promote it.
func looksSubstantive(path, basename string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	if info.Size() < minSubstantiveSize {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	text := string(data)
	if strings.TrimSpace(text) == "" {
		return false
	}
	// Phase outputs must at least look like markdown sections so a
	// block of lorem-ipsum padding cannot pass the size check alone.
	// memory_updates.md has adapter-specific formatting that does
	// not always use `#` headers, so exempt it.
	if basename != "memory_updates.md" {
		if !substantiveHeaderRE.MatchString(text) {
			return false
		}
	}
	return true
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
