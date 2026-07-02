package note

import "testing"

func TestValidateCreateInputAcceptsControlledCategoryAndCity(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: "comida",
		CitySlug:     "sao-paulo",
	})

	if len(problems) != 0 {
		t.Fatalf("problem count = %d, want 0", len(problems))
	}
}

func TestValidateCreateInputRejectsUnknownMetadata(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: "qualquer",
		CitySlug:     "qualquer-lugar",
	})

	if len(problems) != 2 {
		t.Fatalf("problem count = %d, want 2", len(problems))
	}
	if problems[0].Field != "category" {
		t.Fatalf("first field = %s, want category", problems[0].Field)
	}
	if problems[1].Field != "city" {
		t.Fatalf("second field = %s, want city", problems[1].Field)
	}
}

func TestNormalizeCreateInputTrimsBoundaryFields(t *testing.T) {
	normalized := NormalizeCreateInput(CreateInput{
		Title:        " Café bom ",
		Body:         "\nTem pão de queijo.\n",
		CategorySlug: " comida ",
		CitySlug:     " sao-paulo ",
	})

	if normalized.Title != "Café bom" {
		t.Fatalf("title = %q, want Café bom", normalized.Title)
	}
	if normalized.Body != "Tem pão de queijo." {
		t.Fatalf("body = %q, want Tem pão de queijo.", normalized.Body)
	}
	if normalized.CategorySlug != "comida" {
		t.Fatalf("category = %q, want comida", normalized.CategorySlug)
	}
	if normalized.CitySlug != "sao-paulo" {
		t.Fatalf("city = %q, want sao-paulo", normalized.CitySlug)
	}
}
