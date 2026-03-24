package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"time"

	"github.com/grikwong/crossagent/internal/agent"
	"github.com/grikwong/crossagent/internal/cli"
	"github.com/grikwong/crossagent/internal/judge"
	"github.com/grikwong/crossagent/internal/prompt"
	"github.com/grikwong/crossagent/internal/state"
	"github.com/grikwong/crossagent/internal/web"
)

var Version = "dev"
var Commit = ""

func displayVersion() string {
	if Version == "dev" && Commit != "" {
		return "dev-" + Commit
	}
	return Version
}

// ── ANSI Colors ─────────────────────────────────────────────────────────────

const (
	RED     = "\033[0;31m"
	GREEN   = "\033[0;32m"
	YELLOW  = "\033[1;33m"
	BLUE    = "\033[0;34m"
	MAGENTA = "\033[0;35m"
	CYAN    = "\033[0;36m"
	BOLD    = "\033[1m"
	DIM     = "\033[2m"
	NC      = "\033[0m" // No color / reset
)

var phaseColors = [5]string{"", BLUE, MAGENTA, CYAN, GREEN}
var phaseLabels = [5]string{"", "PLAN", "REVIEW", "IMPLEMENT", "VERIFY"}

// ── UI Helpers ──────────────────────────────────────────────────────────────

func info(msg string)    { fmt.Fprintf(os.Stderr, "  %si%s %s\n", BLUE, NC, msg) }
func success(msg string) { fmt.Fprintf(os.Stderr, "  %s✓%s %s\n", GREEN, NC, msg) }
func warn(msg string)    { fmt.Fprintf(os.Stderr, "  %s!%s %s\n", YELLOW, NC, msg) }
func die(msg string)     { fmt.Fprintf(os.Stderr, "  %s✗%s %s\n", RED, NC, msg); os.Exit(1) }

func separator() {
	fmt.Fprintf(os.Stderr, "  %s──────────────────────────────────────────────────%s\n", DIM, NC)
}

func phaseBanner(p int, name, tool string) {
	c := phaseColors[p]
	label := phaseLabels[p]
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", c, NC)
	fmt.Fprintf(os.Stderr, "  %sPHASE %d: %s%s\n", BOLD, p, label, NC)
	fmt.Fprintf(os.Stderr, "  %sTool:%s     %s\n", DIM, NC, tool)
	fmt.Fprintf(os.Stderr, "  %sWorkflow:%s %s\n", DIM, NC, name)
	fmt.Fprintf(os.Stderr, "  %s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", c, NC)
	fmt.Fprintln(os.Stderr)
}

// ── Main ────────────────────────────────────────────────────────────────────

func main() {
	cmd := "help"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	// Handle --version and --help before EnsureDirs
	switch cmd {
	case "--version", "-v", "version":
		fmt.Printf("crossagent %s\n", displayVersion())
		os.Exit(0)
	case "--help", "-h", "help":
		usage()
		os.Exit(0)
	}

	if err := state.EnsureDirs(); err != nil {
		die(fmt.Sprintf("Failed to initialize: %v", err))
	}

	args := os.Args[2:]

	switch cmd {
	case "new":
		cmdNew(args)
	case "plan":
		cmdPlan(args)
	case "review":
		cmdReview(args)
	case "implement", "impl":
		cmdImplement(args)
	case "verify":
		cmdVerify(args)
	case "status", "st":
		cmdStatus(args)
	case "list", "ls":
		cmdList(args)
	case "agents", "agent":
		cmdAgents(args)
	case "repos":
		cmdRepos(args)
	case "projects", "project":
		cmdProjects(args)
	case "move":
		cmdMove(args)
	case "phase-cmd":
		cmdPhaseCmd(args)
	case "use":
		cmdUse(args)
	case "next":
		cmdNext(args)
	case "advance":
		cmdAdvance(args)
	case "revert":
		cmdRevert(args)
	case "supervise":
		cmdSupervise(args)
	case "done":
		cmdDone(args)
	case "open":
		cmdOpen(args)
	case "memory", "mem":
		cmdMemory(args)
	case "log":
		cmdLog(args)
	case "reset":
		cmdReset(args)
	case "serve":
		cmdServe(args)
	default:
		die(fmt.Sprintf("Unknown command: %s. Run 'crossagent help'.", cmd))
	}
}

// ── Workflow Commands ───────────────────────────────────────────────────────

func cmdNew(args []string) {
	var name, repo, project string
	var addDirs []string

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--repo":
			requireArg(args, i)
			repo = args[i+1]
			i += 2
		case "--add-dir":
			requireArg(args, i)
			addDirs = append(addDirs, args[i+1])
			i += 2
		case "--project":
			requireArg(args, i)
			project = args[i+1]
			i += 2
		case "-h", "--help":
			fmt.Println("Usage: crossagent new <name> [--repo <path>] [--add-dir <path>]... [--project <name>]")
			os.Exit(0)
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			name = args[i]
			i++
		}
	}

	if name == "" {
		die("Usage: crossagent new <name> [--repo <path>] [--add-dir <path>]... [--project <name>]")
	}

	// Sanitize name (matching bash behavior)
	name = cli.SanitizeName(name)
	if name == "" {
		die("Invalid workflow name after sanitization.")
	}

	// Default project
	if project == "" {
		project = "default"
	}
	if !state.ProjectExists(project) {
		die(fmt.Sprintf("Project '%s' not found. Create it first: %scrossagent projects new %s%s", project, BOLD, project, NC))
	}

	// Check workflow doesn't exist
	if state.WorkflowExists(name) {
		die(fmt.Sprintf("Workflow '%s' already exists. Use %scrossagent use %s%s to switch to it.", name, BOLD, name, NC))
	}

	// Default repo to cwd
	if repo == "" {
		cwd, err := os.Getwd()
		if err != nil {
			die(fmt.Sprintf("Cannot get working directory: %v", err))
		}
		repo = cwd
	}
	resolvedRepo, err := cli.ValidatePath(repo)
	if err != nil {
		die(fmt.Sprintf("Repository: %v", err))
	}
	repo = resolvedRepo

	// Resolve add_dirs
	var resolvedAddDirs []string
	for _, ad := range addDirs {
		resolved, err := cli.ValidatePath(ad)
		if err != nil {
			die(fmt.Sprintf("Additional directory: %v", err))
		}
		resolvedAddDirs = append(resolvedAddDirs, resolved)
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sNew Workflow: %s%s\n", BOLD, name, NC)
	separator()
	fmt.Fprintf(os.Stderr, "  %sRepository:%s %s\n", DIM, NC, repo)
	fmt.Fprintf(os.Stderr, "  %sProject:%s    %s\n", DIM, NC, project)
	if len(resolvedAddDirs) > 0 {
		fmt.Fprintf(os.Stderr, "  %sExtra dirs:%s %s\n", DIM, NC, strings.Join(resolvedAddDirs, ","))
	}
	fmt.Fprintln(os.Stderr)

	// Read description
	var desc string
	fi, _ := os.Stdin.Stat()
	if fi.Mode()&os.ModeCharDevice != 0 {
		// Terminal — interactive
		fmt.Fprintf(os.Stderr, "  Describe the feature/task %s(empty line to finish):%s\n", DIM, NC)
		fmt.Fprintln(os.Stderr)
		scanner := bufio.NewScanner(os.Stdin)
		var lines []string
		for {
			fmt.Fprint(os.Stderr, "  > ")
			if !scanner.Scan() {
				break
			}
			line := scanner.Text()
			if line == "" && len(lines) > 0 {
				break
			}
			lines = append(lines, line)
		}
		desc = strings.Join(lines, "\n")
	} else {
		// Pipe — read all
		data, err := os.ReadFile("/dev/stdin")
		if err == nil {
			desc = strings.TrimSpace(string(data))
		}
	}

	if desc == "" {
		die("Description cannot be empty.")
	}

	if err := state.CreateWorkflow(name, repo, project, desc, resolvedAddDirs); err != nil {
		die(fmt.Sprintf("Failed to create workflow: %v", err))
	}

	fmt.Fprintln(os.Stderr)
	success(fmt.Sprintf("Workflow '%s' created", name))
	success(fmt.Sprintf("Current phase: %s1 — Plan%s", BOLD, NC))
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Next: %scrossagent plan%s\n", BOLD, NC)
	fmt.Fprintln(os.Stderr)
}

