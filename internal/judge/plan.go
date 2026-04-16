package judge

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var affectedFilesRE = regexp.MustCompile(`(?i)^#{1,4}\s+.*Affected\s+Files`)

// listItemRE captures the first non-backticked token following a list
// marker ("-" or "*"). The caller further filters the token to only
// keep values that look path-like (contain "/" or a file extension),
// which keeps prose bullets from being mistaken for paths.
var listItemRE = regexp.MustCompile(`^\s*[-*]\s+([^\s` + "`" + `]+)`)

// pathishRE recognizes tokens that plausibly name a file: either they
// contain a path separator, or they carry a short file-extension suffix.
var pathishRE = regexp.MustCompile(`(/|\.[A-Za-z0-9]{1,6}$)`)

// ExtractAffectedFiles parses planFile's "Affected Files" section and
// returns the list of files the agent is allowed to edit, canonicalized
// as cleaned absolute paths confined to repo. Paths that escape the
// repo (via "..", absolute paths outside repo, symlink-style traversal)
// are dropped — this is the single canonicalization boundary so every
// downstream consumer (LaunchContext.AffectedFiles, adapter sandbox
// settings) can treat entries as already-sanitized absolute paths.
//
// Both backticked paths (`pkg/foo.go`) and plain list entries
// (- pkg/foo.go) are extracted. Prose labels ("file", "path", …) and
// tokens containing spaces are rejected.
//
// A non-nil error indicates an I/O or scanner failure, never a plan
// content issue: malformed or missing sections just yield a nil slice.
func ExtractAffectedFiles(planFile, repo string) ([]string, error) {
	res, err := ExtractAffectedFilesDetailed(planFile, repo)
	if err != nil {
		return nil, err
	}
	return res.ValidPaths, nil
}

// ExtractionResult captures a content-aware view of a plan-style file's
// "Affected Files" section. It distinguishes between:
//
//   - FileExists=false: the plan file is absent (os.IsNotExist).
//   - SectionPresent=false: the file exists but no "Affected Files"
//     header was found.
//   - SectionPresent=true, ValidPaths empty, InvalidEntries empty: an
//     empty but well-formed section.
//   - SectionPresent=true, ValidPaths empty, InvalidEntries non-empty:
//     the section contained bullets that looked like file entries but
//     none survived validation (malformed section).
//   - SectionPresent=true, ValidPaths non-empty: healthy extraction.
//
// Consumers that care only about the validated paths can keep using
// ExtractAffectedFiles; launch-path observability layers classify a
// richer ExtractionStatus from this diagnostic result.
type ExtractionResult struct {
	FileExists     bool
	SectionPresent bool
	ValidPaths     []string
	InvalidEntries []string
}

