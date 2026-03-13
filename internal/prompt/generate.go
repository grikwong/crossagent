package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pvotal-tech/crossagent/internal/state"
)

// GenerateGeneralInstructions generates prompts/general.md for a workflow.
func GenerateGeneralInstructions(wfDir string, cfg *state.Config) (string, error) {
	outPath := filepath.Join(wfDir, "prompts", "general.md")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", err
	}

	dirs := buildWorkspaceDirs(wfDir, cfg)

	data := GeneralData{
		WorkspaceDirs: dirs,
	}

	tmpl, err := LoadTemplate("general")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute general template: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}

	return outPath, nil
}

// GeneratePlanPrompt generates prompts/plan.md for a workflow.
func GeneratePlanPrompt(wfDir string, cfg *state.Config) (string, error) {
	outPath := filepath.Join(wfDir, "prompts", "plan.md")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", err
	}

	generalPath, err := GenerateGeneralInstructions(wfDir, cfg)
	if err != nil {
		return "", err
	}

	desc, err := state.GetDescription(wfDir)
	if err != nil {
		return "", err
	}

	memCtx, err := BuildMemoryContext(wfDir)
	if err != nil {
		return "", err
	}

	memInstr, err := BuildMemoryUpdateInstructions(wfDir, "Plan")
	if err != nil {
		return "", err
	}

	retryMode, attemptNum, revertCtx := getRetryContext(wfDir, cfg)

	data := PlanData{
		GeneralPath:             generalPath,
		Description:             desc,
		DescPath:                filepath.Join(wfDir, "description"),
		PlanPath:                filepath.Join(wfDir, "plan.md"),
		WfDir:                   wfDir,
		RetryMode:               retryMode,
		AttemptNum:              attemptNum,
		RevertContext:           revertCtx,
		MemoryContext:           memCtx,
		MemoryUpdateInstructions: memInstr,
	}

	tmpl, err := LoadTemplate("plan")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute plan template: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}

	return outPath, nil
}

// GenerateReviewPrompt generates prompts/review.md for a workflow.
func GenerateReviewPrompt(wfDir string, cfg *state.Config) (string, error) {
	outPath := filepath.Join(wfDir, "prompts", "review.md")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", err
	}

	generalPath, err := GenerateGeneralInstructions(wfDir, cfg)
	if err != nil {
		return "", err
	}

	memCtx, err := BuildMemoryContext(wfDir)
	if err != nil {
		return "", err
	}

	memInstr, err := BuildMemoryUpdateInstructions(wfDir, "Review")
	if err != nil {
		return "", err
	}

	retryMode, attemptNum, _ := getRetryContext(wfDir, cfg)

	data := ReviewData{
		GeneralPath:             generalPath,
		PlanPath:                filepath.Join(wfDir, "plan.md"),
		ReviewPath:              filepath.Join(wfDir, "review.md"),
		WfDir:                   wfDir,
		RetryMode:               retryMode,
		AttemptNum:              attemptNum,
		MemoryContext:           memCtx,
		MemoryUpdateInstructions: memInstr,
	}

	tmpl, err := LoadTemplate("review")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute review template: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}

	return outPath, nil
}

// GenerateImplementPrompt generates prompts/implement.md for a workflow.
func GenerateImplementPrompt(wfDir string, cfg *state.Config, subPhase int) (string, error) {
	outPath := filepath.Join(wfDir, "prompts", "implement.md")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", err
	}

	generalPath, err := GenerateGeneralInstructions(wfDir, cfg)
	if err != nil {
		return "", err
	}

	memCtx, err := BuildMemoryContext(wfDir)
	if err != nil {
		return "", err
	}

	memInstr, err := BuildMemoryUpdateInstructions(wfDir, "Implement")
	if err != nil {
		return "", err
	}

	retryMode, attemptNum, revertCtx := getRetryContext(wfDir, cfg)

	data := ImplementData{
		GeneralPath:             generalPath,
		PlanPath:                filepath.Join(wfDir, "plan.md"),
		ReviewPath:              filepath.Join(wfDir, "review.md"),
		ImplReportPath:          filepath.Join(wfDir, "implement.md"),
		ImplPhase:               subPhase,
		WfDir:                   wfDir,
		RetryMode:               retryMode,
		AttemptNum:              attemptNum,
		RevertContext:           revertCtx,
		MemoryContext:           memCtx,
		MemoryUpdateInstructions: memInstr,
	}

	tmpl, err := LoadTemplate("implement")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute implement template: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}

	return outPath, nil
}

// GenerateVerifyPrompt generates prompts/verify.md for a workflow.
func GenerateVerifyPrompt(wfDir string, cfg *state.Config) (string, error) {
	outPath := filepath.Join(wfDir, "prompts", "verify.md")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return "", err
	}

	generalPath, err := GenerateGeneralInstructions(wfDir, cfg)
	if err != nil {
		return "", err
	}

	memCtx, err := BuildMemoryContext(wfDir)
	if err != nil {
		return "", err
	}

	memInstr, err := BuildMemoryUpdateInstructions(wfDir, "Verify")
	if err != nil {
		return "", err
	}

	retryMode, attemptNum, _ := getRetryContext(wfDir, cfg)

	data := VerifyData{
		GeneralPath:             generalPath,
		PlanPath:                filepath.Join(wfDir, "plan.md"),
		ReviewPath:              filepath.Join(wfDir, "review.md"),
		VerifyPath:              filepath.Join(wfDir, "verify.md"),
		WfDir:                   wfDir,
		RetryMode:               retryMode,
		AttemptNum:              attemptNum,
		MemoryContext:           memCtx,
		MemoryUpdateInstructions: memInstr,
	}

	tmpl, err := LoadTemplate("verify")
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute verify template: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0644); err != nil {
		return "", err
	}

	return outPath, nil
}

// buildWorkspaceDirs constructs the workspace directory list for general instructions.
func buildWorkspaceDirs(wfDir string, cfg *state.Config) []WorkspaceDir {
	dirs := []WorkspaceDir{
		{Path: wfDir, Label: "Workflow directory", Description: "where phase artifacts (plan.md, review.md, etc.) live"},
		{Path: cfg.Repo, Label: "Primary repository", Description: "the main codebase"},
		{Path: state.GlobalMemoryDir(), Label: "Global memory", Description: "cross-workflow patterns and lessons learned"},
	}

	// Project memory directory
	if cfg.Project != "" {
		projMemDir := state.ProjectMemoryDir(cfg.Project)
		if info, err := os.Stat(projMemDir); err == nil && info.IsDir() {
			dirs = append(dirs, WorkspaceDir{
				Path:        projMemDir,
				Label:       "Project memory",
				Description: "project-specific patterns, features, and conventions",
			})
		}
	}

	// Additional directories
	for _, d := range cfg.AddDirs {
		if d != "" {
			dirs = append(dirs, WorkspaceDir{
				Path:        d,
				Label:       "Additional directory",
				Description: "",
			})
		}
	}

	return dirs
}

// getRetryContext checks if this is a retry and returns the context.
func getRetryContext(wfDir string, cfg *state.Config) (retryMode bool, attemptNum int, revertCtx string) {
	revertFile := filepath.Join(wfDir, "prompts", "revert-context.md")
	data, err := os.ReadFile(revertFile)
	if err != nil {
		return false, 0, ""
	}

	attemptNum = cfg.RetryCount
	if attemptNum == 0 {
		attemptNum = 1
	}

	return true, attemptNum, string(data)
}