func cmdPlan(args []string) {
	force := hasFlag(args, "--force")

	name, d, err := cli.RequireWorkflow()
	if err != nil {
		die(err.Error())
	}
	phase, _ := state.GetPhase(d)
	pn := state.PhaseNum(phase)

	if pn > 1 && !force {
		warn("Plan phase done. Use --force to redo.")
		return
	}

	cfg := readConfigOrDie(d)
	ag := getPhaseAgentOrDie(d, "plan")

	phaseBanner(1, name, ag.DisplayName)

	promptFile, err := prompt.GeneratePlanPrompt(d, cfg)
	if err != nil {
		die(fmt.Sprintf("Failed to generate plan prompt: %v", err))
	}
	info(fmt.Sprintf("Prompt:  %s", promptFile))
	info(fmt.Sprintf("Output:  %s/plan.md", d))
	separator()

	info(fmt.Sprintf("Agent:   %s (%s)", ag.Name, ag.DisplayName))
	launchAgentOrDie(ag, cfg.Repo, promptFile, d, cfg.AddDirs)

	fmt.Fprintln(os.Stderr)
	if fileExists(filepath.Join(d, "plan.md")) {
		success(fmt.Sprintf("Plan created: %s/plan.md", d))
		state.SetPhase(d, "2")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "  Next: %scrossagent review%s\n", BOLD, NC)
	} else {
		warn(fmt.Sprintf("Plan file not found at: %s/plan.md", d))
		warn("If Claude wrote it elsewhere, copy it there manually.")
		fmt.Fprintf(os.Stderr, "  Then: %scrossagent advance%s to proceed to Phase 2\n", BOLD, NC)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdReview(args []string) {
	force := hasFlag(args, "--force")

	name, d, err := cli.RequireWorkflow()
	if err != nil {
		die(err.Error())
	}
	phase, _ := state.GetPhase(d)
	pn := state.PhaseNum(phase)

	if pn < 2 {
		die(fmt.Sprintf("Complete Phase 1 first. Run: %scrossagent plan%s", BOLD, NC))
	}
	if pn > 2 && !force {
		warn("Review phase done. Use --force to redo.")
		return
	}
	if !fileExists(filepath.Join(d, "plan.md")) {
		die(fmt.Sprintf("Plan file missing: %s/plan.md", d))
	}

	cfg := readConfigOrDie(d)
	ag := getPhaseAgentOrDie(d, "review")

	phaseBanner(2, name, ag.DisplayName)

	promptFile, err := prompt.GenerateReviewPrompt(d, cfg)
	if err != nil {
		die(fmt.Sprintf("Failed to generate review prompt: %v", err))
	}
	info(fmt.Sprintf("Prompt:  %s", promptFile))
	info(fmt.Sprintf("Output:  %s/review.md", d))
	separator()

	info(fmt.Sprintf("Agent:   %s (%s)", ag.Name, ag.DisplayName))
	launchAgentOrDie(ag, cfg.Repo, promptFile, d, cfg.AddDirs)

	fmt.Fprintln(os.Stderr)
	if fileExists(filepath.Join(d, "review.md")) {
		success(fmt.Sprintf("Review created: %s/review.md", d))
		state.SetPhase(d, "3")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "  Next: %scrossagent implement%s\n", BOLD, NC)
	} else {
		warn(fmt.Sprintf("Review file not found at: %s/review.md", d))
		fmt.Fprintf(os.Stderr, "  Then: %scrossagent advance%s to proceed to Phase 3\n", BOLD, NC)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdImplement(args []string) {
	force := false
	implPhase := 1

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--phase":
			requireArg(args, i)
			var err error
			implPhase, err = strconv.Atoi(args[i+1])
			if err != nil || implPhase < 1 {
				die("Implementation phase must be a positive integer.")
			}
			i += 2
		case "--force":
			force = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			i++
		}
	}

	name, d, err := cli.RequireWorkflow()
	if err != nil {
		die(err.Error())
	}
	phase, _ := state.GetPhase(d)
	pn := state.PhaseNum(phase)

	if pn > 4 && !force {
		warn("Workflow is complete. Use --force to redo.")
		return
	}
	if pn < 3 {
		die(fmt.Sprintf("Complete Phase 2 first. Run: %scrossagent review%s", BOLD, NC))
	}

	cfg := readConfigOrDie(d)
	ag := getPhaseAgentOrDie(d, "implement")

	phaseBanner(3, name, ag.DisplayName)
	info(fmt.Sprintf("Implementation sub-phase: %d", implPhase))
	separator()

	promptFile, err := prompt.GenerateImplementPrompt(d, cfg, implPhase)
	if err != nil {
		die(fmt.Sprintf("Failed to generate implement prompt: %v", err))
	}
	info(fmt.Sprintf("Prompt:  %s", promptFile))

	info(fmt.Sprintf("Agent:   %s (%s)", ag.Name, ag.DisplayName))
	launchAgentOrDie(ag, cfg.Repo, promptFile, d, cfg.AddDirs)

	fmt.Fprintln(os.Stderr)
	success("Implementation session complete")
	state.SetPhase(d, "4")
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  Next: %scrossagent verify%s\n", BOLD, NC)
	fmt.Fprintf(os.Stderr, "  %sOr run another sub-phase: crossagent implement --phase %d%s\n", DIM, implPhase+1, NC)
	fmt.Fprintln(os.Stderr)
}

func cmdVerify(args []string) {
	force := hasFlag(args, "--force")

	name, d, err := cli.RequireWorkflow()
	if err != nil {
		die(err.Error())
	}
	phase, _ := state.GetPhase(d)
	pn := state.PhaseNum(phase)

	if pn > 4 && !force {
		warn("Workflow is complete. Use --force to redo.")
		return
	}
	if pn < 4 {
		die(fmt.Sprintf("Complete Phase 3 first. Run: %scrossagent implement%s", BOLD, NC))
	}

	cfg := readConfigOrDie(d)
	ag := getPhaseAgentOrDie(d, "verify")

	phaseBanner(4, name, ag.DisplayName)

	promptFile, err := prompt.GenerateVerifyPrompt(d, cfg)
	if err != nil {
		die(fmt.Sprintf("Failed to generate verify prompt: %v", err))
	}
	info(fmt.Sprintf("Prompt:  %s", promptFile))
	info(fmt.Sprintf("Output:  %s/verify.md", d))
	separator()

	info(fmt.Sprintf("Agent:   %s (%s)", ag.Name, ag.DisplayName))
	launchAgentOrDie(ag, cfg.Repo, promptFile, d, cfg.AddDirs)

	fmt.Fprintln(os.Stderr)
	if fileExists(filepath.Join(d, "verify.md")) {
		success(fmt.Sprintf("Verification report: %s/verify.md", d))
		state.SetPhase(d, "done")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "  %s%sWorkflow complete!%s\n", GREEN, BOLD, NC)
		fmt.Fprintf(os.Stderr, "  Review the report and address any issues found.\n")
	} else {
		warn(fmt.Sprintf("Verification report not found at: %s/verify.md", d))
		fmt.Fprintf(os.Stderr, "  Then: %scrossagent done%s to mark complete\n", BOLD, NC)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdNext(args []string) {
	name, d, err := cli.RequireWorkflow()
	if err != nil {
		die(err.Error())
	}
	phase, _ := state.GetPhase(d)

	switch phase {
	case "1":
		cmdPlan(args)
	case "2":
		cmdReview(args)
	case "3":
		cmdImplement(args)
	case "4":
		cmdVerify(args)
	case "done":
		success(fmt.Sprintf("Workflow '%s' is already complete.", name))
	default:
		die(fmt.Sprintf("Unknown phase: %s", phase))
	}
}

func cmdStatus(args []string) {
	jsonMode := hasFlag(args, "--json")
	name, d := resolveWorkflow(flagStr(args, "--workflow"), jsonMode)

	phase, _ := state.GetPhase(d)
	cfg := readConfigOrDie(d)
	desc, _ := state.GetDescription(d)
	pn := state.PhaseNum(phase)

	phaseLabel := state.PhaseLabel(phase)

	retryCount := confInt(d, "retry_count", 0)
	maxRetries := confInt(d, "max_retries", 10)

	planAg := getPhaseAgentOrDie(d, "plan")
	reviewAg := getPhaseAgentOrDie(d, "review")
	implementAg := getPhaseAgentOrDie(d, "implement")
	verifyAg := getPhaseAgentOrDie(d, "verify")

	if jsonMode {
		addDirs := make([]string, 0)
		if len(cfg.AddDirs) > 0 && !(len(cfg.AddDirs) == 1 && cfg.AddDirs[0] == "") {
			addDirs = cfg.AddDirs
		}

		out := cli.StatusJSON{
			Name:        name,
			Phase:       phase,
			PhaseLabel:  phaseLabel,
			Complete:    phase == "done",
			Project:     cfg.Project,
			Repo:        cfg.Repo,
			AddDirs:     addDirs,
			Repos:       cli.ReposJSON{Primary: cfg.Repo, Additional: addDirs},
			Description: desc,
			Created:     cfg.Created,
			WorkflowDir: d,
			Agents: cli.OrderedAgents{
				Plan:      cli.AgentRefJSON{Name: planAg.Name, DisplayName: planAg.DisplayName},
				Review:    cli.AgentRefJSON{Name: reviewAg.Name, DisplayName: reviewAg.DisplayName},
				Implement: cli.AgentRefJSON{Name: implementAg.Name, DisplayName: implementAg.DisplayName},
				Verify:    cli.AgentRefJSON{Name: verifyAg.Name, DisplayName: verifyAg.DisplayName},
			},
			RetryCount: retryCount,
			MaxRetries: maxRetries,
			Artifacts: cli.OrderedArtifacts{
				Plan:      makeArtifact(filepath.Join(d, "plan.md")),
				Review:    makeArtifact(filepath.Join(d, "review.md")),
				Implement: makeArtifact(filepath.Join(d, "implement.md")),
				Verify:    makeArtifact(filepath.Join(d, "verify.md")),
				Memory:    makeArtifact(filepath.Join(d, "memory.md")),
			},
			ChatHistory: cli.OrderedChatHistory{
				Plan:      makeChatHistoryEntry(filepath.Join(d, "chat-history", "plan.log")),
				Review:    makeChatHistoryEntry(filepath.Join(d, "chat-history", "review.log")),
				Implement: makeChatHistoryEntry(filepath.Join(d, "chat-history", "implement.log")),
				Verify:    makeChatHistoryEntry(filepath.Join(d, "chat-history", "verify.log")),
			},
		}
		if err := cli.PrintStatusJSON(out); err != nil {
			die(err.Error())
		}
		return
	}

	// Human-readable output
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sWorkflow: %s%s\n", BOLD, name, NC)
	separator()
	fmt.Fprintf(os.Stderr, "  %sCreated:%s     %s\n", DIM, NC, cfg.Created)
	fmt.Fprintf(os.Stderr, "  %sProject:%s     %s\n", DIM, NC, cfg.Project)
	fmt.Fprintf(os.Stderr, "  %sRepository:%s  %s\n", DIM, NC, cfg.Repo)
	if len(cfg.AddDirs) > 0 && !(len(cfg.AddDirs) == 1 && cfg.AddDirs[0] == "") {
		fmt.Fprintf(os.Stderr, "  %sExtra dirs:%s  %s\n", DIM, NC, strings.Join(cfg.AddDirs, ","))
	}
	fmt.Fprintf(os.Stderr, "  %sAgents:%s      plan=%s, review=%s, implement=%s, verify=%s\n",
		DIM, NC, planAg.Name, reviewAg.Name, implementAg.Name, verifyAg.Name)
	fmt.Fprintf(os.Stderr, "  %sDescription:%s\n", DIM, NC)
	for _, line := range strings.Split(desc, "\n") {
		fmt.Fprintf(os.Stderr, "    %s\n", line)
	}
	fmt.Fprintln(os.Stderr)

	for i := 1; i <= 4; i++ {
		label := phaseLabels[i]
		var icon string
		if pn > i {
			icon = fmt.Sprintf("%s✓%s", GREEN, NC)
		} else if pn == i {
			icon = fmt.Sprintf("%s→%s", YELLOW, NC)
		} else {
			icon = fmt.Sprintf("%s○%s", DIM, NC)
		}
		extra := ""
		switch i {
		case 1:
			if fileExists(filepath.Join(d, "plan.md")) {
				extra = fmt.Sprintf(" %s(%d lines)%s", DIM, countLines(filepath.Join(d, "plan.md")), NC)
			}
		case 2:
			if fileExists(filepath.Join(d, "review.md")) {
				extra = fmt.Sprintf(" %s(%d lines)%s", DIM, countLines(filepath.Join(d, "review.md")), NC)
			}
		case 4:
			if fileExists(filepath.Join(d, "verify.md")) {
				extra = fmt.Sprintf(" %s(%d lines)%s", DIM, countLines(filepath.Join(d, "verify.md")), NC)
			}
		}
		fmt.Fprintf(os.Stderr, "  %s Phase %d: %-12s%s\n", icon, i, label, extra)
	}
	fmt.Fprintln(os.Stderr)

	if phase == "done" {
		fmt.Fprintf(os.Stderr, "  %s%sWorkflow complete%s\n", GREEN, BOLD, NC)
	} else {
		cmds := [5]string{"", "plan", "review", "implement", "verify"}
		p := state.PhaseNum(phase)
		if p >= 1 && p <= 4 {
			fmt.Fprintf(os.Stderr, "  Next: %scrossagent %s%s\n", BOLD, cmds[p], NC)
		}
	}
	fmt.Fprintln(os.Stderr)
}

func cmdList(args []string) {
	jsonMode := hasFlag(args, "--json")

	current, _ := state.GetCurrent()

	if jsonMode {
		workflows, _ := state.ListWorkflows()
		wfItems := make([]cli.ListWorkflowJSON, 0)
		for _, wname := range workflows {
			d := state.WorkflowDir(wname)
			wphase, _ := state.GetPhase(d)
			wproject := confStr(d, "project", "default")
			wfItems = append(wfItems, cli.ListWorkflowJSON{
				Name:       wname,
				Phase:      wphase,
				PhaseLabel: state.PhaseLabel(wphase),
				Active:     wname == current,
				Project:    wproject,
				Agents: cli.OrderedAgentNames{
					Plan:      phaseAgentName(d, "plan"),
					Review:    phaseAgentName(d, "review"),
					Implement: phaseAgentName(d, "implement"),
					Verify:    phaseAgentName(d, "verify"),
				},
			})
		}

		projects, _ := state.ListProjects()
		projItems := make([]cli.ListProjectJSON, 0)
		for _, p := range projects {
			projItems = append(projItems, cli.ListProjectJSON{
				Name:          p.Name,
				WorkflowCount: p.WorkflowCount,
			})
		}

		out := cli.ListJSON{
			Workflows: wfItems,
			Projects:  projItems,
			Active:    current,
		}
		if err := cli.PrintJSONCompact(out); err != nil {
			die(err.Error())
		}
		return
	}

	// Human-readable
	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sWorkflows%s\n", BOLD, NC)
	separator()

	workflows, _ := state.ListWorkflows()
	count := 0
	for _, wname := range workflows {
		d := state.WorkflowDir(wname)
		wphase, _ := state.GetPhase(d)
		wproject := confStr(d, "project", "default")
		indicator := ""
		if wname == current {
			indicator = fmt.Sprintf(" %s(active)%s", CYAN, NC)
		}

		var phaseDisplay string
		if wphase == "done" {
			phaseDisplay = fmt.Sprintf("%scomplete%s", GREEN, NC)
		} else {
			phaseDisplay = fmt.Sprintf("phase %s — %s", wphase, state.PhaseLabel(wphase))
		}

		fmt.Fprintf(os.Stderr, "  %s%s%s — %s%s\n", BOLD, wname, NC, phaseDisplay, indicator)
		fmt.Fprintf(os.Stderr, "    %sproject:%s %s  %sagents:%s plan=%s, review=%s, implement=%s, verify=%s\n",
			DIM, NC, wproject, DIM, NC,
			phaseAgentName(d, "plan"),
			phaseAgentName(d, "review"),
			phaseAgentName(d, "implement"),
			phaseAgentName(d, "verify"))
		count++
	}

	if count == 0 {
		fmt.Fprintf(os.Stderr, "  %sNo workflows yet.%s\n", DIM, NC)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdUse(args []string) {
	if len(args) == 0 {
		die("Usage: crossagent use <name>")
	}
	target := args[0]
	if !state.WorkflowExists(target) {
		die(fmt.Sprintf("Workflow '%s' not found. Run %scrossagent list%s", target, BOLD, NC))
	}
	if err := state.SetCurrent(target); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Switched to: %s", target))
}

func cmdReset(args []string) {
	if len(args) == 0 {
		die("Usage: crossagent reset <name>")
	}
	target := args[0]
	d := state.WorkflowDir(target)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", target))
	}

	fmt.Fprintf(os.Stderr, "  Reset workflow '%s'? All artifacts will be deleted. [y/N] ", target)
	reader := bufio.NewReader(os.Stdin)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(confirm)
	if confirm != "y" && confirm != "Y" {
		fmt.Fprintln(os.Stderr, "  Cancelled.")
		return
	}

	os.RemoveAll(d)
	current, _ := state.GetCurrent()
	if current == target {
		os.Remove(filepath.Join(state.Home(), "current"))
	}

	success(fmt.Sprintf("Workflow '%s' deleted.", target))
}

func cmdAdvance(args []string) {
	_, d := resolveWorkflow(flagStr(args, "--workflow"), false)
	phase, _ := state.GetPhase(d)

	if phase == "done" {
		warn("Workflow already complete.")
		return
	}

	pn := state.PhaseNum(phase)
	next := pn + 1
	if next > 4 {
		state.SetPhase(d, "done")
		success("Workflow marked complete.")
	} else {
		state.SetPhase(d, strconv.Itoa(next))
		success(fmt.Sprintf("Advanced to phase %d — %s", next, phaseLabels[next]))
	}
}

func cmdDone(args []string) {
	name, d := resolveWorkflow(flagStr(args, "--workflow"), false)
	state.SetPhase(d, "done")
	success(fmt.Sprintf("Workflow '%s' marked complete.", name))
}

func cmdServe(args []string) {
	port := "3456"
	openBrowser := false

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--port":
			requireArg(args, i)
			port = args[i+1]
			i += 2
		case "--open":
			openBrowser = true
			i++
		case "-h", "--help":
			fmt.Println("Usage: crossagent serve [--port <port>] [--open]")
			os.Exit(0)
		default:
			die(fmt.Sprintf("Unknown option: %s", args[i]))
		}
	}

	// Respect CROSSAGENT_PORT env var
	if envPort := os.Getenv("CROSSAGENT_PORT"); envPort != "" && port == "3456" {
		port = envPort
	}

	addr := ":" + port

	if openBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			url := fmt.Sprintf("http://localhost:%s", port)
			if runtime.GOOS == "darwin" {
				exec.Command("open", url).Start()
			} else {
				exec.Command("xdg-open", url).Start()
			}
		}()
	}

	web.AppVersion = displayVersion()
	fmt.Fprintf(os.Stderr, "\n  Crossagent UI running at http://localhost:%s\n\n", port)
	if err := web.Serve(addr); err != nil {
		die(fmt.Sprintf("Server failed: %v", err))
	}
}

