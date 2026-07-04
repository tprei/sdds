package note

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

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

	want := []ValidationProblem{
		{Field: "category_slug", Message: "unknown"},
		{Field: "city_slug", Message: "unknown"},
	}
	if diff := cmp.Diff(want, problems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}

func TestNormalizeCreateInputTrimsBoundaryFields(t *testing.T) {
	normalized := NormalizeCreateInput(CreateInput{
		Title:        " Café bom ",
		Body:         "\nTem pão de queijo.\n",
		CategorySlug: " comida ",
		CitySlug:     " sao-paulo ",
	})

	want := CreateInput{
		Title:        "Café bom",
		Body:         "Tem pão de queijo.",
		CategorySlug: "comida",
		CitySlug:     "sao-paulo",
	}
	if diff := cmp.Diff(want, normalized); diff != "" {
		t.Fatalf("normalized input mismatch (-want +got):\n%s", diff)
	}
}

func TestValidateCreateInputTreatsTrimmedEmptyMetadataAsRequired(t *testing.T) {
	problems := ValidateCreateInput(CreateInput{
		Title:        "Café bom",
		Body:         "Tem pão de queijo decente.",
		CategorySlug: "   ",
		CitySlug:     "\n\t",
	})

	want := []ValidationProblem{
		{Field: "category_slug", Message: "required"},
		{Field: "city_slug", Message: "required"},
	}
	if diff := cmp.Diff(want, problems); diff != "" {
		t.Fatalf("validation problems mismatch (-want +got):\n%s", diff)
	}
}
