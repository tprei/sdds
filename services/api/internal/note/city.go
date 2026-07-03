package note

import "strings"

type CitySlug string

const (
	CitySlugSaoPaulo     CitySlug = "sao-paulo"
	CitySlugRioDeJaneiro CitySlug = "rio-de-janeiro"
	CitySlugLisboa       CitySlug = "lisboa"
)

type City struct {
	Slug  CitySlug
	Label string
}

var Cities = []City{
	{Slug: CitySlugSaoPaulo, Label: "São Paulo"},
	{Slug: CitySlugRioDeJaneiro, Label: "Rio de Janeiro"},
	{Slug: CitySlugLisboa, Label: "Lisboa"},
}

func NormalizeCitySlug(slug CitySlug) CitySlug {
	return CitySlug(strings.TrimSpace(string(slug)))
}

func KnownCitySlug(slug CitySlug) bool {
	for _, city := range Cities {
		if city.Slug == slug {
			return true
		}
	}
	return false
}
