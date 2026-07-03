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
	if problems[0].Field != "category_slug" {
		t.Fatalf("first field = %s, want category_slug", problems[0].Field)
	}
	if problems[1].Field != "city_slug" {
		t.Fatalf("second field = %s, want city_slug", problems[1].Field)
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

func TestValidateCreateInputTreatsTrimmedEmptyMetadataAsRequired(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: "   ",
		CitySlug:     "\n\t",
	})

	if len(problems) != 2 {
		t.Fatalf("problem count = %d, want 2", len(problems))
	}
	if problems[0] != (ValidationProblem{Field: "category_slug", Message: "required"}) {
		t.Fatalf("first problem = %#v, want required category_slug", problems[0])
	}
	if problems[1] != (ValidationProblem{Field: "city_slug", Message: "required"}) {
		t.Fatalf("second problem = %#v, want required city_slug", problems[1])
	}
}
