package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"my-project", "project_v2", "test.name", "A1", "x"}
	for _, name := range valid {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) should pass, got: %v", name, err)
		}
	}

	invalid := []string{"", "-start", ".start", "_start", "has space", "has/slash", "has@at"}
	for _, name := range invalid {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) should fail", name)
		}
	}
}

func TestCreateDeleteProject(t *testing.T) {
	_, cleanup := withTestHome(t)
	defer cleanup()
	os.MkdirAll(ProjectsDir(), 0755)

	// Create project
	if err := CreateProject("myproj"); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if !ProjectExists("myproj") {
		t.Error("project should exist after creation")
	}

	// Check memory files were created
	ctxFile := filepath.Join(ProjectMemoryDir("myproj"), "project-context.md")
	if _, err := os.Stat(ctxFile); err != nil {
		t.Error("project-context.md should be created")
	}

	// Duplicate creation fails
	if err := CreateProject("myproj"); err == nil {
		t.Error("duplicate CreateProject should fail")
	}

	// Delete project
	if err := DeleteProject("myproj"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if ProjectExists("myproj") {
		t.Error("project should not exist after deletion")
	}
}

func TestDeleteDefaultFails(t *testing.T) {
	if err := DeleteProject("default"); err == nil {
		t.Error("deleting default project should fail")
	}
}

func TestDeleteProjectMovesWorkflows(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	// Setup
	os.MkdirAll(filepath.Join(home, "projects", "default", "memory"), 0755)
	os.MkdirAll(filepath.Join(home, "projects", "myproj", "memory"), 0755)

	wfDir := filepath.Join(home, "workflows", "test-wf")
	os.MkdirAll(wfDir, 0755)
	SetConf(wfDir, "project", "myproj")

	// Delete project
	if err := DeleteProject("myproj"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	// Workflow should now be in default
	proj, _ := GetConf(wfDir, "project")
	if proj != "default" {
		t.Errorf("workflow should be in default, got %q", proj)
	}
}

func TestRenameProject(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(home, "projects", "default", "memory"), 0755)
	os.MkdirAll(filepath.Join(home, "projects", "old-name", "memory"), 0755)

	wfDir := filepath.Join(home, "workflows", "wf1")
	os.MkdirAll(wfDir, 0755)
	SetConf(wfDir, "project", "old-name")

	if err := RenameProject("old-name", "new-name"); err != nil {
		t.Fatalf("RenameProject: %v", err)
	}

	if ProjectExists("old-name") {
		t.Error("old name should not exist")
	}
	if !ProjectExists("new-name") {
		t.Error("new name should exist")
	}

	proj, _ := GetConf(wfDir, "project")
	if proj != "new-name" {
		t.Errorf("workflow project should be new-name, got %q", proj)
	}
}

func TestRenameDefaultFails(t *testing.T) {
	if err := RenameProject("default", "other"); err == nil {
		t.Error("renaming default project should fail")
	}
}

func TestListProjects(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(home, "projects", "default", "memory"), 0755)
	os.MkdirAll(filepath.Join(home, "projects", "alpha", "memory"), 0755)

	projects, err := ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
}

func TestSuggestProject(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	// Create a project with memory containing keywords
	memDir := filepath.Join(home, "projects", "fmv-project", "memory")
	os.MkdirAll(memDir, 0755)
	os.WriteFile(filepath.Join(memDir, "project-context.md"), []byte(`
# Project Context: fmv-project
## Tech Stack: grpc, proto, vendor, postgresql
Repository: /Users/wayne/repositories/pvotal-fmv-cmd-vendor
`), 0644)

	// Description that matches multiple keywords
	sugg, err := SuggestProject("add grpc endpoint for vendor service using postgresql")
	if err != nil {
		t.Fatalf("SuggestProject: %v", err)
	}
	if sugg == nil {
		t.Fatal("expected suggestion, got nil")
	}
	if sugg.Project != "fmv-project" {
		t.Errorf("expected fmv-project, got %q", sugg.Project)
	}
	if sugg.Score < 30 {
		t.Errorf("expected score >= 30, got %d", sugg.Score)
	}
}

func TestSuggestProjectNoMatch(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(home, "projects", "default", "memory"), 0755)

	sugg, err := SuggestProject("something completely unrelated")
	if err != nil {
		t.Fatalf("SuggestProject: %v", err)
	}
	if sugg != nil {
		t.Errorf("expected nil suggestion, got %+v", sugg)
	}
}

func TestGetWorkflowProject(t *testing.T) {
	dir := t.TempDir()

	// No project key -> default
	if got := GetWorkflowProject(dir); got != "default" {
		t.Errorf("expected default, got %q", got)
	}

	// With project key
	SetConf(dir, "project", "myproj")
	if got := GetWorkflowProject(dir); got != "myproj" {
		t.Errorf("expected myproj, got %q", got)
	}
}

func TestMoveWorkflow(t *testing.T) {
	home, cleanup := withTestHome(t)
	defer cleanup()

	os.MkdirAll(filepath.Join(home, "projects", "default", "memory"), 0755)
	os.MkdirAll(filepath.Join(home, "projects", "target", "memory"), 0755)

	wfDir := filepath.Join(home, "workflows", "wf1")
	os.MkdirAll(wfDir, 0755)
	SetConf(wfDir, "project", "default")

	if err := MoveWorkflow("wf1", "target"); err != nil {
		t.Fatalf("MoveWorkflow: %v", err)
	}

	proj, _ := GetConf(wfDir, "project")
	if proj != "target" {
		t.Errorf("expected target, got %q", proj)
	}

	// Move to same project fails
	if err := MoveWorkflow("wf1", "target"); err == nil {
		t.Error("moving to same project should fail")
	}
}
