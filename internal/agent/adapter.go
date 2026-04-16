package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grikwong/crossagent/internal/state"
)

// Adapter describes how to launch one family of agent CLIs (Claude Code,
// OpenAI Codex, Google Gemini, …). Each adapter encapsulates:
//
//   - Identity (Name, DisplayName, DefaultCommand) — used for the
//     builtin-agent list, the Web UI dropdown, and validation messages.
//   - Launch shape (Plan) — turns a LaunchContext into an argv + prompt
//     text + working directory. This is the ONLY place a caller needs
//     to know anything adapter-specific; everything else flows through
//     the registry.
//
// Adding a new adapter means creating one file (adapter_<name>.go),
// implementing this interface, and calling RegisterAdapter from init().
// Validation, builtin listing, CLI help strings, the Web UI dropdown,
// and dispatch in LaunchAgent/BuildPhaseCmd all read from the registry,
// so no other code needs to change.
type Adapter interface {
	Name() string           // stable identifier, e.g. "gemini"
	DisplayName() string    // human-readable label, e.g. "Google Gemini"
	DefaultCommand() string // executable to exec when agent.Command is empty
	Family() string         // model family, e.g. "anthropic", "openai", "google"

	// Plan builds the launch shape for the given context. The adapter
	// owns any side effects (e.g. writing a sandbox settings JSON) and
	// returns argv excluding argv[0]. WorkDir is optional — an empty
	// string means the caller's CWD should be used.
	Plan(ctx *LaunchContext) (*LaunchPlan, error)
}

// ExtractionStatus classifies the health of the "Affected Files"
// extraction used to build the adapter sandbox. It is surfaced via
// LaunchContext and PhaseCmdResult so direct-CLI and Web UI consumers
// can warn the operator when the sandbox is degraded (e.g. plan.md
// missing a parseable Affected Files section leads to an empty
// implement sandbox).
//
// The values are content-aware: "malformed" means the source file had
// an Affected Files section with bullets that looked like paths but
// none survived validation, and is distinct from "empty" (well-formed
// but no paths listed) and "missing" (no Affected Files section at all,
// or the source file itself is absent).
type ExtractionStatus string

const (
	// ExtractionOK — at least one valid path was extracted, or the
	// phase has no extraction requirement (e.g. plan phase).
	ExtractionOK ExtractionStatus = "ok"
	// ExtractionMissing — the source file is absent or contains no
	// Affected Files section.
	ExtractionMissing ExtractionStatus = "missing"
	// ExtractionEmpty — an Affected Files section was found but was
	// well-formed and empty (no bullets).
	ExtractionEmpty ExtractionStatus = "empty"
	// ExtractionMalformed — an Affected Files section was found with
	// bullet content that failed path validation (e.g. prose, repo-
	// root tokens, escape attempts). No valid paths resulted.
	ExtractionMalformed ExtractionStatus = "malformed"
)

// LaunchContext aggregates every input an adapter could need to build a
// launch. It is constructed once per launch and passed to Plan; adapters
// read what they need and ignore the rest. Adding a new field here is
// backward compatible — existing adapters simply don't reference it.
type LaunchContext struct {
	Repo             string           // agent working directory / project root
	WorkflowDir      string           // ~/.crossagent/workflows/<name>/
	PromptFile       string           // absolute path to the generated phase prompt
	RepoOverride     string           // absolute path to the staged shadow repo (optional)
	AffectedFiles    []string         // validated list of files the agent is allowed to edit
	PhaseKey         string           // "plan", "review", "implement", "verify"
	ExtractionStatus ExtractionStatus // diagnostic for the Affected Files extraction that produced AffectedFiles
	// AllDirs is the ordered list of directories the agent should be
	// able to access — typically {WorkflowDir, GlobalMemoryDir,
	// ProjectMemDir (if present), AddDirs…}. Each adapter translates
	// this into its native workspace-dir flag syntax (repeated
	// --add-dir for claude/codex, comma-joined --include-directories
	// for gemini).
	AllDirs []string
}

// LaunchPlan is the adapter's answer to "how do I launch this agent?"
// — the command to execute, the argv (minus argv[0]), the prompt text
// for display, and an optional working directory override.
type LaunchPlan struct {
	Command string
	Args    []string
	Prompt  string
	WorkDir string
}

