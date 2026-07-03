package note

import (
	"strings"
	"unicode/utf8"
)

const (
	TitleMinLength = 3
	TitleMaxLength = 120
	BodyMaxLength  = 4000
)

type ValidationProblem struct {
	Field   string
	Message string
}

func NormalizeCreateInput(input CreateInput) CreateInput {
	return CreateInput{
		Title:        strings.TrimSpace(input.Title),
		Body:         strings.TrimSpace(input.Body),
		CategorySlug: NormalizeCategorySlug(input.CategorySlug),
		CitySlug:     NormalizeCitySlug(input.CitySlug),
	}
}

func ValidateCreateInput(input CreateInput) []ValidationProblem {
	normalized := NormalizeCreateInput(input)
	problems := make([]ValidationProblem, 0, 4)
	problems = appendTitleValidationProblems(problems, normalized.Title)
	problems = appendBodyValidationProblems(problems, normalized.Body)
	problems = appendCategoryValidationProblems(problems, normalized.CategorySlug)
	problems = appendCityValidationProblems(problems, normalized.CitySlug)
	return problems
}

func appendTitleValidationProblems(problems []ValidationProblem, title string) []ValidationProblem {
	titleLength := utf8.RuneCountInString(title)
	if titleLength == 0 {
		return append(problems, ValidationProblem{Field: "title", Message: "required"})
	}
	if titleLength < TitleMinLength {
		return append(problems, ValidationProblem{Field: "title", Message: "too_short"})
	}
	if titleLength > TitleMaxLength {
		return append(problems, ValidationProblem{Field: "title", Message: "too_long"})
	}
	return problems
}

func appendBodyValidationProblems(problems []ValidationProblem, body string) []ValidationProblem {
	bodyLength := utf8.RuneCountInString(body)
	if bodyLength == 0 {
		return append(problems, ValidationProblem{Field: "body", Message: "required"})
	}
	if bodyLength > BodyMaxLength {
		return append(problems, ValidationProblem{Field: "body", Message: "too_long"})
	}
	return problems
}

func appendCategoryValidationProblems(problems []ValidationProblem, slug CategorySlug) []ValidationProblem {
	if slug == "" {
		return append(problems, ValidationProblem{Field: "category_slug", Message: "required"})
	}
	if !KnownCategorySlug(slug) {
		return append(problems, ValidationProblem{Field: "category_slug", Message: "unknown"})
	}
	return problems
}

func appendCityValidationProblems(problems []ValidationProblem, slug CitySlug) []ValidationProblem {
	if slug == "" {
		return append(problems, ValidationProblem{Field: "city_slug", Message: "required"})
	}
	if !KnownCitySlug(slug) {
		return append(problems, ValidationProblem{Field: "city_slug", Message: "unknown"})
	}
	return problems
}