func cmdOpen(args []string) {
	_, d, err := cli.RequireWorkflow()
	if err != nil {
		die(err.Error())
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", d)
	} else {
		cmd = exec.Command("xdg-open", d)
	}
	if err := cmd.Run(); err != nil {
		die(fmt.Sprintf("No supported opener found. Open the directory manually: %s", d))
	}
}

func cmdLog(args []string) {
	chatMode := hasFlag(args, "--chat")

	name, d, err := cli.RequireWorkflow()
	if err != nil {
		die(err.Error())
	}

	if chatMode {
		fmt.Fprintln(os.Stderr)
		fmt.Fprintf(os.Stderr, "  %sChat History: %s%s\n", BOLD, name, NC)
		separator()

		found := false
		for _, phase := range []string{"plan", "review", "implement", "verify"} {
			logPath := filepath.Join(d, "chat-history", phase+".log")
			if fileExists(logPath) {
				found = true
				fmt.Fprintln(os.Stderr)
				fmt.Fprintf(os.Stderr, "  %s%s.log%s\n", BOLD, phase, NC)
				fmt.Fprintf(os.Stderr, "  %s%s%s\n", DIM, logPath, NC)
				fmt.Fprintln(os.Stderr)
				data, _ := os.ReadFile(logPath)
				// Write raw terminal data to stdout so ANSI renders correctly
				os.Stdout.Write(data)
				fmt.Fprintln(os.Stdout)
				separator()
			}
		}

		if !found {
			fmt.Fprintf(os.Stderr, "  %sNo chat history yet.%s\n", DIM, NC)
		}
		fmt.Fprintln(os.Stderr)
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sArtifacts: %s%s\n", BOLD, name, NC)
	separator()

	found := false
	// Bash prints plan.md, review.md, verify.md (not implement.md)
	for _, artifact := range []string{"plan.md", "review.md", "verify.md"} {
		path := filepath.Join(d, artifact)
		if fileExists(path) {
			found = true
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  %s%s%s\n", BOLD, artifact, NC)
			fmt.Fprintf(os.Stderr, "  %s%s%s\n", DIM, path, NC)
			fmt.Fprintln(os.Stderr)
			data, _ := os.ReadFile(path)
			for _, line := range strings.Split(string(data), "\n") {
				fmt.Fprintf(os.Stderr, "    %s\n", line)
			}
			fmt.Fprintln(os.Stderr)
			separator()
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "  %sNo artifacts yet.%s\n", DIM, NC)
	}
	fmt.Fprintln(os.Stderr)
}

// ── Agents Subcommands ──────────────────────────────────────────────────────

func cmdAgents(args []string) {
	subcmd := "list"
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "list":
		cmdAgentsList(args)
	case "add":
		cmdAgentsAdd(args)
	case "remove", "rm", "delete":
		cmdAgentsRemove(args)
	case "show", "assignments":
		cmdAgentsShow(args)
	case "assign":
		cmdAgentsAssign(args)
	case "reset":
		cmdAgentsReset(args)
	case "help", "--help", "-h":
		fmt.Println(`Usage:
  crossagent agents list [--json]
  crossagent agents add <name> --adapter <claude|codex> [--command <cmd>] [--display-name <name>]
  crossagent agents remove <name>
  crossagent agents show [--workflow <name>] [--json]
  crossagent agents assign <plan|review|implement|verify> <agent> [--workflow <name>]
  crossagent agents reset <plan|review|implement|verify> [--workflow <name>]`)
	default:
		die(fmt.Sprintf("Unknown agents command: %s. Run 'crossagent agents help'.", subcmd))
	}
}

