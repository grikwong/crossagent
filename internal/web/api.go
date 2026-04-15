package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/grikwong/crossagent/internal/agent"
	"github.com/grikwong/crossagent/internal/state"
)

// AppVersion is set by the CLI entry point before serving. It is returned
// by the /api/version endpoint so the frontend can display the current version.
var AppVersion = "dev"

// ── Validation ──────────────────────────────────────────────────────────────

var (
	validPhases    = map[string]bool{"plan": true, "review": true, "implement": true, "verify": true}
	validArtifacts = map[string]bool{"plan": true, "review": true, "implement": true, "verify": true, "memory": true}
	nameRe         = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
)

func validateName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	return nameRe.MatchString(name)
}

// ── CLI helpers ─────────────────────────────────────────────────────────────
// The web handlers shell out to the crossagent binary for operations that
// produce complex JSON output. This guarantees identical JSON shapes between
// CLI and web API without duplicating formatting logic.

var crossagentBin string

func init() {
	// Resolve at init; overridden in tests.
	bin, err := os.Executable()
	if err != nil {
		bin = "crossagent"
	}
	crossagentBin = bin
}

func runCLI(args ...string) ([]byte, error) {
	cmd := exec.Command(crossagentBin, args...)
	cmd.Env = append(os.Environ(), "CROSSAGENT_HOME="+state.Home())
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func runCLIWithStdin(input string, args ...string) ([]byte, error) {
	cmd := exec.Command(crossagentBin, args...)
	cmd.Env = append(os.Environ(), "CROSSAGENT_HOME="+state.Home())
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

// ── JSON helpers ────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func writeJSONObj(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func readBody(r *http.Request) map[string]any {
	var body map[string]any
	data, err := io.ReadAll(r.Body)
	if err != nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, &body); err != nil {
		return nil
	}
	return body
}

