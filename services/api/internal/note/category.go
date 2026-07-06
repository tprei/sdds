package note

import "strings"

type CategorySlug string

const (
	CategorySlugBeauty CategorySlug = "beauty"
	CategorySlugFood   CategorySlug = "food"
	CategorySlugTravel CategorySlug = "travel"
	CategorySlugFinds  CategorySlug = "finds"
)

type Category struct {
	Slug         CategorySlug
	Label        string
	Active       bool
	DisplayOrder int
}

var Categories = []Category{
	{Slug: CategorySlugBeauty, Label: "Beleza", Active: true, DisplayOrder: 10},
	{Slug: CategorySlugFood, Label: "Comida", Active: true, DisplayOrder: 20},
	{Slug: CategorySlugTravel, Label: "Viagem", Active: true, DisplayOrder: 30},
	{Slug: CategorySlugFinds, Label: "Achadinhos", Active: true, DisplayOrder: 40},
}

func NormalizeCategorySlug(slug CategorySlug) CategorySlug {
	return CategorySlug(strings.TrimSpace(string(slug)))
}