func cmdAgentsList(args []string) {
	jsonMode := hasFlag(args, "--json")

	agents, err := agent.ListAgents()
	if err != nil {
		die(err.Error())
	}

	if jsonMode {
		items := make([]cli.AgentJSON, 0, len(agents))
		for _, ag := range agents {
			items = append(items, cli.AgentJSON{
				Name:        ag.Name,
				DisplayName: ag.DisplayName,
				Adapter:     ag.Adapter,
				Command:     ag.Command,
				Builtin:     ag.Builtin,
			})
		}
		out := cli.AgentsListJSON{Agents: items}
		if err := cli.PrintJSONCompact(out); err != nil {
			die(err.Error())
		}
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sAgents%s\n", BOLD, NC)
	separator()
	for _, ag := range agents {
		kind := "custom"
		if ag.Builtin {
			kind = "builtin"
		}
		fmt.Fprintf(os.Stderr, "  %s%s%s — %s %s[%s | adapter=%s | command=%s]%s\n",
			BOLD, ag.Name, NC, ag.DisplayName, DIM, kind, ag.Adapter, ag.Command, NC)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdAgentsAdd(args []string) {
	if len(args) == 0 {
		die("Usage: crossagent agents add <name> --adapter <claude|codex> [--command <cmd>] [--display-name <name>]")
	}

	name := cli.SanitizeAgentName(args[0])
	if name == "" {
		die("Invalid agent name.")
	}
	args = args[1:]

	// Check not builtin
	if name == "claude" || name == "codex" {
		die(fmt.Sprintf("Cannot overwrite builtin agent '%s'.", name))
	}
	// Check doesn't already exist
	if _, err := agent.GetAgent(name); err == nil {
		if ag, _ := agent.GetAgent(name); !ag.Builtin {
			die(fmt.Sprintf("Agent already exists: %s", name))
		}
	}

	var adapter, command, displayName string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--adapter":
			requireArg(args, i)
			adapter = args[i+1]
			i += 2
		case "--command":
			requireArg(args, i)
			command = args[i+1]
			i += 2
		case "--display-name":
			requireArg(args, i)
			displayName = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			die(fmt.Sprintf("Unexpected argument: %s", args[i]))
		}
	}

	if adapter != "claude" && adapter != "codex" {
		die("Agent adapter must be one of: claude, codex")
	}
	if command == "" {
		command = adapter
	}
	if displayName == "" {
		displayName = name
	}

	if err := agent.AddAgent(name, adapter, command, displayName); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Agent '%s' added.", name))
}

func cmdAgentsRemove(args []string) {
	if len(args) == 0 {
		die("Usage: crossagent agents remove <name>")
	}
	name := args[0]

	if name == "claude" || name == "codex" {
		die(fmt.Sprintf("Cannot remove builtin agent '%s'.", name))
	}

	ag, err := agent.GetAgent(name)
	if err != nil || ag.Builtin {
		die(fmt.Sprintf("Agent not found: %s", name))
	}

	// Check not in use by any workflow
	workflows, _ := state.ListWorkflows()
	for _, wf := range workflows {
		d := state.WorkflowDir(wf)
		for _, key := range []string{"plan", "review", "implement", "verify"} {
			assigned := phaseAgentName(d, key)
			if assigned == name {
				die(fmt.Sprintf("Agent '%s' is assigned to workflow '%s' phase '%s'. Reassign it first.", name, wf, key))
			}
		}
	}

	if err := agent.RemoveAgent(name); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Agent '%s' removed.", name))
}

func cmdAgentsShow(args []string) {
	jsonMode := false
	workflow := ""

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--json":
			jsonMode = true
			i++
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			die(fmt.Sprintf("Unexpected argument: %s", args[i]))
		}
	}

	if workflow == "" {
		name, _, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		workflow = name
	}

	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}

	planAg := getPhaseAgentOrDie(d, "plan")
	reviewAg := getPhaseAgentOrDie(d, "review")
	implementAg := getPhaseAgentOrDie(d, "implement")
	verifyAg := getPhaseAgentOrDie(d, "verify")

	if jsonMode {
		out := cli.AgentsShowJSON{
			Workflow: workflow,
			Agents: cli.OrderedAgentNames{
				Plan:      planAg.Name,
				Review:    reviewAg.Name,
				Implement: implementAg.Name,
				Verify:    verifyAg.Name,
			},
		}
		if err := cli.PrintJSONCompact(out); err != nil {
			die(err.Error())
		}
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sWorkflow Agents: %s%s\n", BOLD, workflow, NC)
	separator()
	fmt.Fprintf(os.Stderr, "  plan:      %s %s(%s)%s\n", planAg.Name, DIM, planAg.DisplayName, NC)
	fmt.Fprintf(os.Stderr, "  review:    %s %s(%s)%s\n", reviewAg.Name, DIM, reviewAg.DisplayName, NC)
	fmt.Fprintf(os.Stderr, "  implement: %s %s(%s)%s\n", implementAg.Name, DIM, implementAg.DisplayName, NC)
	fmt.Fprintf(os.Stderr, "  verify:    %s %s(%s)%s\n", verifyAg.Name, DIM, verifyAg.DisplayName, NC)
	fmt.Fprintln(os.Stderr)
}

func cmdAgentsAssign(args []string) {
	if len(args) < 2 {
		die("Usage: crossagent agents assign <plan|review|implement|verify> <agent> [--workflow <name>]")
	}
	phaseArg := args[0]
	agentName := args[1]
	args = args[2:]

	workflow := ""
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			die(fmt.Sprintf("Unexpected argument: %s", args[i]))
		}
	}

	phaseKey, err := state.PhaseKey(phaseArg)
	if err != nil {
		die(fmt.Sprintf("Unknown phase: %s", phaseArg))
	}

	if workflow == "" {
		name, _, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		workflow = name
	}
	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}

	if err := agent.SetPhaseAgent(d, phaseKey, agentName); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Assigned agent '%s' to %s for workflow '%s'.", agentName, phaseKey, workflow))
}

func cmdAgentsReset(args []string) {
	if len(args) == 0 {
		die("Usage: crossagent agents reset <plan|review|implement|verify> [--workflow <name>]")
	}
	phaseArg := args[0]
	args = args[1:]

	workflow := ""
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			die(fmt.Sprintf("Unexpected argument: %s", args[i]))
		}
	}

	phaseKey, err := state.PhaseKey(phaseArg)
	if err != nil {
		die(fmt.Sprintf("Unknown phase: %s", phaseArg))
	}

	if workflow == "" {
		name, _, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		workflow = name
	}
	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}

	if err := agent.ResetPhaseAgent(d, phaseKey); err != nil {
		die(err.Error())
	}
	defaultAgent := agent.DefaultPhaseAgent(phaseKey)
	success(fmt.Sprintf("Reset %s agent for workflow '%s' to default (%s).", phaseKey, workflow, defaultAgent))
}

// ── Repos Subcommands ───────────────────────────────────────────────────────

func cmdRepos(args []string) {
	subcmd := "list"
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "list":
		cmdReposList(args)
	case "add":
		cmdReposAdd(args)
	case "remove", "rm":
		cmdReposRemove(args)
	case "set-primary":
		cmdReposSetPrimary(args)
	case "help", "--help", "-h":
		fmt.Println(`Usage:
  crossagent repos list [--json] [--workflow <name>]
  crossagent repos add <path> [--workflow <name>]
  crossagent repos remove <path> [--workflow <name>]
  crossagent repos set-primary <path> [--workflow <name>]`)
	default:
		die(fmt.Sprintf("Unknown repos command: %s. Run 'crossagent repos help'.", subcmd))
	}
}

func cmdReposList(args []string) {
	jsonMode := false
	workflow := ""

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--json":
			jsonMode = true
			i++
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			die(fmt.Sprintf("Unexpected argument: %s", args[i]))
		}
	}

	if workflow == "" {
		name, _, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		workflow = name
	}
	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}

	cfg := readConfigOrDie(d)
	additional := cleanAddDirs(cfg.AddDirs)

	if jsonMode {
		out := cli.ReposJSON{
			Primary:    cfg.Repo,
			Additional: additional,
		}
		if err := cli.PrintJSONCompact(out); err != nil {
			die(err.Error())
		}
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sRepositories: %s%s\n", BOLD, workflow, NC)
	separator()
	fmt.Fprintf(os.Stderr, "  %s*%s %s %s(primary)%s\n", GREEN, NC, cfg.Repo, DIM, NC)
	for _, ad := range additional {
		fmt.Fprintf(os.Stderr, "    %s\n", ad)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdReposAdd(args []string) {
	var path, workflow string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			path = args[i]
			i++
		}
	}
	if path == "" {
		die("Usage: crossagent repos add <path> [--workflow <name>]")
	}

	resolvedPath, err := cli.ValidatePath(path)
	if err != nil {
		die(err.Error())
	}

	if workflow == "" {
		name, _, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		workflow = name
	}
	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}

	cfg := readConfigOrDie(d)
	if resolvedPath == cfg.Repo {
		die("Path is already the primary repository.")
	}
	for _, ad := range cfg.AddDirs {
		if ad == resolvedPath {
			die(fmt.Sprintf("Path is already in the repository list: %s", resolvedPath))
		}
	}

	newAddDirs := appendAddDir(cfg.AddDirs, resolvedPath)
	if err := state.SetConf(d, "add_dirs", strings.Join(newAddDirs, ",")); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Added repository: %s", resolvedPath))
}

func cmdReposRemove(args []string) {
	var path, workflow string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			path = args[i]
			i++
		}
	}
	if path == "" {
		die("Usage: crossagent repos remove <path> [--workflow <name>]")
	}

	// Resolve if path exists
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = abs
		}
	}

	if workflow == "" {
		name, _, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		workflow = name
	}
	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}

	cfg := readConfigOrDie(d)
	if path == cfg.Repo {
		die(fmt.Sprintf("Cannot remove the primary repository. Use %scrossagent repos set-primary%s to change it.", BOLD, NC))
	}

	found := false
	var newDirs []string
	for _, ad := range cfg.AddDirs {
		if ad == path {
			found = true
		} else if ad != "" {
			newDirs = append(newDirs, ad)
		}
	}
	if !found {
		die(fmt.Sprintf("Path not found in repository list: %s", path))
	}

	csv := strings.Join(newDirs, ",")
	if err := state.SetConf(d, "add_dirs", csv); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Removed repository: %s", path))
}

