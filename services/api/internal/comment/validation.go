package comment

import (
	"strings"
	"unicode/utf8"
)

const (
	BodyMaxLength    = 1000
	ListDefaultLimit = 20
	ListMaxLimit     = 50
)

type ValidationProblem struct {
	Field string
	Code  string
}

func NormalizeCreateInput(input CreateInput) CreateInput {
	return CreateInput{
		NoteID: input.NoteID,
		UserID: input.UserID,
		Body:   strings.TrimSpace(input.Body),
	}
}

func ValidateCreateInput(input CreateInput) []ValidationProblem {
	normalized := NormalizeCreateInput(input)
	problems := make([]ValidationProblem, 0, 1)
	bodyLength := utf8.RuneCountInString(normalized.Body)
	if bodyLength == 0 {
		return append(problems, ValidationProblem{Field: "body", Code: "required"})
	}
	if bodyLength > BodyMaxLength {
		return append(problems, ValidationProblem{Field: "body", Code: "too_long"})
	}
	return problems
}

func NormalizeListInput(input ListInput) ListInput {
	if input.Limit == 0 {
		input.Limit = ListDefaultLimit
	}
	return input
}

func ValidateListInput(input ListInput) []ValidationProblem {
	normalized := NormalizeListInput(input)
	problems := make([]ValidationProblem, 0, 2)
	if normalized.Limit < 1 || normalized.Limit > ListMaxLimit {
		problems = append(problems, ValidationProblem{Field: "limit", Code: "invalid"})
	}
	if normalized.After != nil && normalized.After.PageKey <= 0 {
		problems = append(problems, ValidationProblem{Field: "cursor", Code: "invalid"})
	}
	return problems
}
