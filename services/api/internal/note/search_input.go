package note

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	SearchQueryMaxLength = 120
	SearchDefaultLimit   = 50
)

type SearchInput struct {
	CategorySlug CategorySlug
	Query        string
	Limit        int
	ViewerUserID user.UserID
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
		ViewerUserID: input.ViewerUserID,
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

// HasSearchableTokens reports whether query contains at least one run of
// letters or digits. A query with none -- punctuation, symbols, or
// whitespace only -- has no lexical token for FTS5 to match and no
// meaningful content for the embedding model to represent, so hybrid search
// treats it as a no-results query rather than embedding it: the exact-KNN
// semantic scan has no similarity threshold and would otherwise return the
// nearest stored vectors to a meaningless embedding, surfacing the whole
// corpus for garbage input.
func HasSearchableTokens(query string) bool {
	for _, value := range query {
		if unicode.IsLetter(value) || unicode.IsDigit(value) {
			return true
		}
	}
	return false
}