func cmdReposSetPrimary(args []string) {
	var path, workflow string
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			path = args[i]
			i++
		}
	}
	if path == "" {
		die("Usage: crossagent repos set-primary <path> [--workflow <name>]")
	}

	resolvedPath, err := cli.ValidatePath(path)
	if err != nil {
		die(err.Error())
	}

	if workflow == "" {
		name, _, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		workflow = name
	}
	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}

	cfg := readConfigOrDie(d)
	if resolvedPath == cfg.Repo {
		info("Path is already the primary repository.")
		return
	}

	// Remove new primary from add_dirs if present
	var newAddDirs []string
	for _, ad := range cfg.AddDirs {
		if ad != resolvedPath && ad != "" {
			newAddDirs = append(newAddDirs, ad)
		}
	}

	// Move old primary to add_dirs
	if cfg.Repo != "" {
		found := false
		for _, ad := range newAddDirs {
			if ad == cfg.Repo {
				found = true
				break
			}
		}
		if !found {
			newAddDirs = append(newAddDirs, cfg.Repo)
		}
	}

	state.SetConf(d, "repo", resolvedPath)
	state.SetConf(d, "add_dirs", strings.Join(newAddDirs, ","))
	success(fmt.Sprintf("Primary repository changed to: %s", resolvedPath))
	if cfg.Repo != "" {
		info(fmt.Sprintf("Previous primary moved to additional: %s", cfg.Repo))
	}
}

// ── Projects Subcommands ────────────────────────────────────────────────────

func cmdProjects(args []string) {
	subcmd := "list"
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "list":
		cmdProjectsList(args)
	case "new":
		cmdProjectsNew(args)
	case "delete", "rm":
		cmdProjectsDelete(args)
	case "show":
		cmdProjectsShow(args)
	case "rename":
		cmdProjectsRename(args)
	case "suggest":
		cmdProjectsSuggest(args)
	case "help", "--help", "-h":
		fmt.Println(`Usage:
  crossagent projects list [--json]
  crossagent projects new <name>
  crossagent projects delete <name>
  crossagent projects show <name> [--json]
  crossagent projects rename <old> <new>
  crossagent projects suggest [--description <text>] [--json]`)
	default:
		die(fmt.Sprintf("Unknown projects command: %s. Run 'crossagent projects help'.", subcmd))
	}
}

func cmdProjectsList(args []string) {
	jsonMode := hasFlag(args, "--json")

	projects, err := state.ListProjects()
	if err != nil {
		die(err.Error())
	}

	if jsonMode {
		items := make([]cli.ListProjectJSON, 0, len(projects))
		for _, p := range projects {
			items = append(items, cli.ListProjectJSON{Name: p.Name, WorkflowCount: p.WorkflowCount})
		}
		out := cli.ProjectsListJSON{Projects: items}
		if err := cli.PrintJSONCompact(out); err != nil {
			die(err.Error())
		}
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sProjects%s\n", BOLD, NC)
	separator()
	for _, p := range projects {
		fmt.Fprintf(os.Stderr, "  %s%s%s — %d workflow(s)\n", BOLD, p.Name, NC, p.WorkflowCount)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdProjectsNew(args []string) {
	if len(args) == 0 {
		die("Usage: crossagent projects new <name>")
	}
	name := args[0]
	if err := state.ValidateName(name); err != nil {
		die(err.Error())
	}
	if state.ProjectExists(name) {
		die(fmt.Sprintf("Project '%s' already exists.", name))
	}
	if err := state.CreateProject(name); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Project '%s' created.", name))
}

func cmdProjectsDelete(args []string) {
	if len(args) == 0 {
		die("Usage: crossagent projects delete <name>")
	}
	name := args[0]
	if name == "default" {
		die("Cannot delete the default project.")
	}
	if !state.ProjectExists(name) {
		die(fmt.Sprintf("Project '%s' not found.", name))
	}

	if err := state.DeleteProject(name); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Project '%s' deleted.", name))
}

func cmdProjectsShow(args []string) {
	jsonMode := false
	name := ""
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--json":
			jsonMode = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			name = args[i]
			i++
		}
	}
	if name == "" {
		die("Usage: crossagent projects show <name> [--json]")
	}
	if !state.ProjectExists(name) {
		die(fmt.Sprintf("Project '%s' not found.", name))
	}

	wfs, err := state.ListProjectWorkflows(name)
	if err != nil {
		die(err.Error())
	}

	if jsonMode {
		wfItems := make([]cli.ProjectWorkflowJSON, 0, len(wfs))
		for _, wf := range wfs {
			d := state.WorkflowDir(wf)
			wphase, _ := state.GetPhase(d)
			wfItems = append(wfItems, cli.ProjectWorkflowJSON{
				Name:       wf,
				Phase:      wphase,
				PhaseLabel: state.PhaseLabel(wphase),
			})
		}
		out := cli.ProjectShowJSON{
			Name:          name,
			WorkflowCount: len(wfs),
			Workflows:     wfItems,
			MemoryDir:     filepath.Join(state.ProjectsDir(), name, "memory"),
		}
		if err := cli.PrintJSONCompact(out); err != nil {
			die(err.Error())
		}
		return
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintf(os.Stderr, "  %sProject: %s%s\n", BOLD, name, NC)
	separator()
	fmt.Fprintf(os.Stderr, "  %sMemory:%s %s\n", DIM, NC, filepath.Join(state.ProjectsDir(), name, "memory"))
	fmt.Fprintf(os.Stderr, "  %sWorkflows:%s %d\n", DIM, NC, len(wfs))
	for _, wf := range wfs {
		d := state.WorkflowDir(wf)
		wphase, _ := state.GetPhase(d)
		var phaseDisplay string
		if wphase == "done" {
			phaseDisplay = fmt.Sprintf("%scomplete%s", GREEN, NC)
		} else {
			phaseDisplay = fmt.Sprintf("phase %s — %s", wphase, state.PhaseLabel(wphase))
		}
		fmt.Fprintf(os.Stderr, "    %s%s%s — %s\n", BOLD, wf, NC, phaseDisplay)
	}
	fmt.Fprintln(os.Stderr)
}

func cmdProjectsRename(args []string) {
	if len(args) < 2 {
		die("Usage: crossagent projects rename <old> <new>")
	}
	oldName := args[0]
	newName := args[1]

	if oldName == "default" {
		die("Cannot rename the default project.")
	}
	if err := state.ValidateName(newName); err != nil {
		die(err.Error())
	}
	if !state.ProjectExists(oldName) {
		die(fmt.Sprintf("Project '%s' not found.", oldName))
	}
	if state.ProjectExists(newName) {
		die(fmt.Sprintf("Project '%s' already exists.", newName))
	}

	if err := state.RenameProject(oldName, newName); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Project '%s' renamed to '%s'.", oldName, newName))
}

func cmdProjectsSuggest(args []string) {
	jsonMode := false
	description := ""
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--description":
			requireArg(args, i)
			description = args[i+1]
			i += 2
		case "--json":
			jsonMode = true
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			i++
		}
	}

	// Fall back to current workflow description
	if description == "" {
		_, d, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		description, _ = state.GetDescription(d)
	}
	if description == "" {
		die("No description provided. Use --description <text>")
	}

	suggestion, err := state.SuggestProject(description)
	if err != nil {
		die(err.Error())
	}

	if jsonMode {
		out := cli.ProjectSuggestJSON{
			Score:        0,
			MatchedTerms: "",
		}
		if suggestion != nil {
			out.SuggestedProject = &suggestion.Project
			out.Score = suggestion.Score
			out.MatchedTerms = suggestion.MatchedTerms
		}
		if err := cli.PrintJSONCompact(out); err != nil {
			die(err.Error())
		}
		return
	}

	if suggestion != nil {
		fmt.Fprintln(os.Stderr)
		info(fmt.Sprintf("Suggested project: %s%s%s", BOLD, suggestion.Project, NC))
		info(fmt.Sprintf("Score: %d", suggestion.Score))
		info(fmt.Sprintf("Matched: %s", suggestion.MatchedTerms))
		fmt.Fprintln(os.Stderr)
	} else {
		info("No project suggestion (no strong match found).")
	}
}

func cmdMove(args []string) {
	workflow := ""
	project := ""
	i := 0
	for i < len(args) {
		switch args[i] {
		case "--project":
			requireArg(args, i)
			project = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			workflow = args[i]
			i++
		}
	}
	if workflow == "" || project == "" {
		die("Usage: crossagent move <workflow> --project <project>")
	}

	d := state.WorkflowDir(workflow)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		die(fmt.Sprintf("Workflow '%s' not found.", workflow))
	}
	if !state.ProjectExists(project) {
		die(fmt.Sprintf("Project '%s' not found.", project))
	}

	currentProject := confStr(d, "project", "default")
	if currentProject == project {
		info(fmt.Sprintf("Workflow '%s' is already in project '%s'.", workflow, project))
		return
	}

	if err := state.MoveWorkflow(workflow, project); err != nil {
		die(err.Error())
	}
	success(fmt.Sprintf("Moved workflow '%s' from '%s' to '%s'.", workflow, currentProject, project))
}

// ── Phase Cmd ───────────────────────────────────────────────────────────────

func cmdPhaseCmd(args []string) {
	phaseArg := ""
	jsonMode := false
	force := false
	implPhase := 1
	workflow := ""

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--json":
			jsonMode = true
			i++
		case "--force":
			force = true
			i++
		case "--phase":
			requireArg(args, i)
			var err error
			implPhase, err = strconv.Atoi(args[i+1])
			if err != nil || implPhase < 1 {
				die("Implementation phase must be a positive integer.")
			}
			i += 2
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			phaseArg = args[i]
			i++
		}
	}

	if phaseArg == "" {
		die("Usage: crossagent phase-cmd <plan|review|implement|verify> [--json] [--phase N] [--workflow <name>]")
	}

	// Bash only accepts named phases, not numeric ones.
	switch phaseArg {
	case "plan", "review", "implement", "impl", "verify":
		// valid
	default:
		die(fmt.Sprintf("Unknown phase: %s. Use: plan, review, implement, verify", phaseArg))
	}

	name, d := resolveWorkflow(workflow, jsonMode)

	result, err := agent.BuildPhaseCmd(d, name, phaseArg, force, implPhase)
	if err != nil {
		die(err.Error())
	}

	if jsonMode {
		if err := printPhaseCmdJSON(result); err != nil {
			die(err.Error())
		}
		return
	}

	fmt.Printf("agent: %s (%s)\n", result.Agent.Name, result.Agent.DisplayName)
	fmt.Printf("tool: %s\n", result.Command)
	fmt.Printf("cwd: %s\n", result.Cwd)
	fmt.Printf("prompt: %s\n", result.Prompt)
	fmt.Printf("prompt_file: %s\n", result.PromptFile)
	if result.OutputFile != nil {
		fmt.Printf("output_file: %s\n", *result.OutputFile)
	}
	fmt.Printf("phase: %d (%s)\n", result.Phase, strings.ToUpper(result.PhaseLabel))
	fmt.Printf("args: %s\n", strings.Join(result.Args, " "))
}

