package note

import "strings"

type PlaceSlug string

const (
	PlaceSlugSaoPaulo     PlaceSlug = "sao-paulo"
	PlaceSlugRioDeJaneiro PlaceSlug = "rio-de-janeiro"
	PlaceSlugLisboa       PlaceSlug = "lisboa"
)

type Place struct {
	Slug         PlaceSlug
	Label        string
	Active       bool
	DisplayOrder int
}

var Places = []Place{
	{Slug: PlaceSlugSaoPaulo, Label: "São Paulo", Active: true, DisplayOrder: 10},
	{Slug: PlaceSlugRioDeJaneiro, Label: "Rio de Janeiro", Active: true, DisplayOrder: 20},
	{Slug: PlaceSlugLisboa, Label: "Lisboa", Active: true, DisplayOrder: 30},
}

func NormalizePlaceSlug(slug PlaceSlug) PlaceSlug {
	return PlaceSlug(strings.TrimSpace(string(slug)))
}
