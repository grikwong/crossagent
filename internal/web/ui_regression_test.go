package web

import (
	"strings"
	"testing"
)

// TestTerminalDrawerEdgeTriggered asserts that applyOpen() only schedules a fit
// on the false→true open transition, not on every render while already open.
// If this test fails, the drawer will queue repeated fits during active sessions.
func TestTerminalDrawerEdgeTriggered(t *testing.T) {
	src, err := frontendFS.ReadFile("public/js/regions/terminal-drawer.js")
	if err != nil {
		t.Fatalf("cannot read terminal-drawer.js: %v", err)
	}
	content := string(src)

	// The wasOpen guard is the critical invariant: fit is only scheduled when
	// transitioning from closed to open, not on every render while open.
	if !strings.Contains(content, "wasOpen") {
		t.Error("terminal-drawer.js: applyOpen() is missing the wasOpen edge-trigger guard")
	}

	// The old unconditional timer must not exist; it caused repeated fits on every
	// store update while the drawer was open.
	if strings.Contains(content, "setTimeout(doFit, 220)") {
		t.Error("terminal-drawer.js: unconditional setTimeout(doFit, 220) still present — this causes repeated fits on every render while the drawer is open")
	}

	// transitionend must be used for accurate post-animation timing.
	if !strings.Contains(content, "transitionend") {
		t.Error("terminal-drawer.js: applyOpen() must use transitionend for accurate post-animation fit timing")
	}
}

// TestArtifactInfoRailOrder asserts that the right rail renders sections in the
// order: This run → Directories → Description.
// If this test fails, the Description section will appear before the run info.
func TestArtifactInfoRailOrder(t *testing.T) {
	src, err := frontendFS.ReadFile("public/js/regions/artifact-info-rail.js")
	if err != nil {
		t.Fatalf("cannot read artifact-info-rail.js: %v", err)
	}
	content := string(src)

	// Find the innerHTML assignment block to check section order.
	thisRunIdx := strings.Index(content, "${thisRun}")
	descIdx := strings.Index(content, "${descHtml}")
	if thisRunIdx < 0 {
		t.Fatal("artifact-info-rail.js: ${thisRun} not found in template")
	}
	if descIdx < 0 {
		t.Fatal("artifact-info-rail.js: ${descHtml} not found in template")
	}
	if descIdx < thisRunIdx {
		t.Error("artifact-info-rail.js: ${descHtml} appears before ${thisRun} — expected order is: This run → Directories → Description")
	}
}
