package report

import (
	"errors"
	"strings"
	"unicode/utf8"
)

type ValidationProblem struct {
	Field string
	Code  string
}

// NormalizeCreateInput trims the target id and the optional explanation. A
// blank explanation becomes nil so callers never store empty details text.
func NormalizeCreateInput(input CreateInput) CreateInput {
	var details *string
	if input.Details != nil {
		trimmed := strings.TrimSpace(*input.Details)
		if trimmed != "" {
			details = &trimmed
		}
	}
	return CreateInput{
		TargetType:     input.TargetType,
		TargetID:       strings.TrimSpace(input.TargetID),
		Reason:         input.Reason,
		Details:        details,
		ReporterUserID: input.ReporterUserID,
	}
}

// ValidateCreateInput reports public field problems for a report submission.
// It checks only the four public fields. A missing reporter is internal input
// rejected by ValidateInternal; it is never surfaced as a public validation
// field because the HTTP validation mapper consumes only these problems.
func ValidateCreateInput(input CreateInput) []ValidationProblem {
	normalized := NormalizeCreateInput(input)
	problems := make([]ValidationProblem, 0, 4)

	switch {
	case normalized.TargetType == "":
		problems = append(problems, ValidationProblem{Field: "target_type", Code: "required"})
	case !isKnownTargetType(normalized.TargetType):
		problems = append(problems, ValidationProblem{Field: "target_type", Code: "invalid"})
	}

	if normalized.TargetID == "" {
		problems = append(problems, ValidationProblem{Field: "target_id", Code: "required"})
	}

	switch {
	case normalized.Reason == "":
		problems = append(problems, ValidationProblem{Field: "reason", Code: "required"})
	case !isKnownReason(normalized.Reason):
		problems = append(problems, ValidationProblem{Field: "reason", Code: "invalid"})
	}

	if normalized.Details != nil && utf8.RuneCountInString(*normalized.Details) > DetailsMaxLength {
		problems = append(problems, ValidationProblem{Field: "details", Code: "too_long"})
	}

	return problems
}

// ErrMissingReporter signals internal input with no authenticated reporter.
// It is a sentinel error, not a ValidationProblem, so the HTTP validation
// mapper never surfaces it as a field error.
var ErrMissingReporter = errors.New("reporter user id is required")

// ValidateInternal rejects internal input that must never reach persistence.
// A missing reporter is always rejected but never surfaces as a public
// validation field; callers translate it to an internal error response.
func ValidateInternal(input CreateInput) error {
	if NormalizeCreateInput(input).ReporterUserID == "" {
		return ErrMissingReporter
	}
	return nil
}

func isKnownTargetType(value TargetType) bool {
	switch value {
	case TargetTypeNote, TargetTypeComment:
		return true
	default:
		return false
	}
}

func isKnownReason(value Reason) bool {
	switch value {
	case ReasonSpam, ReasonHarassment, ReasonHarmfulOrMisleading, ReasonOther:
		return true
	default:
		return false
	}
}
