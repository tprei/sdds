package comment

import (
	"strings"
	"unicode/utf8"
)

const (
	BodyMaxLength     = 1000
	ListDefaultLimit  = 20
	ListMaxLimit      = 50
	ReplyMaxPerParent = 20
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
	return validateBody(normalized.Body)
}

func validateBody(body string) []ValidationProblem {
	bodyLength := utf8.RuneCountInString(body)
	if bodyLength == 0 {
		return []ValidationProblem{{Field: "body", Code: "required"}}
	}
	if bodyLength > BodyMaxLength {
		return []ValidationProblem{{Field: "body", Code: "too_long"}}
	}
	return []ValidationProblem{}
}

func NormalizeCreateReplyInput(input CreateReplyInput) CreateReplyInput {
	return CreateReplyInput{
		ParentCommentID: input.ParentCommentID,
		UserID:          input.UserID,
		Body:            strings.TrimSpace(input.Body),
	}
}

func ValidateCreateReplyInput(input CreateReplyInput) []ValidationProblem {
	normalized := NormalizeCreateReplyInput(input)
	if normalized.ParentCommentID == "" {
		return []ValidationProblem{{Field: "parent_comment_id", Code: "required"}}
	}
	return validateBody(normalized.Body)
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