// ExtractAffectedFilesDetailed is the content-aware counterpart to
// ExtractAffectedFiles. It returns the same valid paths plus the
// diagnostic metadata needed to tell "section missing" apart from
// "section present but unusable", so the agent launcher can report
// missing / empty / malformed status instead of a single nil slice.
//
// A non-nil error indicates an I/O or scanner failure only; a missing
// file is reported via FileExists=false with a nil error (mirroring
// ExtractAffectedFiles' legacy behavior for absent plan.md).
func ExtractAffectedFilesDetailed(planFile, repo string) (ExtractionResult, error) {
	f, err := os.Open(planFile)
	if err != nil {
		if os.IsNotExist(err) {
			return ExtractionResult{}, nil
		}
		return ExtractionResult{}, err
	}
	defer f.Close()

	absRepo := repo
	if a, err := filepath.Abs(repo); err == nil {
		absRepo = a
	}

	res := ExtractionResult{FileExists: true}
	var files []string
	var invalid []string
	inAffectedSection := false
	backtickRE := regexp.MustCompile("`([^`]+)`")

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if affectedFilesRE.MatchString(line) {
			inAffectedSection = true
			res.SectionPresent = true
			continue
		}

		if inAffectedSection && strings.HasPrefix(line, "#") {
			inAffectedSection = false
		}

		if !inAffectedSection {
			continue
		}

		backtickHits := backtickRE.FindAllStringSubmatch(line, -1)
		if len(backtickHits) > 0 {
			for _, m := range backtickHits {
				tok := strings.TrimSpace(m[1])
				if canon, ok := ValidatePath(absRepo, tok); ok {
					files = append(files, canon)
				} else {
					invalid = append(invalid, tok)
				}
			}
			continue
		}

		// No backticks on this line — try a conservative list-item
		// match scoped to the Affected Files section only. Trailing
		// sentence punctuation (":", ",", ";") is stripped so bullets
		// like "- launcher.go: update launch logic" still parse.
		if m := listItemRE.FindStringSubmatch(line); m != nil {
			tok := strings.TrimRight(strings.TrimSpace(m[1]), ":,;")
			if !pathishRE.MatchString(tok) {
				continue
			}
			if canon, ok := ValidatePath(absRepo, tok); ok {
				files = append(files, canon)
			} else {
				invalid = append(invalid, tok)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return res, err
	}

	res.ValidPaths = dedupe(files)
	res.InvalidEntries = invalid
	return res, nil
}

// ValidatePath cleans candidate and returns (absolutePath, true) if the
// path stays inside repo and names a file at least one level below the
// repository root. Relative paths are resolved against repo; absolute
// paths are accepted only when they fall under repo. Prose labels,
// tokens with spaces, traversal attempts, and repo-root tokens
// (".", absolute repo path) are rejected — authorizing the repo root
// would collapse file-level sandbox restrictions back to repo-wide
// writes, so we fail closed here.
//
// repo is assumed to already be absolute; callers that cannot guarantee
// that should pre-resolve it via filepath.Abs.
func ValidatePath(repo, candidate string) (string, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" || isLabel(candidate) || strings.ContainsAny(candidate, " \t") {
		return "", false
	}

	var abs string
	if filepath.IsAbs(candidate) {
		abs = filepath.Clean(candidate)
	} else {
		abs = filepath.Clean(filepath.Join(repo, candidate))
	}

	rel, err := filepath.Rel(repo, abs)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if rel == "." {
		return "", false
	}
	return abs, true
}

// ExtractAffectedFilesFromFiles parses each of planFiles for an
// "Affected Files" section and returns the de-duplicated union of
// validated paths. Missing files are skipped silently (mirroring
// ExtractAffectedFiles' behavior for absent plan.md); a real I/O
// failure on any file is returned. This lets the implement/verify
// launcher authorize the union of plan-approved and review-approved
// files so review-driven additions are writable without a manual
// plan rewrite.
func ExtractAffectedFilesFromFiles(planFiles []string, repo string) ([]string, error) {
	var merged []string
	for _, pf := range planFiles {
		files, err := ExtractAffectedFiles(pf, repo)
		if err != nil {
			return nil, err
		}
		merged = append(merged, files...)
	}
	return dedupe(merged), nil
}

// ExtractAffectedFilesDetailedFromFiles merges the per-file diagnostic
// results in planFiles into a single ExtractionResult. FileExists is
// true if ANY input file exists (so a present review.md with an absent
// plan.md still reads as "file exists"); SectionPresent is true if any
// input file contained an Affected Files section; ValidPaths and
// InvalidEntries are concatenated (with paths de-duplicated). Missing
// input files are treated as "not present" rather than errors, matching
// ExtractAffectedFilesFromFiles' behavior for absent review.md.
func ExtractAffectedFilesDetailedFromFiles(planFiles []string, repo string) (ExtractionResult, error) {
	merged := ExtractionResult{}
	var files []string
	for _, pf := range planFiles {
		res, err := ExtractAffectedFilesDetailed(pf, repo)
		if err != nil {
			return ExtractionResult{}, err
		}
		if res.FileExists {
			merged.FileExists = true
		}
		if res.SectionPresent {
			merged.SectionPresent = true
		}
		files = append(files, res.ValidPaths...)
		merged.InvalidEntries = append(merged.InvalidEntries, res.InvalidEntries...)
	}
	merged.ValidPaths = dedupe(files)
	return merged, nil
}

func isLabel(path string) bool {
	lower := strings.ToLower(path)
	labels := []string{"file", "path", "description", "summary"}
	for _, l := range labels {
		if lower == l {
			return true
		}
	}
	return false
}

func dedupe(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
