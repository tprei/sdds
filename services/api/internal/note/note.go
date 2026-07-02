package note

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	TitleMinLength = 3
	TitleMaxLength = 120
	BodyMaxLength  = 4000
)

type CategorySlug string

const (
	CategorySlugBeleza     CategorySlug = "beleza"
	CategorySlugComida     CategorySlug = "comida"
	CategorySlugViagem     CategorySlug = "viagem"
	CategorySlugAchadinhos CategorySlug = "achadinhos"
)

type CitySlug string

const (
	CitySlugSaoPaulo     CitySlug = "sao-paulo"
	CitySlugRioDeJaneiro CitySlug = "rio-de-janeiro"
	CitySlugLisboa       CitySlug = "lisboa"
)

type Note struct {
	ID           string
	Title        string
	Body         string
	CategorySlug CategorySlug
	CitySlug     CitySlug
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateInput struct {
	Title        string
	Body         string
	CategorySlug CategorySlug
	CitySlug     CitySlug
}

type Store interface {
	CreateNote(ctx context.Context, input CreateInput) (Note, error)
	ListRecentNotes(ctx context.Context, limit int) ([]Note, error)
}

type Category struct {
	Slug  CategorySlug
	Label string
}

type City struct {
	Slug  CitySlug
	Label string
}

type ValidationProblem struct {
	Field   string
	Message string
}

var Categories = []Category{
	{Slug: CategorySlugBeleza, Label: "Beleza"},
	{Slug: CategorySlugComida, Label: "Comida"},
	{Slug: CategorySlugViagem, Label: "Viagem"},
	{Slug: CategorySlugAchadinhos, Label: "Achadinhos"},
}

var Cities = []City{
	{Slug: CitySlugSaoPaulo, Label: "São Paulo"},
	{Slug: CitySlugRioDeJaneiro, Label: "Rio de Janeiro"},
	{Slug: CitySlugLisboa, Label: "Lisboa"},
}

func NormalizeCreateInput(input CreateInput) CreateInput {
	return CreateInput{
		Title:        strings.TrimSpace(input.Title),
		Body:         strings.TrimSpace(input.Body),
		CategorySlug: CategorySlug(strings.TrimSpace(string(input.CategorySlug))),
		CitySlug:     CitySlug(strings.TrimSpace(string(input.CitySlug))),
	}
}

func ValidateCreateInput(input CreateInput) []ValidationProblem {
	normalized := NormalizeCreateInput(input)
	problems := make([]ValidationProblem, 0)

	titleLength := utf8.RuneCountInString(normalized.Title)
	if titleLength == 0 {
		problems = append(problems, ValidationProblem{Field: "title", Message: "required"})
	} else if titleLength < TitleMinLength {
		problems = append(problems, ValidationProblem{Field: "title", Message: "too_short"})
	} else if titleLength > TitleMaxLength {
		problems = append(problems, ValidationProblem{Field: "title", Message: "too_long"})
	}

	bodyLength := utf8.RuneCountInString(normalized.Body)
	if bodyLength == 0 {
		problems = append(problems, ValidationProblem{Field: "body", Message: "required"})
	} else if bodyLength > BodyMaxLength {
		problems = append(problems, ValidationProblem{Field: "body", Message: "too_long"})
	}

	if normalized.CategorySlug == "" {
		problems = append(problems, ValidationProblem{Field: "category", Message: "required"})
	} else if !KnownCategorySlug(normalized.CategorySlug) {
		problems = append(problems, ValidationProblem{Field: "category", Message: "unknown"})
	}

	if normalized.CitySlug == "" {
		problems = append(problems, ValidationProblem{Field: "city", Message: "required"})
	} else if !KnownCitySlug(normalized.CitySlug) {
		problems = append(problems, ValidationProblem{Field: "city", Message: "unknown"})
	}

	return problems
}

func KnownCategorySlug(slug CategorySlug) bool {
	for _, category := range Categories {
		if category.Slug == slug {
			return true
		}
	}
	return false
}

func KnownCitySlug(slug CitySlug) bool {
	for _, city := range Cities {
		if city.Slug == slug {
			return true
		}
	}
	return false
}
