package note

import (
	"strings"
	"unicode/utf8"
)

const (
	SearchQueryMaxLength = 120
	SearchDefaultLimit   = 50
)

type SearchInput struct {
	CategorySlug CategorySlug
	Query        string
	Limit        int
}

func NormalizeSearchInput(input SearchInput) SearchInput {
	limit := input.Limit
	if limit == 0 {
		limit = SearchDefaultLimit
	}

	return SearchInput{
		CategorySlug: NormalizeCategorySlug(input.CategorySlug),
		Query:        strings.TrimSpace(input.Query),
		Limit:        limit,
	}
}

func ValidateSearchInput(input SearchInput) []ValidationProblem {
	normalized := NormalizeSearchInput(input)
	problems := make([]ValidationProblem, 0, 2)
	problems = appendSearchQueryValidationProblems(problems, normalized.Query)
	problems = appendLimitValidationProblems(problems, normalized.Limit)
	return problems
}

func appendSearchQueryValidationProblems(problems []ValidationProblem, query string) []ValidationProblem {
	queryLength := utf8.RuneCountInString(query)
	if queryLength == 0 {
		return append(problems, ValidationProblem{Field: "q", Message: "required"})
	}
	if queryLength > SearchQueryMaxLength {
		return append(problems, ValidationProblem{Field: "q", Message: "too_long"})
	}
	return problems
}