func bodyStr(body map[string]any, key string) string {
	if body == nil {
		return ""
	}
	v, ok := body[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func bodyStrSlice(body map[string]any, key string) []string {
	if body == nil {
		return nil
	}
	v, ok := body[key]
	if !ok {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// ── Handlers ────────────────────────────────────────────────────────────────

// GET /api/status
func handleStatus(w http.ResponseWriter, r *http.Request) {
	out, err := runCLI("status", "--json")
	if err != nil {
		writeJSON(w, []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		return
	}
	writeJSON(w, out)
}

// GET /api/list
func handleList(w http.ResponseWriter, r *http.Request) {
	out, err := runCLI("list", "--json")
	if err != nil {
		writeJSON(w, []byte(fmt.Sprintf(`{"error":%q,"workflows":[],"active":""}`, err.Error())))
		return
	}
	writeJSON(w, out)
}

// GET /api/phase-cmd/{phase}
func handlePhaseCmd(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	args := []string{"phase-cmd", phase, "--json"}
	if sub := r.URL.Query().Get("subphase"); sub != "" {
		// Validate subphase is numeric
		for _, c := range sub {
			if c < '0' || c > '9' {
				writeError(w, 400, "subphase must be a number")
				return
			}
		}
		args = append(args, "--phase", sub)
	}
	if r.URL.Query().Get("force") == "true" {
		args = append(args, "--force")
	}

	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/artifact/{type}
func handleArtifact(w http.ResponseWriter, r *http.Request) {
	artType := r.PathValue("type")
	if !validArtifacts[artType] {
		writeError(w, 400, fmt.Sprintf("Invalid artifact type: %s", artType))
		return
	}

	// Get current workflow dir from status
	statusOut, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	var status map[string]any
	if err := json.Unmarshal(statusOut, &status); err != nil {
		writeError(w, 500, "Failed to parse status")
		return
	}
	if errMsg, ok := status["error"]; ok {
		writeError(w, 404, fmt.Sprintf("%v", errMsg))
		return
	}

	wfDir, _ := status["workflow_dir"].(string)
	wfName, _ := status["name"].(string)
	if wfDir == "" || wfName == "" {
		writeError(w, 404, "No active workflow")
		return
	}

	filePath := filepath.Join(wfDir, artType+".md")
	tryRecoverArtifact(wfName, filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		writeError(w, 404, "Artifact not found")
		return
	}

	writeJSONObj(w, map[string]string{
		"content": string(data),
		"path":    filePath,
	})
}

// POST /api/use/{name}
func handleUse(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validateName(name) {
		writeError(w, 400, "Invalid workflow name")
		return
	}

	if _, err := runCLI("use", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/advance
func handleAdvance(w http.ResponseWriter, r *http.Request) {
	if _, err := runCLI("advance"); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/done
func handleDone(w http.ResponseWriter, r *http.Request) {
	if _, err := runCLI("done"); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/new
func handleNew(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	name := bodyStr(body, "name")
	description := bodyStr(body, "description")
	repo := bodyStr(body, "repo")
	project := bodyStr(body, "project")
	addDirs := bodyStrSlice(body, "addDirs")

	if name == "" || description == "" {
		writeError(w, 400, "name and description required")
		return
	}
	if !validateName(name) {
		writeError(w, 400, "Invalid workflow name. Use alphanumeric characters, hyphens, underscores, and dots.")
		return
	}

	args := []string{"new", name}
	if repo != "" {
		args = append(args, "--repo", repo)
	}
	if project != "" && validateName(project) {
		args = append(args, "--project", project)
	}
	for _, d := range addDirs {
		args = append(args, "--add-dir", d)
	}

	if _, err := runCLIWithStdin(description, args...); err != nil {
		writeError(w, 400, err.Error())
		return
	}

	out, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/update-description
func handleUpdateDescription(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	workflow := bodyStr(body, "workflow")
	appendText := bodyStr(body, "append")

	if workflow == "" || appendText == "" {
		writeError(w, 400, "workflow and append fields are required")
		return
	}
	if !validateName(workflow) {
		writeError(w, 400, "invalid workflow name")
		return
	}

	wfDir := state.WorkflowDir(workflow)
	descPath := filepath.Join(wfDir, "description")

	existing, err := os.ReadFile(descPath)
	if err != nil {
		writeError(w, 404, "workflow description not found")
		return
	}

	updated := strings.TrimRight(string(existing), "\n") + "\n\n---\n\n" + appendText + "\n"

	// Atomic write via temp file + rename
	tmp, err := os.CreateTemp(wfDir, ".tmp-desc-*")
	if err != nil {
		writeError(w, 500, "failed to create temp file")
		return
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(updated); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		writeError(w, 500, "failed to write description")
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		writeError(w, 500, "failed to write description")
		return
	}
	if err := os.Rename(tmpPath, descPath); err != nil {
		os.Remove(tmpPath)
		writeError(w, 500, "failed to update description")
		return
	}

	writeJSONObj(w, map[string]string{"ok": "true"})
}

// ── Project API ─────────────────────────────────────────────────────────────

// GET /api/projects
func handleProjectsList(w http.ResponseWriter, r *http.Request) {
	out, err := runCLI("projects", "list", "--json")
	if err != nil {
		writeJSON(w, []byte(fmt.Sprintf(`{"error":%q,"projects":[]}`, err.Error())))
		return
	}
	writeJSON(w, out)
}

// POST /api/projects/new
func handleProjectsNew(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	name := bodyStr(body, "name")
	if !validateName(name) {
		writeError(w, 400, "Invalid project name")
		return
	}

	if _, err := runCLI("projects", "new", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("projects", "list", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/projects/delete
func handleProjectsDelete(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	name := bodyStr(body, "name")
	if !validateName(name) {
		writeError(w, 400, "Invalid project name")
		return
	}

	if _, err := runCLI("projects", "delete", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("projects", "list", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/projects/{name}
func handleProjectsShow(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validateName(name) {
		writeError(w, 400, "Invalid project name")
		return
	}

	out, err := runCLI("projects", "show", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/projects/rename
func handleProjectsRename(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	oldName := bodyStr(body, "old_name")
	newName := bodyStr(body, "new_name")
	if !validateName(oldName) || !validateName(newName) {
		writeError(w, 400, "Invalid project name(s)")
		return
	}

	if _, err := runCLI("projects", "rename", oldName, newName); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("projects", "list", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/move
func handleMove(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	workflow := bodyStr(body, "workflow")
	project := bodyStr(body, "project")
	if !validateName(workflow) || !validateName(project) {
		writeError(w, 400, "Invalid workflow or project name")
		return
	}

	if _, err := runCLI("move", workflow, "--project", project); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/suggest-project
func handleSuggestProject(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	description := bodyStr(body, "description")
	if description == "" {
		writeError(w, 400, "description required")
		return
	}

	out, err := runCLI("projects", "suggest", "--description", description, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// ── Supervise & Revert ──────────────────────────────────────────────────────

// POST /api/supervise
func handleSupervise(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	args := []string{"supervise", "--json"}
	if phase := bodyStr(body, "phase"); phase != "" {
		args = append(args, "--phase", phase)
	}

	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/revert
func handleRevert(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	targetPhase := bodyStr(body, "target_phase")

	// Validate target_phase is 1-4
	if targetPhase == "" || len(targetPhase) != 1 || targetPhase[0] < '1' || targetPhase[0] > '4' {
		writeError(w, 400, "target_phase must be 1-4")
		return
	}

	args := []string{"revert", targetPhase, "--json"}
	if reason := bodyStr(body, "reason"); reason != "" {
		args = append(args, "--reason", reason)
	}

	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// ── Chat History ────────────────────────────────────────────────────────────

// GET /api/chat-history/{phase}
func handleChatHistory(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	statusOut, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	var status map[string]any
	if err := json.Unmarshal(statusOut, &status); err != nil {
		writeError(w, 500, "Failed to parse status")
		return
	}
	if errMsg, ok := status["error"]; ok {
		writeError(w, 404, fmt.Sprintf("%v", errMsg))
		return
	}

	wfDir, _ := status["workflow_dir"].(string)
	logPath := filepath.Join(wfDir, "chat-history", phase+".log")

	info, err := os.Stat(logPath)
	if err != nil {
		writeJSONObj(w, map[string]bool{"exists": false})
		return
	}

	// Large file: return metadata only
	if info.Size() > 5*1024*1024 {
		writeJSONObj(w, map[string]any{
			"exists": true,
			"path":   logPath,
			"size":   info.Size(),
			"large":  true,
		})
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSONObj(w, map[string]any{
		"exists":  true,
		"content": string(data),
		"path":    logPath,
		"size":    info.Size(),
	})
}

// GET /api/chat-history/{phase}/stream
func handleChatHistoryStream(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	statusOut, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 404, err.Error())
		return
	}
	var status map[string]any
	if err := json.Unmarshal(statusOut, &status); err != nil {
		writeError(w, 500, "Failed to parse status")
		return
	}
	if errMsg, ok := status["error"]; ok {
		writeError(w, 404, fmt.Sprintf("%v", errMsg))
		return
	}

	wfDir, _ := status["workflow_dir"].(string)
	logPath := filepath.Join(wfDir, "chat-history", phase+".log")

	if _, err := os.Stat(logPath); err != nil {
		writeError(w, 404, "Chat history not found")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeFile(w, r, logPath)
}

// ── File Checks ─────────────────────────────────────────────────────────────

// POST /api/check-file
func handleCheckFile(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	filePath := bodyStr(body, "path")
	if filePath == "" {
		writeError(w, 400, "path required")
		return
	}

	_, err := os.Stat(filePath)
	writeJSONObj(w, map[string]bool{"exists": err == nil})
}

// POST /api/check-advance
func handleCheckAdvance(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	outputFile := bodyStr(body, "output_file")
	if outputFile == "" {
		writeError(w, 400, "output_file required")
		return
	}

	if _, err := os.Stat(outputFile); err != nil {
		writeJSONObj(w, map[string]bool{"advanced": false})
		return
	}

	// File exists — advance
	if _, err := runCLI("advance"); err != nil {
		writeError(w, 400, err.Error())
		return
	}

	statusOut, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}

	var statusData any
	json.Unmarshal(statusOut, &statusData)
	writeJSONObj(w, map[string]any{
		"advanced": true,
		"status":   statusData,
	})
}

// ── Repos ───────────────────────────────────────────────────────────────────

// POST /api/repos/add
func handleReposAdd(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	dirPath := bodyStr(body, "path")
	if dirPath == "" {
		writeError(w, 400, "path required")
		return
	}

	if _, err := runCLI("repos", "add", dirPath); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/version
func handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSONObj(w, map[string]string{"version": AppVersion})
}

// POST /api/repos/remove
func handleReposRemove(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	dirPath := bodyStr(body, "path")
	if dirPath == "" {
		writeError(w, 400, "path required")
		return
	}

	if _, err := runCLI("repos", "remove", dirPath); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// ── Workflow-Scoped Handlers ────────────────────────────────────────────────
// These handlers accept a workflow name in the URL path and pass --workflow
// to the CLI, avoiding dependence on the global ~/.crossagent/current file.

// ── Agents handlers ─────────────────────────────────────────────────────────

// GET /api/agents
func handleAgentsList(w http.ResponseWriter, r *http.Request) {
	out, err := runCLI("agents", "list", "--json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/adapters
//
// Returns the list of registered adapters, in registration order:
// [{name, display_name, default_command}, ...]. Consumed by the Web UI
// to populate the adapter dropdown without hardcoding names in HTML.
// Adding a new adapter on the backend automatically surfaces it here.
func handleAdaptersList(w http.ResponseWriter, r *http.Request) {
	names := agent.AdapterNames()
	items := make([]map[string]string, 0, len(names))
	for _, n := range names {
		ad, ok := agent.AdapterFor(n)
		if !ok {
			continue
		}
		items = append(items, map[string]string{
			"name":            ad.Name(),
			"display_name":    ad.DisplayName(),
			"default_command": ad.DefaultCommand(),
		})
	}
	writeJSONObj(w, map[string]any{"adapters": items})
}

// POST /api/agents
// Body: {name, adapter, command, displayName}
func handleAgentsCreate(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	name := bodyStr(body, "name")
	adapter := bodyStr(body, "adapter")
	command := bodyStr(body, "command")
	displayName := bodyStr(body, "displayName")

	if !validateName(name) {
		writeError(w, 400, "Invalid agent name")
		return
	}
	if !agent.ValidAdapter(adapter) {
		writeError(w, 400, fmt.Sprintf("Agent adapter must be one of: %s",
			strings.Join(agent.AdapterNames(), ", ")))
		return
	}

	args := []string{"agents", "add", name, "--adapter", adapter}
	if command != "" {
		args = append(args, "--command", command)
	}
	if displayName != "" {
		args = append(args, "--display-name", displayName)
	}
	if _, err := runCLI(args...); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("agents", "list", "--json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, out)
}

// DELETE /api/agents/{name}
func handleAgentsDelete(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validateName(name) {
		writeError(w, 400, "Invalid agent name")
		return
	}
	if _, err := runCLI("agents", "remove", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("agents", "list", "--json")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/workflow/{name}/agents
func handleWorkflowAgentsGet(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	out, err := runCLI("agents", "show", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/agents
// Body: {phase, agent}
func handleWorkflowAgentsSet(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	phase := bodyStr(body, "phase")
	agentName := bodyStr(body, "agent")
	if !validPhases[phase] {
		writeError(w, 400, "Invalid phase")
		return
	}
	if !validateName(agentName) {
		writeError(w, 400, "Invalid agent name")
		return
	}
	if _, err := runCLI("agents", "assign", phase, agentName, "--workflow", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("agents", "show", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/agents/autoselect
func handleWorkflowAgentsAutoselect(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	out, err := runCLI("agents", "autoselect", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// requireWorkflowName extracts and validates the {name} path parameter.
func requireWorkflowName(w http.ResponseWriter, r *http.Request) (string, bool) {
	name := r.PathValue("name")
	if !validateName(name) {
		writeError(w, 400, "Invalid workflow name")
		return "", false
	}
	return name, true
}

// GET /api/workflow/{name}/status
func handleWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	out, err := runCLI("status", "--workflow", name, "--json")
	if err != nil {
		writeJSON(w, []byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		return
	}
	writeJSON(w, out)
}

// GET /api/workflow/{name}/phase-cmd/{phase}
func handleWorkflowPhaseCmd(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	args := []string{"phase-cmd", phase, "--workflow", name, "--json"}
	if sub := r.URL.Query().Get("subphase"); sub != "" {
		for _, c := range sub {
			if c < '0' || c > '9' {
				writeError(w, 400, "subphase must be a number")
				return
			}
		}
		args = append(args, "--phase", sub)
	}
	if r.URL.Query().Get("force") == "true" {
		args = append(args, "--force")
	}

	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/workflow/{name}/artifact/{type}
func handleWorkflowArtifact(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	artType := r.PathValue("type")
	if !validArtifacts[artType] {
		writeError(w, 400, fmt.Sprintf("Invalid artifact type: %s", artType))
		return
	}

	wfDir := state.WorkflowDir(name)
	// Support ?attempt=N for retry-attempt artifacts
	var filePath string
	if attemptStr := r.URL.Query().Get("attempt"); attemptStr != "" {
		filePath = filepath.Join(wfDir, artType+".attempt-"+attemptStr+".md")
	} else {
		filePath = filepath.Join(wfDir, artType+".md")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		writeError(w, 404, "Artifact not found")
		return
	}

	writeJSONObj(w, map[string]string{
		"content": string(data),
		"path":    filePath,
	})
}

// POST /api/workflow/{name}/advance
func handleWorkflowAdvance(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	if _, err := runCLI("advance", "--workflow", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/done
func handleWorkflowDone(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	if _, err := runCLI("done", "--workflow", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/supervise
func handleWorkflowSupervise(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	args := []string{"supervise", "--workflow", name, "--json"}
	if phase := bodyStr(body, "phase"); phase != "" {
		args = append(args, "--phase", phase)
	}
	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/revert
func handleWorkflowRevert(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	targetPhase := bodyStr(body, "target_phase")
	if targetPhase == "" || len(targetPhase) != 1 || targetPhase[0] < '1' || targetPhase[0] > '4' {
		writeError(w, 400, "target_phase must be 1-4")
		return
	}
	args := []string{"revert", targetPhase, "--workflow", name, "--json"}
	if reason := bodyStr(body, "reason"); reason != "" {
		args = append(args, "--reason", reason)
	}
	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/check-file
func handleWorkflowCheckFile(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	filePath := bodyStr(body, "path")
	if filePath == "" {
		writeError(w, 400, "path required")
		return
	}

	// Attempt sandbox-fallback recovery: if the expected file is missing
	// but a same-named file exists at the repo root (where an OS-sandboxed
	// agent like gemini would have been forced to write), relocate it into
	// the workflow directory before reporting existence. This unblocks
	// auto-advance when --sandbox / seatbelt denies writes outside CWD.
	recovered, srcPath := tryRecoverArtifact(name, filePath)

	_, err := os.Stat(filePath)
	writeJSONObj(w, map[string]any{
		"exists":         err == nil,
		"recovered":      recovered,
		"recovered_from": srcPath,
	})
}

// tryRecoverArtifact runs sandbox-fallback recovery for a single file
// whose expected path sits inside the named workflow's directory. It
// returns (recovered, srcPath). If the file is outside the workflow
// directory, or the basename is not one of the known recoverable
// artifacts, recovery is skipped and (false, "") is returned.
func tryRecoverArtifact(workflowName, expectedPath string) (bool, string) {
	wfDir := state.WorkflowDir(workflowName)
	// Only attempt recovery for files we expect to live inside the
	// workflow directory — guards against accidentally moving unrelated
	// files into the workflow dir.
	rel, err := filepath.Rel(wfDir, expectedPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel != filepath.Base(expectedPath) {
		return false, ""
	}
	cfg, err := state.ReadConfig(wfDir)
	if err != nil || cfg == nil || cfg.Repo == "" {
		return false, ""
	}
	moved, src, _ := state.RecoverMisplacedOutput(wfDir, cfg.Repo, filepath.Base(expectedPath))
	return moved, src
}

// POST /api/workflow/{name}/check-advance
func handleWorkflowCheckAdvance(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	outputFile := bodyStr(body, "output_file")
	if outputFile == "" {
		writeError(w, 400, "output_file required")
		return
	}

	// Sweep for misplaced artifacts before deciding whether to advance.
	// Covers the sandboxed-gemini case where writes to the workflow dir
	// were refused and the artifact landed in the repo root instead.
	recovered, recoveredFrom := tryRecoverArtifact(name, outputFile)

	if _, err := os.Stat(outputFile); err != nil {
		writeJSONObj(w, map[string]any{
			"advanced":       false,
			"recovered":      recovered,
			"recovered_from": recoveredFrom,
		})
		return
	}

	// File exists — advance the specific workflow
	if _, err := runCLI("advance", "--workflow", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}

	statusOut, err := runCLI("status", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}

	var statusData any
	json.Unmarshal(statusOut, &statusData)
	writeJSONObj(w, map[string]any{
		"advanced":       true,
		"status":         statusData,
		"recovered":      recovered,
		"recovered_from": recoveredFrom,
	})
}

// GET /api/workflow/{name}/chat-history/{phase}
func handleWorkflowChatHistory(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	wfDir := state.WorkflowDir(name)
	// Support ?attempt=N for retry-attempt chat logs
	var logPath string
	if attemptStr := r.URL.Query().Get("attempt"); attemptStr != "" {
		logPath = filepath.Join(wfDir, "chat-history", phase+".attempt-"+attemptStr+".log")
	} else {
		logPath = filepath.Join(wfDir, "chat-history", phase+".log")
	}

	info, err := os.Stat(logPath)
	if err != nil {
		writeJSONObj(w, map[string]bool{"exists": false})
		return
	}

	if info.Size() > 5*1024*1024 {
		writeJSONObj(w, map[string]any{
			"exists": true,
			"path":   logPath,
			"size":   info.Size(),
			"large":  true,
		})
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSONObj(w, map[string]any{
		"exists":  true,
		"content": string(data),
		"path":    logPath,
		"size":    info.Size(),
	})
}

// GET /api/workflow/{name}/chat-history/{phase}/stream
func handleWorkflowChatHistoryStream(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	wfDir := state.WorkflowDir(name)
	// Support ?attempt=N for retry-attempt chat logs
	var logPath string
	if attemptStr := r.URL.Query().Get("attempt"); attemptStr != "" {
		logPath = filepath.Join(wfDir, "chat-history", phase+".attempt-"+attemptStr+".log")
	} else {
		logPath = filepath.Join(wfDir, "chat-history", phase+".log")
	}

	if _, err := os.Stat(logPath); err != nil {
		writeError(w, 404, "Chat history not found")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeFile(w, r, logPath)
}

// POST /api/workflow/{name}/repos/add
func handleWorkflowReposAdd(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	dirPath := bodyStr(body, "path")
	if dirPath == "" {
		writeError(w, 400, "path required")
		return
	}

	if _, err := runCLI("repos", "add", dirPath, "--workflow", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/repos/remove
func handleWorkflowReposRemove(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	dirPath := bodyStr(body, "path")
	if dirPath == "" {
		writeError(w, 400, "path required")
		return
	}

	if _, err := runCLI("repos", "remove", dirPath, "--workflow", name); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	out, err := runCLI("status", "--workflow", name, "--json")
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// ── Followup & Round History ───────────────────────────────────────────────

// POST /api/followup
func handleFollowup(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	args := []string{"followup", "--json"}
	if desc := bodyStr(body, "description"); desc != "" {
		args = append(args, "--description", desc)
	}
	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// POST /api/workflow/{name}/followup
func handleWorkflowFollowup(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	body := readBody(r)
	args := []string{"followup", "--workflow", name, "--json"}
	if desc := bodyStr(body, "description"); desc != "" {
		args = append(args, "--description", desc)
	}
	out, err := runCLI(args...)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, out)
}

// GET /api/workflow/{name}/rounds/{n}/artifact/{type}
func handleWorkflowRoundArtifact(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	n := r.PathValue("n")
	if n == "" {
		writeError(w, 400, "Missing round number")
		return
	}
	artType := r.PathValue("type")
	if !validArtifacts[artType] {
		writeError(w, 400, fmt.Sprintf("Invalid artifact type: %s", artType))
		return
	}

	wfDir := state.WorkflowDir(name)
	// Support ?attempt=N for retry-attempt artifacts
	var filePath string
	if attemptStr := r.URL.Query().Get("attempt"); attemptStr != "" {
		filePath = filepath.Join(wfDir, "rounds", n, artType+".attempt-"+attemptStr+".md")
	} else {
		filePath = filepath.Join(wfDir, "rounds", n, artType+".md")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		writeError(w, 404, "Artifact not found")
		return
	}

	writeJSONObj(w, map[string]string{
		"content": string(data),
		"path":    filePath,
	})
}

// GET /api/workflow/{name}/rounds/{n}/chat-history/{phase}
func handleWorkflowRoundChatHistory(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	n := r.PathValue("n")
	if n == "" {
		writeError(w, 400, "Missing round number")
		return
	}
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	wfDir := state.WorkflowDir(name)
	// Support ?attempt=N for retry-attempt chat logs
	var logPath string
	if attemptStr := r.URL.Query().Get("attempt"); attemptStr != "" {
		logPath = filepath.Join(wfDir, "rounds", n, "chat-history", phase+".attempt-"+attemptStr+".log")
	} else {
		logPath = filepath.Join(wfDir, "rounds", n, "chat-history", phase+".log")
	}

	info, err := os.Stat(logPath)
	if err != nil {
		writeJSONObj(w, map[string]bool{"exists": false})
		return
	}

	if info.Size() > 5*1024*1024 {
		writeJSONObj(w, map[string]any{
			"exists": true,
			"path":   logPath,
			"size":   info.Size(),
			"large":  true,
		})
		return
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}

	writeJSONObj(w, map[string]any{
		"exists":  true,
		"content": string(data),
		"path":    logPath,
		"size":    info.Size(),
	})
}

// GET /api/workflow/{name}/rounds/{n}/chat-history/{phase}/stream
func handleWorkflowRoundChatHistoryStream(w http.ResponseWriter, r *http.Request) {
	name, ok := requireWorkflowName(w, r)
	if !ok {
		return
	}
	n := r.PathValue("n")
	if n == "" {
		writeError(w, 400, "Missing round number")
		return
	}
	phase := r.PathValue("phase")
	if !validPhases[phase] {
		writeError(w, 400, fmt.Sprintf("Invalid phase: %s", phase))
		return
	}

	wfDir := state.WorkflowDir(name)
	// Support ?attempt=N for retry-attempt chat logs
	var logPath string
	if attemptStr := r.URL.Query().Get("attempt"); attemptStr != "" {
		logPath = filepath.Join(wfDir, "rounds", n, "chat-history", phase+".attempt-"+attemptStr+".log")
	} else {
		logPath = filepath.Join(wfDir, "rounds", n, "chat-history", phase+".log")
	}

	if _, err := os.Stat(logPath); err != nil {
		writeError(w, 404, "Chat history not found")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeFile(w, r, logPath)
}