// NewLaunchContext assembles a LaunchContext from the raw parameters
// that crossagent propagates through its phase-runner plumbing. It
// centralizes the rules for which directories an agent is granted
// access to (workflow dir → home dir → global memory → project memory → add_dirs)
// so every adapter sees a consistent ordered list of resolved paths.
func NewLaunchContext(repo, wfDir, promptFile string, addDirs []string, projectMemDir string, affectedFiles []string, phaseKey string, extractionStatus ExtractionStatus) *LaunchContext {
	// resolve returns the absolute, symlink-evaluated version of p.
	// This is critical for macOS seatbelt sandboxes which often fail
	// on non-canonical paths (e.g. /Users vs /private/var/users).
	resolve := func(p string) string {
		if abs, err := filepath.EvalSymlinks(p); err == nil {
			return abs
		}
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}

	rawDirs := []string{}
	// 1. Workflow directory (ALWAYS first slot for tools like gemini)
	rawDirs = append(rawDirs, resolve(wfDir))

	// 2. Crossagent home directory (broad coverage for all state)
	rawDirs = append(rawDirs, resolve(state.Home()))

	// 3. Global memory directory
	rawDirs = append(rawDirs, resolve(state.GlobalMemoryDir()))

	// 4. Project memory directory
	if projectMemDir != "" {
		if info, err := os.Stat(projectMemDir); err == nil && info.IsDir() {
			rawDirs = append(rawDirs, resolve(projectMemDir))
		}
	}

	// 5. Additional user-specified directories
	for _, d := range addDirs {
		if d != "" {
			rawDirs = append(rawDirs, resolve(d))
		}
	}
	
	// Deduplicate and consolidate while preserving order.
	// We keep a directory if it is not already covered by a parent
	// directory that appeared EARLIER in the list.
	unique := []string{}
	for _, d := range rawDirs {
		isSub := false
		for _, existing := range unique {
			rel, err := filepath.Rel(existing, d)
			if err == nil && !strings.HasPrefix(rel, "..") && rel != ".." {
				isSub = true
				break
			}
		}
		if !isSub {
			unique = append(unique, d)
		}
	}
	
	resolvedAffected := []string{}
	for _, f := range affectedFiles {
		resolvedAffected = append(resolvedAffected, resolve(f))
	}
	
	return &LaunchContext{
		Repo:             resolve(repo),
		WorkflowDir:      resolve(wfDir),
		PromptFile:       resolve(promptFile),
		AllDirs:          unique,
		AffectedFiles:    resolvedAffected,
		PhaseKey:         phaseKey,
		ExtractionStatus: extractionStatus,
	}
}

// assertUnderRepo is a defensive check used by adapter sandbox builders.
// It returns (cleaned, true) when p resolves to an absolute path within
// repo. Canonicalization is centralized in judge.ExtractAffectedFiles;
// this helper fails closed if a non-canonical entry ever reaches an
// adapter.
func assertUnderRepo(repo, p string) (string, bool) {
	if !filepath.IsAbs(p) {
		return "", false
	}
	absRepo := repo
	if a, err := filepath.Abs(repo); err == nil {
		absRepo = a
	}
	cleaned := filepath.Clean(p)
	rel, err := filepath.Rel(absRepo, cleaned)
	if err != nil {
		return "", false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return cleaned, true
}

// ── Registry ────────────────────────────────────────────────────────────────

var adapterRegistry = map[string]Adapter{}

// RegisterAdapter adds an adapter to the registry. Panics on duplicate
// names because registration happens at process start (in init()) and
// a duplicate is always a programming error.
func RegisterAdapter(a Adapter) {
	name := a.Name()
	if name == "" {
		panic("agent.RegisterAdapter: empty adapter name")
	}
	if _, dup := adapterRegistry[name]; dup {
		panic(fmt.Sprintf("agent.RegisterAdapter: duplicate adapter %q", name))
	}
	adapterRegistry[name] = a
	registrationOrder = append(registrationOrder, name)
}

// AdapterFor returns the registered adapter with the given name.
func AdapterFor(name string) (Adapter, bool) {
	a, ok := adapterRegistry[name]
	return a, ok
}

// AdapterNames returns the set of registered adapter names in
// registration order (stable: claude, codex, gemini, then any future
// additions in the order they registered). Used to build the Web UI
// dropdown, CLI usage strings, and validation messages.
//
// The order is deterministic: registration happens at init() time in
// filename order of the adapter_<name>.go files.
func AdapterNames() []string {
	// Maps have random iteration order. Use a separate ordered list
	// captured at registration time to keep output stable.
	return append([]string(nil), registrationOrder...)
}

// ValidAdapter returns true iff name is a registered adapter.
func ValidAdapter(name string) bool {
	_, ok := adapterRegistry[name]
	return ok
}

// registrationOrder tracks adapter registration order for deterministic
// enumeration. Updated by RegisterAdapter.
var registrationOrder []string

// registerForOrderedEnumeration is called from RegisterAdapter to keep
// the order slice in sync. Kept as a small helper so tests can clear
// state cleanly if needed.
func init() {
	// Nothing to do at package init — adapters register themselves via
	// their own init() functions. This empty init exists only to give
	// a clear anchor point for the registration sequencing note above.
}
