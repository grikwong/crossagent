package agent

import (
	"fmt"
	"os"

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

	// Plan builds the launch shape for the given context. The adapter
	// owns any side effects (e.g. writing a sandbox settings JSON) and
	// returns argv excluding argv[0]. WorkDir is optional — an empty
	// string means the caller's CWD should be used.
	Plan(ctx *LaunchContext) (*LaunchPlan, error)
}

// LaunchContext aggregates every input an adapter could need to build a
// launch. It is constructed once per launch and passed to Plan; adapters
// read what they need and ignore the rest. Adding a new field here is
// backward compatible — existing adapters simply don't reference it.
type LaunchContext struct {
	Repo        string // agent working directory / project root
	WorkflowDir string // ~/.crossagent/workflows/<name>/
	PromptFile  string // absolute path to the generated phase prompt
	// AllDirs is the ordered list of directories the agent should be
	// able to access — typically {WorkflowDir, GlobalMemoryDir,
	// ProjectMemDir (if present), AddDirs…}. Each adapter translates
	// this into its native workspace-dir flag syntax (repeated
	// --add-dir for claude/codex, comma-joined --include-directories
	// for gemini).
	AllDirs []string
}

// LaunchPlan is the adapter's answer to "how do I launch this agent?"
// — the argv (minus argv[0]), the prompt text for display, and an
// optional working directory override.
type LaunchPlan struct {
	Args    []string
	Prompt  string
	WorkDir string
}

// NewLaunchContext assembles a LaunchContext from the raw parameters
// that crossagent propagates through its phase-runner plumbing. It
// centralizes the rules for which directories an agent is granted
// access to (workflow dir → global memory → project memory → add_dirs)
// so every adapter sees a consistent ordered list.
func NewLaunchContext(repo, wfDir, promptFile string, addDirs []string, projectMemDir string) *LaunchContext {
	dirs := []string{wfDir, state.GlobalMemoryDir()}
	if projectMemDir != "" {
		if info, err := os.Stat(projectMemDir); err == nil && info.IsDir() {
			dirs = append(dirs, projectMemDir)
		}
	}
	for _, d := range addDirs {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	return &LaunchContext{
		Repo:        repo,
		WorkflowDir: wfDir,
		PromptFile:  promptFile,
		AllDirs:     dirs,
	}
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
