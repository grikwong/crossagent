package prompt

import (
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*.md.tmpl
var templateFS embed.FS

// LoadTemplate loads and parses a named template from the embedded filesystem.
func LoadTemplate(name string) (*template.Template, error) {
	path := "templates/" + name + ".md.tmpl"
	data, err := templateFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("template %q not found: %w", name, err)
	}
	tmpl, err := template.New(name).Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %q: %w", name, err)
	}
	return tmpl, nil
}

// WorkspaceDir describes a workspace directory for general instructions.
type WorkspaceDir struct {
	Path        string
	Label       string // e.g., "Workflow directory"
	Description string // e.g., "where phase artifacts live"
}

// GeneralData holds data for general.md.tmpl.
type GeneralData struct {
	WorkspaceDirs []WorkspaceDir
}

// PlanData holds data for plan.md.tmpl.
type PlanData struct {
	GeneralPath              string
	Description              string
	DescPath                 string
	PlanPath                 string
	WfDir                    string
	RetryMode                bool
	AttemptNum               int
	RevertContext            string
	FollowupMode             bool
	FollowupRound            int
	FollowupContext          string
	MemoryContext            string
	MemoryUpdateInstructions string
}

// ReviewData holds data for review.md.tmpl.
type ReviewData struct {
	GeneralPath             string
	PlanPath                string
	ReviewPath              string
	WfDir                   string
	RetryMode               bool
	AttemptNum              int
	MemoryContext           string
	MemoryUpdateInstructions string
}

// ImplementData holds data for implement.md.tmpl.
type ImplementData struct {
	GeneralPath             string
	PlanPath                string
	ReviewPath              string
	ImplReportPath          string
	ImplPhase               int
	WfDir                   string
	RetryMode               bool
	AttemptNum              int
	RevertContext           string
	MemoryContext           string
	MemoryUpdateInstructions string
}

// VerifyData holds data for verify.md.tmpl.
type VerifyData struct {
	GeneralPath             string
	PlanPath                string
	ReviewPath              string
	VerifyPath              string
	WfDir                   string
	RetryMode               bool
	AttemptNum              int
	MemoryContext           string
	MemoryUpdateInstructions string
}
