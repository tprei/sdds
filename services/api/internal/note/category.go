package note

import "strings"

type CategorySlug string

const (
	CategorySlugBeleza     CategorySlug = "beleza"
	CategorySlugComida     CategorySlug = "comida"
	CategorySlugViagem     CategorySlug = "viagem"
	CategorySlugAchadinhos CategorySlug = "achadinhos"
)

type Category struct {
	Slug  CategorySlug
	Label string
}

var Categories = []Category{
	{Slug: CategorySlugBeleza, Label: "Beleza"},
	{Slug: CategorySlugComida, Label: "Comida"},
	{Slug: CategorySlugViagem, Label: "Viagem"},
	{Slug: CategorySlugAchadinhos, Label: "Achadinhos"},
}

func NormalizeCategorySlug(slug CategorySlug) CategorySlug {
	return CategorySlug(strings.TrimSpace(string(slug)))
}

func KnownCategorySlug(slug CategorySlug) bool {
	for _, category := range Categories {
		if category.Slug == slug {
			return true
		}
	}
	return false
}
