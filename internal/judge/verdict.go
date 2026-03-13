package judge

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// Verdict represents the parsed outcome of a review or verify report.
type Verdict string

const (
	Pass               Verdict = "pass"
	Fix                Verdict = "fix"
	Rework             Verdict = "rework"
	Approve            Verdict = "approve"
	ApproveWithChanges Verdict = "approve_with_changes"
	Unknown            Verdict = "unknown"
	Missing            Verdict = "missing"
)

var (
	statusRE         = regexp.MustCompile(`(?i)^#{1,4}\s+status`)
	recommendationRE = regexp.MustCompile(`(?i)^#{1,4}\s+recommendation`)
	verdictRE        = regexp.MustCompile(`(?i)^#{1,4}\s+verdict`)
)

// stripBold removes markdown bold markers (**) from a string.
func stripBold(s string) string {
	return strings.ReplaceAll(s, "*", "")
}

// JudgeVerify parses a verification report and returns (verdict, status, recommendation, error).
// If the file is missing, returns (Missing, "", "", nil).
func JudgeVerify(verifyFile string) (Verdict, string, string, error) {
	f, err := os.Open(verifyFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Missing, "", "", nil
		}
		return Unknown, "", "", err
	}
	defer f.Close()

	var statusVal, recVal string
	inStatus := false
	inRec := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Detect section headers
		if statusRE.MatchString(line) {
			inStatus = true
			inRec = false
			continue
		}
		if recommendationRE.MatchString(line) {
			inRec = true
			inStatus = false
			continue
		}

		// Capture first non-empty line after header
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}

		if inStatus {
			statusVal = stripBold(stripped)
			inStatus = false
		}
		if inRec {
			recVal = stripBold(stripped)
			inRec = false
		}
	}
	if err := scanner.Err(); err != nil {
		return Unknown, "", "", err
	}

	// Determine verdict from recommendation (primary) then status (fallback)
	combined := strings.ToLower(recVal + " " + statusVal)

	var verdict Verdict
	switch {
	case strings.Contains(combined, "ship it"):
		verdict = Pass
	case strings.Contains(combined, "needs rework"):
		verdict = Rework
	case strings.Contains(combined, "fix issues"):
		verdict = Fix
	case strings.Contains(combined, "pass"):
		verdict = Pass
	case strings.Contains(combined, "fail"):
		verdict = Fix
	default:
		verdict = Unknown
	}

	return verdict, statusVal, recVal, nil
}

// JudgeReview parses a review report and returns (verdict, rawVerdictText, error).
// If the file is missing, returns (Missing, "", nil).
func JudgeReview(reviewFile string) (Verdict, string, error) {
	f, err := os.Open(reviewFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Missing, "", nil
		}
		return Unknown, "", err
	}
	defer f.Close()

	var verdictVal string
	inVerdict := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if verdictRE.MatchString(line) {
			inVerdict = true
			continue
		}

		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}

		if inVerdict {
			verdictVal = stripBold(stripped)
			inVerdict = false
		}
	}
	if err := scanner.Err(); err != nil {
		return Unknown, "", err
	}

	combined := strings.ToLower(verdictVal)

	var verdict Verdict
	switch {
	case strings.Contains(combined, "request rework"):
		verdict = Rework
	case strings.Contains(combined, "approve with changes"):
		verdict = ApproveWithChanges
	case strings.Contains(combined, "approve"):
		verdict = Approve
	default:
		verdict = Unknown
	}

	return verdict, verdictVal, nil
}
