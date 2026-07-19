package note

import "testing"

func TestCreateRequestFingerprintFramesFieldsAndOrder(t *testing.T) {
	base := CreateInput{
		Title: "Title", Body: "Body", CategorySlug: CategorySlugFood, PlaceSlug: PlaceSlugSaoPaulo,
		ImageUploadIDs: []string{"upload-a", "upload-b"},
	}
	tests := []struct {
		name   string
		mutate func(*CreateInput)
	}{
		{"title", func(input *CreateInput) { input.Title = "Other title" }},
		{"body", func(input *CreateInput) { input.Body = "Other body" }},
		{"category", func(input *CreateInput) { input.CategorySlug = CategorySlugTravel }},
		{"place", func(input *CreateInput) { input.PlaceSlug = PlaceSlugRioDeJaneiro }},
		{"image value", func(input *CreateInput) { input.ImageUploadIDs[0] = "upload-c" }},
		{"image order", func(input *CreateInput) {
			input.ImageUploadIDs[0], input.ImageUploadIDs[1] = input.ImageUploadIDs[1], input.ImageUploadIDs[0]
		}},
	}
	for _, test := range tests {
		mutated := base
		mutated.ImageUploadIDs = append([]string(nil), base.ImageUploadIDs...)
		test.mutate(&mutated)
		if CreateRequestFingerprint(base) == CreateRequestFingerprint(mutated) {
			t.Fatalf("%s mutation produced the same fingerprint", test.name)
		}
	}

	boundary := CreateInput{Title: "ab", Body: "c", CategorySlug: CategorySlugFood, ImageUploadIDs: []string{"upload-a"}}
	if CreateRequestFingerprint(boundary) == CreateRequestFingerprint(CreateInput{Title: "a", Body: "bc", CategorySlug: CategorySlugFood, ImageUploadIDs: []string{"upload-a"}}) {
		t.Fatal("length-framed title/body boundary collision")
	}
	withNil, withEmpty := base, base
	withNil.ImageUploadIDs, withEmpty.ImageUploadIDs = nil, []string{}
	if CreateRequestFingerprint(withNil) != CreateRequestFingerprint(withEmpty) {
		t.Fatal("nil and empty upload ID lists should have the same fingerprint")
	}

	raw := base
	raw.UserID, raw.ClientRequestID = "user-a", "request-a"
	raw.Title, raw.Body = " Title ", "\nBody\t"
	raw.CategorySlug, raw.PlaceSlug = " food ", " sao-paulo "
	if got, want := CreateRequestFingerprint(raw), CreateRequestFingerprint(NormalizeCreateInput(raw)); got != want {
		t.Fatalf("raw fingerprint = %q, want normalized fingerprint %q", got, want)
	}
	changedReceiptKey := raw
	changedReceiptKey.UserID, changedReceiptKey.ClientRequestID = "user-b", "request-b"
	if got, want := CreateRequestFingerprint(changedReceiptKey), CreateRequestFingerprint(raw); got != want {
		t.Fatalf("receipt-key fingerprint = %q, want %q", got, want)
	}

	const want = "a663e5a87ebaba289a7d6cb6049914183e0e038366db244f527bc401d54bb5db"
	if got := CreateRequestFingerprint(base); got != want {
		t.Fatalf("fingerprint = %q, want %q", got, want)
	}
}
