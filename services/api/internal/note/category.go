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
	Slug         CategorySlug
	Label        string
	Active       bool
	DisplayOrder int
}

var Categories = []Category{
	{Slug: CategorySlugBeleza, Label: "Beleza", Active: true, DisplayOrder: 10},
	{Slug: CategorySlugComida, Label: "Comida", Active: true, DisplayOrder: 20},
	{Slug: CategorySlugViagem, Label: "Viagem", Active: true, DisplayOrder: 30},
	{Slug: CategorySlugAchadinhos, Label: "Achadinhos", Active: true, DisplayOrder: 40},
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
