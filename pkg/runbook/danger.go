package runbook

import (
	"fmt"
	"strings"
)

// MatchesDanger reports whether source contains any of the document's
// danger_patterns, returning the first pattern matched.
func MatchesDanger(source string, patterns []string) (matched bool, pattern string) {
	for _, p := range patterns {
		if strings.Contains(source, p) {
			return true, p
		}
	}
	return false, ""
}

// RequiresConfirmation reports whether running step needs an explicit
// confirmation click, and why. A danger_patterns match forces confirmation
// regardless of the step's own confirm= value (Requirement 8.2).
func RequiresConfirmation(step *Step, dangerPatterns []string) (required bool, reason string) {
	if matched, pattern := MatchesDanger(step.Source, dangerPatterns); matched {
		return true, fmt.Sprintf("matches danger pattern %q", pattern)
	}
	if step.Confirm {
		return true, "step declares confirm=true"
	}
	return false, ""
}

// CheckConfirmation validates a run request: it fails if confirmation is
// required but the request didn't carry confirmed=true.
func CheckConfirmation(step *Step, dangerPatterns []string, confirmed bool) error {
	required, reason := RequiresConfirmation(step, dangerPatterns)
	if required && !confirmed {
		return &Error{Type: ErrorTypeValidation, Message: fmt.Sprintf("step %q requires confirmation: %s", step.Name, reason)}
	}
	return nil
}