// ── Revert ──────────────────────────────────────────────────────────────────

func cmdRevert(args []string) {
	target := ""
	reason := ""
	jsonMode := false
	workflow := ""

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--json":
			jsonMode = true
			i++
		case "--reason":
			requireArg(args, i)
			reason = args[i+1]
			i += 2
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			if strings.HasPrefix(args[i], "-") {
				die(fmt.Sprintf("Unknown option: %s", args[i]))
			}
			target = args[i]
			i++
		}
	}

	if target == "" {
		die("Usage: crossagent revert <target_phase> [--reason <text>] [--workflow <name>]")
	}
	targetNum, err := strconv.Atoi(target)
	if err != nil || targetNum < 1 || targetNum > 4 {
		die("Target phase must be 1-4.")
	}

	_, d := resolveWorkflow(workflow, jsonMode)
	phase, _ := state.GetPhase(d)
	pn := state.PhaseNum(phase)

	if pn <= targetNum {
		die(fmt.Sprintf("Cannot revert: current phase (%s) is not ahead of target (%s).", phase, target))
	}

	retryCount := confInt(d, "retry_count", 0)
	maxRetries := confInt(d, "max_retries", 10)

	if retryCount >= maxRetries {
		if jsonMode {
			out := cli.RevertJSON{
				Action:     "needs_human",
				Reason:     fmt.Sprintf("Retry limit reached (%d/%d)", retryCount, maxRetries),
				RetryCount: retryCount,
				MaxRetries: maxRetries,
			}
			cli.PrintJSONCompact(out)
			return
		}
		die(fmt.Sprintf("Retry limit reached (%d/%d). Resolve issues manually.", retryCount, maxRetries))
	}

	retryCount++
	attempt := retryCount

	// Archive artifacts and chat history from target phase through phase 4
	phaseKeys := [5]string{"", "plan", "review", "implement", "verify"}
	for pi := targetNum; pi <= 4; pi++ {
		key := phaseKeys[pi]
		artifact := filepath.Join(d, key+".md")
		if fileExists(artifact) {
			os.Rename(artifact, filepath.Join(d, fmt.Sprintf("%s.attempt-%d.md", key, attempt)))
		}
		chatLog := filepath.Join(d, "chat-history", key+".log")
		if fileExists(chatLog) {
			os.Rename(chatLog, filepath.Join(d, "chat-history", fmt.Sprintf("%s.attempt-%d.log", key, attempt)))
		}
	}

	// Find source artifact for revert context
	sourceArtifact := ""
	sourceLabel := ""
	verifyAttempt := filepath.Join(d, fmt.Sprintf("verify.attempt-%d.md", attempt))
	reviewAttempt := filepath.Join(d, fmt.Sprintf("review.attempt-%d.md", attempt))
	if fileExists(verifyAttempt) {
		sourceArtifact = verifyAttempt
		sourceLabel = "verification report"
	} else if fileExists(filepath.Join(d, "verify.md")) {
		sourceArtifact = filepath.Join(d, "verify.md")
		sourceLabel = "verification report"
	} else if fileExists(reviewAttempt) {
		sourceArtifact = reviewAttempt
		sourceLabel = "review feedback"
	} else if fileExists(filepath.Join(d, "review.md")) {
		sourceArtifact = filepath.Join(d, "review.md")
		sourceLabel = "review feedback"
	}

	extractedIssues := ""
	if sourceArtifact != "" {
		data, err := os.ReadFile(sourceArtifact)
		if err == nil {
			extractedIssues = string(data)
		}
	}

	// Write revert context
	os.MkdirAll(filepath.Join(d, "prompts"), 0755)
	ctx := filepath.Join(d, "prompts", "revert-context.md")

	reasonText := reason
	if reasonText == "" {
		reasonText = "The previous attempt did not pass review/verification."
	}

	var ctxContent strings.Builder
	fmt.Fprintf(&ctxContent, "# Revert Context (Attempt %d)\n\n", attempt)
	fmt.Fprintf(&ctxContent, "## Why This Phase Is Being Re-run\n%s\n\n", reasonText)
	ctxContent.WriteString("## IMPORTANT: Surgical Fix Required\n")
	ctxContent.WriteString("This is a **targeted retry**, NOT a full redo. You must:\n")
	ctxContent.WriteString("1. Read the specific issues listed below\n")
	ctxContent.WriteString("2. Make **only** the changes needed to address those issues\n")
	ctxContent.WriteString("3. Do NOT rewrite or redo work that was already correct\n")
	ctxContent.WriteString("4. Preserve all existing work that is not related to the issues\n")

	if extractedIssues != "" {
		fmt.Fprintf(&ctxContent, "\n## Issues to Address (from %s)\n", sourceLabel)
		fmt.Fprintf(&ctxContent, "The following is the complete %s from the previous attempt.\n", sourceLabel)
		ctxContent.WriteString("Focus on sections marked as issues, failures, or requiring changes:\n\n")
		ctxContent.WriteString("---\n")
		ctxContent.WriteString(extractedIssues)
		ctxContent.WriteString("---\n")
	}

	// List archived files
	hasArchived := false
	for pi := targetNum; pi <= 4; pi++ {
		key := phaseKeys[pi]
		archived := filepath.Join(d, fmt.Sprintf("%s.attempt-%d.md", key, attempt))
		if fileExists(archived) {
			if !hasArchived {
				ctxContent.WriteString("\n## Previous Attempt Artifacts\n")
				hasArchived = true
			}
			fmt.Fprintf(&ctxContent, "- `%s`\n", archived)
		}
	}

	fmt.Fprintf(&ctxContent, "\n## Surgical Fix Guidelines\n")
	fmt.Fprintf(&ctxContent, "- Identify each specific issue from the %s above\n", sourceLabel)
	ctxContent.WriteString("- For each issue, make the minimum change required to resolve it\n")
	ctxContent.WriteString("- Do NOT refactor, restructure, or rewrite code that was not flagged\n")
	fmt.Fprintf(&ctxContent, "- If the %s mentions specific files and line numbers, go directly to those locations\n", sourceLabel)
	ctxContent.WriteString("- After fixing, verify your changes resolve each listed issue\n")
	ctxContent.WriteString("- Document what you fixed and how in your output file\n")

	os.WriteFile(ctx, []byte(ctxContent.String()), 0644)

	// Update state
	state.SetPhase(d, target)
	state.SetConf(d, "retry_count", strconv.Itoa(retryCount))

	if jsonMode {
		out := cli.RevertJSON{
			Action:        "reverted",
			TargetPhase:   targetNum,
			TargetLabel:   state.PhaseLabel(target),
			Attempt:       attempt,
			RetryCount:    retryCount,
			MaxRetries:    maxRetries,
			RevertContext: ctx,
		}
		cli.PrintJSON(out)
	} else {
		success(fmt.Sprintf("Reverted to phase %d — %s (attempt %d/%d)",
			targetNum, strings.ToUpper(state.PhaseLabel(target)), attempt, maxRetries))
		info(fmt.Sprintf("Revert context written to: %s", ctx))
	}
}

// ── Supervise ───────────────────────────────────────────────────────────────

func cmdSupervise(args []string) {
	jsonMode := false
	supervisePhase := ""
	workflow := ""

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--json":
			jsonMode = true
			i++
		case "--phase":
			requireArg(args, i)
			supervisePhase = args[i+1]
			i += 2
		case "--workflow":
			requireArg(args, i)
			workflow = args[i+1]
			i += 2
		default:
			i++
		}
	}

	_, d := resolveWorkflow(workflow, jsonMode)

	retryCount := confInt(d, "retry_count", 0)
	maxRetries := confInt(d, "max_retries", 10)

	if supervisePhase == "" {
		if fileExists(filepath.Join(d, "verify.md")) {
			supervisePhase = "verify"
		} else if fileExists(filepath.Join(d, "review.md")) {
			supervisePhase = "review"
		} else {
			if jsonMode {
				fmt.Println(`{"action":"waiting","reason":"No artifact to supervise"}`)
			} else {
				info("No artifact to supervise.")
			}
			return
		}
	}

	switch supervisePhase {
	case "verify":
		superviseVerify(d, jsonMode, retryCount, maxRetries)
	case "review":
		superviseReview(d, jsonMode, retryCount, maxRetries)
	default:
		if jsonMode {
			fmt.Printf(`{"action":"pass","reason":"No supervision needed for phase: %s"}`+"\n", supervisePhase)
		} else {
			info(fmt.Sprintf("No supervision defined for phase: %s", supervisePhase))
		}
	}
}

func superviseVerify(d string, jsonMode bool, retryCount, maxRetries int) {
	verifyFile := filepath.Join(d, "verify.md")
	if !fileExists(verifyFile) {
		if jsonMode {
			fmt.Println(`{"action":"waiting","reason":"verify.md not found"}`)
		} else {
			info("Verify artifact not found.")
		}
		return
	}

	verdict, status, rec, _ := judge.JudgeVerify(verifyFile)

	switch verdict {
	case judge.Pass:
		state.SetPhase(d, "done")
		if jsonMode {
			out := cli.SuperviseJSON{
				Action:         "done",
				Verdict:        string(verdict),
				Status:         status,
				Recommendation: rec,
			}
			cli.PrintJSONCompact(out)
		} else {
			success("Verification passed — workflow complete!")
		}
	case judge.Fix:
		reason := fmt.Sprintf("Verifier recommendation: %s. Status: %s. Surgically fix the issues flagged in verify.md — do NOT re-implement from scratch.", rec, status)
		revertArgs := []string{"3", "--reason", reason}
		if jsonMode {
			revertArgs = append(revertArgs, "--json")
		} else {
			warn("Verification found issues. Reverting to implement for surgical fix.")
		}
		cmdRevert(revertArgs)
	case judge.Rework:
		reason := fmt.Sprintf("Verifier recommendation: %s. Status: %s. The approach needs fundamental rework — revise the plan addressing the verifier's concerns.", rec, status)
		revertArgs := []string{"1", "--reason", reason}
		if jsonMode {
			revertArgs = append(revertArgs, "--json")
		} else {
			warn("Verification requires rework. Reverting to plan.")
		}
		cmdRevert(revertArgs)
	default:
		if jsonMode {
			out := cli.SuperviseJSON{
				Action:         "unknown",
				Verdict:        string(verdict),
				Status:         status,
				Recommendation: rec,
				RetryCount:     retryCount,
				MaxRetries:     maxRetries,
			}
			cli.PrintJSONCompact(out)
		} else {
			warn("Could not determine verdict from verify.md.")
			info(fmt.Sprintf("Status: %s", status))
			info(fmt.Sprintf("Recommendation: %s", rec))
		}
	}
}

