package event

import (
	"strings"
	"testing"

	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/user"
)

const (
	testEventID  = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d01"
	testUserID   = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d02"
	testNoteID   = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d03"
	testNoteID2  = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d04"
	testSearchID = "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d05"
)

func TestNormalizeAndValidateAcceptsEveryKind(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		kind    Kind
		payload Payload
	}{
		{"explore impression", KindExploreNotesImpression, ExploreNotesImpressionPayload{CategorySlug: categoryPtr(note.CategorySlugFood), ResultCount: 1, Results: []ExploreResult{{NoteID: testNoteID, Rank: 1}}}},
		{"explore opened", KindExploreNoteOpened, ExploreNoteOpenedPayload{NoteID: testNoteID, Rank: 1}},
		{"search submitted", KindSearchSubmitted, SearchSubmittedPayload{SearchID: testSearchID, SearchVersion: SearchVersionFTS5V1, Query: " café  bom "}},
		{"search impression", KindSearchResultsImpression, SearchResultsImpressionPayload{SearchID: testSearchID, SearchVersion: SearchVersionFTS5V1, Query: "café bom", ResultCount: 1, Results: []SearchResult{{NoteID: testNoteID, Rank: 1, RetrievalSource: RetrievalSourceLexical}}}},
		{"search opened", KindSearchResultOpened, SearchResultOpenedPayload{SearchID: testSearchID, SearchVersion: SearchVersionFTS5V1, NoteID: testNoteID, Rank: 1, RetrievalSource: RetrievalSourceLexical}},
		{"search reformulated", KindSearchReformulated, SearchReformulatedPayload{PreviousSearchID: testSearchID, PreviousSearchVersion: SearchVersionFTS5V1, SearchID: testEventID, SearchVersion: SearchVersionFTS5V1, PreviousQuery: "café", Query: "café bom"}},
		{"search empty", KindSearchNoResults, SearchNoResultsPayload{SearchID: testSearchID, SearchVersion: SearchVersionFTS5V1, Query: "café", ResultCount: 0}},
		{"marked useful", KindNoteMarkedUseful, NoteMarkedUsefulPayload{NoteID: testNoteID, Context: SearchUsefulContext{Source: "search", SearchID: testSearchID, SearchVersion: SearchVersionFTS5V1, Rank: 1, RetrievalSource: RetrievalSourceLexical}}},
		{"unmarked useful", KindNoteUnmarkedUseful, NoteUnmarkedUsefulPayload{NoteID: testNoteID, Context: NoteDetailUsefulContext{Source: "note_detail"}}},
		{"comment", KindCommentCreated, CommentCreatedPayload{NoteID: testNoteID, CommentID: "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d06"}},
		{"report", KindReportCreated, ReportCreatedPayload{ReportID: "018f2f5b-9f1f-7b42-9a43-7c9c6f8f1d07", TargetType: "note", TargetID: testNoteID}},
		{"published", KindNotePublished, NotePublishedPayload{NoteID: testNoteID, CategorySlug: note.CategorySlugFood}},
	}

	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			record, problems := NormalizeAndValidate(validInput(testCase.kind, testCase.payload))
			if len(problems) != 0 {
				t.Fatalf("unexpected problems: %+v", problems)
			}
			if record.ID != testEventID || record.Kind != testCase.kind || record.Payload == nil {
				t.Fatalf("unexpected record: %+v", record)
			}
		})
	}
}

func TestNormalizeAndValidateTrimsOnlyQueryWhitespace(t *testing.T) {
	input := validInput(KindSearchSubmitted, SearchSubmittedPayload{
		SearchID:      testSearchID,
		SearchVersion: SearchVersionFTS5V1,
		Query:         "  Café   BOM  ",
	})
	record, problems := NormalizeAndValidate(input)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %+v", problems)
	}
	payload := record.Payload.(SearchSubmittedPayload)
	if payload.Query != "Café   BOM" {
		t.Fatalf("query was normalized incorrectly: %q", payload.Query)
	}
}