func superviseReview(d string, jsonMode bool, retryCount, maxRetries int) {
	reviewFile := filepath.Join(d, "review.md")
	if !fileExists(reviewFile) {
		if jsonMode {
			fmt.Println(`{"action":"waiting","reason":"review.md not found"}`)
		} else {
			info("Review artifact not found.")
		}
		return
	}

	verdict, reviewVerdict, _ := judge.JudgeReview(reviewFile)

	switch verdict {
	case judge.Approve, judge.ApproveWithChanges:
		if jsonMode {
			out := cli.SuperviseJSON{
				Action:        "pass",
				Verdict:       string(verdict),
				ReviewVerdict: reviewVerdict,
			}
			cli.PrintJSONCompact(out)
		} else {
			if verdict == judge.Approve {
				success("Review approved the plan.")
			} else {
				success("Review approved with changes. Implementation should address the feedback.")
			}
		}
	case judge.Rework:
		reason := fmt.Sprintf("Reviewer verdict: %s. The reviewer requests the plan be reworked. Surgically update the plan addressing the specific issues raised in review.md — do NOT rewrite the entire plan from scratch.", reviewVerdict)
		revertArgs := []string{"1", "--reason", reason}
		if jsonMode {
			revertArgs = append(revertArgs, "--json")
		} else {
			warn("Review requests rework. Reverting to plan for surgical update.")
		}
		cmdRevert(revertArgs)
	default:
		if jsonMode {
			out := cli.SuperviseJSON{
				Action:        "unknown",
				Verdict:       string(verdict),
				ReviewVerdict: reviewVerdict,
				RetryCount:    retryCount,
				MaxRetries:    maxRetries,
			}
			cli.PrintJSONCompact(out)
		} else {
			warn("Could not determine verdict from review.md.")
			info(fmt.Sprintf("Verdict: %s", reviewVerdict))
		}
	}
}

// ── Memory Subcommands ──────────────────────────────────────────────────────

func cmdMemory(args []string) {
	subcmd := "show"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		subcmd = args[0]
		args = args[1:]
	}

	globalFlag := false
	projectFlag := false
	projectName := ""
	jsonMode := false

	i := 0
	for i < len(args) {
		switch args[i] {
		case "--global":
			globalFlag = true
			i++
		case "--project":
			projectFlag = true
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				projectName = args[i+1]
				i += 2
			} else {
				i++
			}
		case "--json":
			jsonMode = true
			i++
		case "-h", "--help":
			fmt.Println("Usage: crossagent memory [show|list|edit] [--global|--project [name]] [--json]")
			os.Exit(0)
		default:
			i++
		}
	}

	// Resolve project name for --project flag
	if projectFlag && projectName == "" {
		_, d, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		projectName = confStr(d, "project", "default")
	}
	if projectFlag && !state.ProjectExists(projectName) {
		die(fmt.Sprintf("Project '%s' not found.", projectName))
	}

	switch subcmd {
	case "show":
		cmdMemoryShow(globalFlag, projectFlag, projectName, jsonMode)
	case "list":
		cmdMemoryList(globalFlag, projectFlag, projectName, jsonMode)
	case "edit":
		cmdMemoryEdit(globalFlag, projectFlag, projectName)
	default:
		die(fmt.Sprintf("Unknown memory command: %s. Use: show, list, edit", subcmd))
	}
}

func cmdMemoryShow(globalFlag, projectFlag bool, projectName string, jsonMode bool) {
	if projectFlag {
		projMemDir := state.ProjectMemoryDir(projectName)
		if jsonMode {
			files := cli.NewOrderedFileMap()
			memFiles, _ := state.ListProjectMemoryFiles(projectName)
			for _, fpath := range memFiles {
				relName, _ := filepath.Rel(projMemDir, fpath)
				data, _ := os.ReadFile(fpath)
				files.Set(relName, cli.MemoryFileJSON{Path: fpath, Content: string(data)})
			}
			out := cli.MemoryShowJSON{Type: "project", Name: projectName, Files: files}
			cli.PrintJSONCompact(out)
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  %sProject Memory: %s%s\n", BOLD, projectName, NC)
			separator()
			memFiles, _ := state.ListProjectMemoryFiles(projectName)
			if len(memFiles) == 0 {
				fmt.Fprintf(os.Stderr, "  %sNo memory files.%s\n", DIM, NC)
			} else {
				for _, fpath := range memFiles {
					relName, _ := filepath.Rel(projMemDir, fpath)
					fmt.Fprintln(os.Stderr)
					fmt.Fprintf(os.Stderr, "  %s%s%s\n", BOLD, relName, NC)
					fmt.Fprintf(os.Stderr, "  %s%s%s\n", DIM, fpath, NC)
					fmt.Fprintln(os.Stderr)
					data, _ := os.ReadFile(fpath)
					for _, line := range strings.Split(string(data), "\n") {
						fmt.Fprintf(os.Stderr, "    %s\n", line)
					}
					fmt.Fprintln(os.Stderr)
					separator()
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	} else if globalFlag {
		memDir := state.GlobalMemoryDir()
		if jsonMode {
			gcContent := readFileStr(filepath.Join(memDir, "global-context.md"))
			llContent := readFileStr(filepath.Join(memDir, "lessons-learned.md"))
			files := cli.NewOrderedFileMap()
			files.Set("global-context.md", cli.MemoryFileJSON{Path: filepath.Join(memDir, "global-context.md"), Content: gcContent})
			files.Set("lessons-learned.md", cli.MemoryFileJSON{Path: filepath.Join(memDir, "lessons-learned.md"), Content: llContent})
			out := cli.MemoryShowJSON{Type: "global", Files: files}
			cli.PrintJSONCompact(out)
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  %sGlobal Memory%s\n", BOLD, NC)
			separator()
			entries, _ := os.ReadDir(memDir)
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					fpath := filepath.Join(memDir, e.Name())
					fmt.Fprintln(os.Stderr)
					fmt.Fprintf(os.Stderr, "  %s%s%s\n", BOLD, e.Name(), NC)
					fmt.Fprintf(os.Stderr, "  %s%s%s\n", DIM, fpath, NC)
					fmt.Fprintln(os.Stderr)
					data, _ := os.ReadFile(fpath)
					for _, line := range strings.Split(string(data), "\n") {
						fmt.Fprintf(os.Stderr, "    %s\n", line)
					}
					fmt.Fprintln(os.Stderr)
					separator()
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	} else {
		name, d, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		memPath := filepath.Join(d, "memory.md")
		if jsonMode {
			out := cli.MemoryShowJSON{Type: "workflow", Name: name, Path: memPath}
			if fileExists(memPath) {
				content := readFileStr(memPath)
				out.Content = &content
			}
			cli.PrintJSONCompact(out)
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  %sWorkflow Memory: %s%s\n", BOLD, name, NC)
			separator()
			if fileExists(memPath) {
				fmt.Fprintln(os.Stderr)
				data, _ := os.ReadFile(memPath)
				for _, line := range strings.Split(string(data), "\n") {
					fmt.Fprintf(os.Stderr, "    %s\n", line)
				}
				fmt.Fprintln(os.Stderr)
			} else {
				fmt.Fprintf(os.Stderr, "  %sNo memory file yet.%s\n", DIM, NC)
			}
			fmt.Fprintln(os.Stderr)
		}
	}
}

func cmdMemoryList(globalFlag, projectFlag bool, projectName string, jsonMode bool) {
	if projectFlag {
		projMemDir := state.ProjectMemoryDir(projectName)
		if jsonMode {
			memFiles, _ := state.ListProjectMemoryFiles(projectName)
			entries := make([]cli.MemoryListEntry, 0, len(memFiles))
			for _, fpath := range memFiles {
				relName, _ := filepath.Rel(projMemDir, fpath)
				entries = append(entries, cli.MemoryListEntry{
					Name:  relName,
					Path:  fpath,
					Lines: countLines(fpath),
				})
			}
			out := cli.MemoryListJSON{Type: "project", Name: projectName, Files: entries}
			cli.PrintJSONCompact(out)
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  %sProject Memory Files: %s%s\n", BOLD, projectName, NC)
			separator()
			memFiles, _ := state.ListProjectMemoryFiles(projectName)
			if len(memFiles) == 0 {
				fmt.Fprintf(os.Stderr, "  %s(none)%s\n", DIM, NC)
			} else {
				for _, fpath := range memFiles {
					relName, _ := filepath.Rel(projMemDir, fpath)
					fmt.Fprintf(os.Stderr, "  %s  %s(%d lines)%s\n", relName, DIM, countLines(fpath), NC)
				}
			}
			fmt.Fprintln(os.Stderr)
		}
	} else if globalFlag {
		memDir := state.GlobalMemoryDir()
		if jsonMode {
			entries := make([]cli.MemoryListEntry, 0)
			dirEntries, _ := os.ReadDir(memDir)
			for _, e := range dirEntries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					fpath := filepath.Join(memDir, e.Name())
					entries = append(entries, cli.MemoryListEntry{
						Name:  e.Name(),
						Path:  fpath,
						Lines: countLines(fpath),
					})
				}
			}
			out := cli.MemoryListJSON{Type: "global", Files: entries}
			cli.PrintJSONCompact(out)
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  %sGlobal Memory Files%s\n", BOLD, NC)
			separator()
			dirEntries, _ := os.ReadDir(memDir)
			found := false
			for _, e := range dirEntries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
					found = true
					fpath := filepath.Join(memDir, e.Name())
					fmt.Fprintf(os.Stderr, "  %s  %s(%d lines)%s\n", e.Name(), DIM, countLines(fpath), NC)
				}
			}
			if !found {
				fmt.Fprintf(os.Stderr, "  %s(none)%s\n", DIM, NC)
			}
			fmt.Fprintln(os.Stderr)
		}
	} else {
		name, d, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		if jsonMode {
			entries := make([]cli.MemoryListEntry, 0)
			memPath := filepath.Join(d, "memory.md")
			if fileExists(memPath) {
				entries = append(entries, cli.MemoryListEntry{
					Name:  "memory.md",
					Path:  memPath,
					Lines: countLines(memPath),
				})
			}
			out := cli.MemoryListJSON{Type: "workflow", Name: name, Files: entries}
			cli.PrintJSONCompact(out)
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "  %sMemory Files: %s%s\n", BOLD, name, NC)
			separator()
			memPath := filepath.Join(d, "memory.md")
			if fileExists(memPath) {
				fmt.Fprintf(os.Stderr, "  memory.md  %s(%d lines)%s\n", DIM, countLines(memPath), NC)
			} else {
				fmt.Fprintf(os.Stderr, "  %s(none)%s\n", DIM, NC)
			}
			fmt.Fprintln(os.Stderr)
		}
	}
}

func cmdMemoryEdit(globalFlag, projectFlag bool, projectName string) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	var target string
	if projectFlag {
		target = filepath.Join(state.ProjectMemoryDir(projectName), "project-context.md")
	} else if globalFlag {
		target = filepath.Join(state.GlobalMemoryDir(), "global-context.md")
	} else {
		name, d, err := cli.RequireWorkflow()
		if err != nil {
			die(err.Error())
		}
		target = filepath.Join(d, "memory.md")
		if !fileExists(target) {
			die(fmt.Sprintf("No memory file for workflow '%s'.", name))
		}
	}

	cmd := exec.Command(editor, target)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		die(fmt.Sprintf("Editor exited with error: %v", err))
	}
}

// ── Usage ───────────────────────────────────────────────────────────────────

func usage() {
	fmt.Printf(`
  crossagent — Cross-Model AI Agent Orchestrator v%s`, displayVersion())
	fmt.Print(`

  WORKFLOW COMMANDS
    new <name> [opts]     Create a new workflow
      --repo <path>         Target repository (default: cwd)
      --add-dir <path>      Additional repo directories (repeatable)
      --project <name>      Parent project (default: default)

    plan [--force]        Phase 1: Plan with Claude Code
    review [--force]      Phase 2: Review plan with Codex CLI
    implement [opts]      Phase 3: Implement with Claude Code
      --phase <n>           Implementation sub-phase (default: 1)
    verify [--force]      Phase 4: Verify with Codex CLI

  NAVIGATION
    next                  Run the next pending phase
    status                Show current workflow state
    list                  List all workflows
    use <name>            Switch active workflow
    advance               Manually advance to next phase
    done                  Mark workflow as complete

  PROJECTS
    projects list         List all projects
    projects new <name>   Create a new project
    projects delete <name> Delete a project (moves workflows to default)
    projects show <name>  Show project details and workflows
    projects rename <o> <n> Rename a project
    projects suggest      Suggest a project for a description
      --description <text>  Description to match against
      --json                JSON output
    move <wf> --project <p> Move a workflow to a different project

  REPOSITORIES
    repos list            List repositories for active workflow
    repos add <path>      Add an additional repository
    repos remove <path>   Remove an additional repository
    repos set-primary     Change the primary repository (cwd)

  AGENTS
    agents list           List builtin and custom static agents
    agents add            Register a custom static agent
    agents remove         Remove a custom static agent
    agents show           Show per-phase agent assignments
    agents assign         Assign an agent to a workflow phase
    agents reset          Reset a workflow phase to its default agent

  INTEGRATION (for UI companion / scripting)
    status --json         Machine-readable workflow state
    list --json           Machine-readable workflow list
    phase-cmd <phase>     Get launch parameters for a phase (does not spawn)
      --json                JSON output (default: plain text)
      --force               Allow launch config for already-completed phases
      --phase <n>           Sub-phase for implement (default: 1)

  MEMORY
    memory show           Show workflow memory (default)
    memory show --global  Show global memory files
    memory show --project [name] Show project memory files
    memory list           List memory files for current workflow
    memory list --global  List global memory files
    memory list --project [name] List project memory files
    memory edit           Edit workflow memory in $EDITOR
    memory edit --global  Edit global context in $EDITOR
    memory edit --project [name] Edit project context in $EDITOR
      --json              JSON output for show/list

  WEB UI
    serve [opts]          Start the web UI server
      --port <port>         Listen port (default: 3456, or CROSSAGENT_PORT)
      --open                Open browser automatically

  UTILITIES
    log                   Display all artifacts (plan, review, report)
    open                  Open workflow directory in Finder
    reset <name>          Delete a workflow and its artifacts
    help                  Show this help
    version               Show version

  WORKFLOW
    ┌────────┐  plan.md  ┌────────┐  review.md  ┌────────┐ changes  ┌────────┐
    │ 1.PLAN ├──────────►│2.REVIEW├────────────►│3.IMPL  ├─────────►│4.VERIFY│
    │ Claude │           │ Codex  │             │ Claude │          │ Codex  │
    └────────┘           └────────┘             └────────┘          └────────┘

  EXAMPLES
    crossagent new auth-refactor --repo ~/projects/api
    crossagent new cache-layer --repo ~/proj/svc --add-dir ~/proj/shared
    crossagent repos list
    crossagent repos add ~/projects/shared-lib
    crossagent repos set-primary ~/projects/api-v2
    crossagent agents add reviewer-alt --adapter codex --command codex
    crossagent agents assign review reviewer-alt
    crossagent plan
    crossagent review
    crossagent implement --phase 1
    crossagent implement --phase 2
    crossagent verify
`)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	if len(data) == 0 {
		return 0
	}
	return strings.Count(string(data), "\n")
}

func readFileStr(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	// Strip trailing newlines to match bash command substitution behavior.
	// Bash: content=$(cat file) strips trailing newlines before JSON embedding.
	return strings.TrimRight(string(data), "\n")
}

// printPhaseCmdJSON writes phase-cmd JSON in the exact hybrid format matching bash CLI:
// top-level keys indented 2 spaces, nested agent object and args array compact.
func printPhaseCmdJSON(r *agent.PhaseCmdResult) error {
	mc := func(v any) string {
		b, _ := cli.MarshalCompact(v)
		return string(b)
	}
	js := func(s string) string {
		b, _ := cli.MarshalCompact(s)
		return string(b)
	}

	outFile := "null"
	if r.OutputFile != nil {
		outFile = js(*r.OutputFile)
	}

	var buf bytes.Buffer
	buf.WriteString("{\n")
	fmt.Fprintf(&buf, "  \"agent\": %s,\n", mc(r.Agent))
	fmt.Fprintf(&buf, "  \"command\": %s,\n", js(r.Command))
	fmt.Fprintf(&buf, "  \"args\": %s,\n", mc(r.Args))
	fmt.Fprintf(&buf, "  \"cwd\": %s,\n", js(r.Cwd))
	fmt.Fprintf(&buf, "  \"prompt\": %s,\n", js(r.Prompt))
	fmt.Fprintf(&buf, "  \"prompt_file\": %s,\n", js(r.PromptFile))
	fmt.Fprintf(&buf, "  \"output_file\": %s,\n", outFile)
	fmt.Fprintf(&buf, "  \"phase\": %d,\n", r.Phase)
	fmt.Fprintf(&buf, "  \"phase_label\": %s,\n", js(r.PhaseLabel))
	fmt.Fprintf(&buf, "  \"workflow\": %s,\n", js(r.Workflow))
	fmt.Fprintf(&buf, "  \"workflow_dir\": %s\n", js(r.WorkflowDir))
	buf.WriteString("}\n")

	_, err := os.Stdout.Write(buf.Bytes())
	return err
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

// flagStr extracts the value of a --key <value> flag from args, returning ""
// if not present. Used by --workflow to let commands target a specific workflow.
func flagStr(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// resolveWorkflow returns (name, dir) for the given workflow name, or from the
// global current file if name is empty. Dies on error.
func resolveWorkflow(name string, jsonMode bool) (string, string) {
	if name == "" {
		var err error
		name, err = state.GetCurrent()
		if err != nil {
			die(err.Error())
		}
		if name == "" {
			if jsonMode {
				fmt.Println(`{"error":"no active workflow"}`)
				os.Exit(1)
			}
			die(fmt.Sprintf("No active workflow. Run %scrossagent new <name>%s", BOLD, NC))
		}
	}
	d := state.WorkflowDir(name)
	if _, err := os.Stat(d); os.IsNotExist(err) {
		if jsonMode {
			fmt.Println(`{"error":"workflow dir missing"}`)
			os.Exit(1)
		}
		die(fmt.Sprintf("Workflow dir missing: %s", d))
	}
	return name, d
}

func requireArg(args []string, i int) {
	if i+1 >= len(args) {
		die(fmt.Sprintf("Option %s requires an argument.", args[i]))
	}
}

func readConfigOrDie(d string) *state.Config {
	cfg, err := state.ReadConfig(d)
	if err != nil {
		die(fmt.Sprintf("Failed to read config: %v", err))
	}
	return cfg
}

// getPhaseAgentOrDie returns the *Agent for a phase. Dies on error.
func getPhaseAgentOrDie(d, phase string) *agent.Agent {
	ag, err := agent.GetPhaseAgent(d, phase)
	if err != nil {
		die(err.Error())
	}
	return ag
}

// phaseAgentName returns just the agent name string for a phase. Safe (returns default on error).
func phaseAgentName(d, phase string) string {
	ag, err := agent.GetPhaseAgent(d, phase)
	if err != nil {
		return agent.DefaultPhaseAgent(phase)
	}
	return ag.Name
}

func confInt(d, key string, defaultVal int) int {
	v, err := state.GetConf(d, key)
	if err != nil || v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func confStr(d, key, defaultVal string) string {
	v, err := state.GetConf(d, key)
	if err != nil || v == "" {
		return defaultVal
	}
	return v
}

func cleanAddDirs(addDirs []string) []string {
	result := make([]string, 0)
	for _, d := range addDirs {
		if d != "" {
			result = append(result, d)
		}
	}
	return result
}

func appendAddDir(addDirs []string, path string) []string {
	result := cleanAddDirs(addDirs)
	result = append(result, path)
	return result
}

func makeArtifact(path string) cli.ArtifactJSON {
	if fileExists(path) {
		return cli.ArtifactJSON{Exists: true, Path: path, Lines: countLines(path)}
	}
	return cli.ArtifactJSON{Exists: false, Path: path}
}

func makeChatHistoryEntry(path string) cli.ChatHistoryEntry {
	info, err := os.Stat(path)
	if err != nil {
		return cli.ChatHistoryEntry{Exists: false, Path: path}
	}
	return cli.ChatHistoryEntry{Exists: true, Path: path, Size: info.Size()}
}

func launchAgentOrDie(ag *agent.Agent, repo, promptFile, wfDir string, addDirs []string) {
	cfg := readConfigOrDie(wfDir)
	projectMemDir := ""
	if cfg.Project != "" {
		projectMemDir = state.ProjectMemoryDir(cfg.Project)
	}
	err := agent.LaunchAgent(ag, repo, promptFile, wfDir, addDirs, projectMemDir)
	// Match bash: don't propagate exit code as fatal (|| true)
	_ = err
}