func TestNormalizeAndValidateRejectsResultShape(t *testing.T) {
	cases := []struct {
		name    string
		payload Payload
		field   string
	}{
		{
			name:    "nil explore results",
			payload: ExploreNotesImpressionPayload{ResultCount: 0, Results: nil},
			field:   "payload.results",
		},
		{
			name:    "duplicate note IDs",
			payload: ExploreNotesImpressionPayload{ResultCount: 2, Results: []ExploreResult{{NoteID: testNoteID, Rank: 1}, {NoteID: testNoteID, Rank: 2}}},
			field:   "payload.results[1].note_id",
		},
		{
			name:    "noncontiguous rank",
			payload: SearchResultsImpressionPayload{SearchID: testSearchID, SearchVersion: SearchVersionFTS5V1, Query: "q", ResultCount: 2, Results: []SearchResult{{NoteID: testNoteID, Rank: 1, RetrievalSource: RetrievalSourceLexical}, {NoteID: testNoteID2, Rank: 3, RetrievalSource: RetrievalSourceLexical}}},
			field:   "payload.results[1].rank",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, problems := NormalizeAndValidate(validInput(kindForPayload(testCase.payload), testCase.payload))
			if !hasProblem(problems, testCase.field, "") {
				t.Fatalf("expected %s in %+v", testCase.field, problems)
			}
		})
	}
}

func TestNormalizeAndValidateRejectsEnvelopeBoundaries(t *testing.T) {
	base := validInput(KindSearchSubmitted, SearchSubmittedPayload{SearchID: testSearchID, SearchVersion: SearchVersionFTS5V1, Query: "q"})
	cases := []struct {
		name   string
		mutate func(*Input)
		field  string
		code   string
	}{
		{"noncanonical ID", func(input *Input) { input.ID = strings.ToUpper(testEventID) }, "id", "invalid"},
		{"future schema", func(input *Input) { input.SchemaVersion = 2 }, "schema_version", "unsupported"},
		{"timestamp before epoch", func(input *Input) { input.OccurredAt = 0 }, "occurred_at", "invalid"},
		{"timestamp after supported range", func(input *Input) { input.OccurredAt = maxOccurredAt + 1 }, "occurred_at", "invalid"},
		{"invalid platform", func(input *Input) { input.Platform = "desktop" }, "platform", "invalid"},
		{"overlong app version", func(input *Input) { value := strings.Repeat("v", 65); input.AppVersion = &value }, "app_version", "too_long"},
		{"unknown search version", func(input *Input) {
			payload := input.Payload.(SearchSubmittedPayload)
			payload.SearchVersion = "semantic-v1"
			input.Payload = payload
		}, "payload.search_version", "unsupported"},
		{"overlong query", func(input *Input) {
			payload := input.Payload.(SearchSubmittedPayload)
			payload.Query = strings.Repeat("q", 121)
			input.Payload = payload
		}, "payload.query", "too_long"},
		{"kind payload mismatch", func(input *Input) { input.Kind = KindNotePublished }, "payload", "invalid"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			input := base
			testCase.mutate(&input)
			_, problems := NormalizeAndValidate(input)
			if !hasProblem(problems, testCase.field, testCase.code) {
				t.Fatalf("expected %s/%s in %+v", testCase.field, testCase.code, problems)
			}
		})
	}
}

func TestNormalizeAndValidateAcceptsHybridSearchVersion(t *testing.T) {
	input := validInput(KindSearchSubmitted, SearchSubmittedPayload{
		SearchID:      testSearchID,
		SearchVersion: SearchVersionHybridSerafim100mFTS5V1,
		Query:         "lugar bom pra trabalhar",
	})
	_, problems := NormalizeAndValidate(input)
	if hasProblem(problems, "payload.search_version", "unsupported") {
		t.Fatalf("hybrid search version rejected: %+v", problems)
	}
}

func TestNormalizeAndValidateRejectsPartialSearchUsefulContext(t *testing.T) {
	_, problems := NormalizeAndValidate(validInput(KindNoteMarkedUseful, NoteMarkedUsefulPayload{
		NoteID:  testNoteID,
		Context: SearchUsefulContext{Source: "search", SearchID: testSearchID, Rank: 1, RetrievalSource: RetrievalSourceLexical},
	}))
	if !hasProblem(problems, "payload.context.search_version", "required") {
		t.Fatalf("expected partial context problem, got %+v", problems)
	}
}

func validInput(kind Kind, payload Payload) Input {
	return Input{
		ID:            testEventID,
		Kind:          kind,
		OccurredAt:    1,
		UserID:        user.UserID(testUserID),
		Platform:      PlatformWeb,
		SchemaVersion: SchemaVersion1,
		Payload:       payload,
	}
}
func kindForPayload(payload Payload) Kind {
	switch payload.(type) {
	case ExploreNotesImpressionPayload:
		return KindExploreNotesImpression
	case SearchResultsImpressionPayload:
		return KindSearchResultsImpression
	default:
		panic("unknown test payload")
	}
}

func categoryPtr(value note.CategorySlug) *note.CategorySlug {
	return &value
}

func hasProblem(problems []ValidationProblem, field, code string) bool {
	for _, problem := range problems {
		if problem.Field == field && (code == "" || problem.Code == code) {
			return true
		}
	}
	return false
}
