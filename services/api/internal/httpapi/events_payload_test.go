package httpapi

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func rawJSON(t *testing.T, value string) json.RawMessage {
	t.Helper()
	raw := json.RawMessage(value)
	if !json.Valid(raw) {
		t.Fatalf("test fixture is not valid JSON: %s", value)
	}
	return raw
}

func TestDecodeEventPayloadDecodesValidEvents(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		kind    event.Kind
		payload string
		check   func(event.Payload) bool
	}{
		{
			name:    "note_published",
			kind:    event.KindNotePublished,
			payload: `{"note_id":"note-1","category_slug":"tech"}`,
			check: func(payload event.Payload) bool {
				published, ok := payload.(event.NotePublishedPayload)
				return ok && published.NoteID == "note-1" && published.CategorySlug == "tech"
			},
		},
		{
			name:    "explore_notes_impression_with_results",
			kind:    event.KindExploreNotesImpression,
			payload: `{"category_slug":"tech","result_count":2,"results":[{"note_id":"note-1","rank":1},{"note_id":"note-2","rank":2}]}`,
			check: func(payload event.Payload) bool {
				impression, ok := payload.(event.ExploreNotesImpressionPayload)
				return ok && impression.ResultCount == 2 && len(impression.Results) == 2 && impression.Results[1].NoteID == "note-2"
			},
		},
		{
			name:    "search_submitted",
			kind:    event.KindSearchSubmitted,
			payload: `{"search_id":"search-1","search_version":"fts5-v1","query":"feridas","category_slug":null}`,
			check: func(payload event.Payload) bool {
				submitted, ok := payload.(event.SearchSubmittedPayload)
				return ok && submitted.Query == "feridas" && submitted.CategorySlug == nil
			},
		},
		{
			name:    "search_results_impression",
			kind:    event.KindSearchResultsImpression,
			payload: `{"search_id":"search-1","search_version":"fts5-v1","query":"feridas","category_slug":"tech","result_count":1,"results":[{"note_id":"note-1","rank":1,"retrieval_source":"fts5-v1"}]}`,
			check: func(payload event.Payload) bool {
				impression, ok := payload.(event.SearchResultsImpressionPayload)
				return ok && impression.ResultCount == 1 && len(impression.Results) == 1 && impression.Results[0].RetrievalSource == event.RetrievalSource("fts5-v1")
			},
		},
		{
			name:    "note_marked_useful_with_search_context",
			kind:    event.KindNoteMarkedUseful,
			payload: `{"note_id":"note-1","context":{"source":"search","search_id":"search-1","search_version":"fts5-v1","rank":1,"retrieval_source":"fts5-v1"}}`,
			check: func(payload event.Payload) bool {
				useful, ok := payload.(event.NoteMarkedUsefulPayload)
				if !ok || useful.NoteID != "note-1" {
					return false
				}
				context, ok := useful.Context.(event.SearchUsefulContext)
				return ok && context.SearchID == "search-1" && context.Rank == 1
			},
		},
		{
			name:    "comment_created_top_level_null_parent",
			kind:    event.KindCommentCreated,
			payload: `{"note_id":"note-1","comment_id":"comment-1","parent_comment_id":null}`,
			check: func(payload event.Payload) bool {
				created, ok := payload.(event.CommentCreatedPayload)
				return ok && created.NoteID == "note-1" && created.CommentID == "comment-1" && created.ParentCommentID == ""
			},
		},
		{
			name:    "comment_created_reply_parent",
			kind:    event.KindCommentCreated,
			payload: `{"note_id":"note-1","comment_id":"reply-1","parent_comment_id":"parent-1"}`,
			check: func(payload event.Payload) bool {
				created, ok := payload.(event.CommentCreatedPayload)
				return ok && created.NoteID == "note-1" && created.CommentID == "reply-1" && created.ParentCommentID == "parent-1"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload, problems := decodeEventPayload(tc.kind, rawJSON(t, tc.payload), 0)
			if len(problems) != 0 {
				t.Fatalf("decodeEventPayload(%s) reported problems: %+v", tc.kind, problems)
			}
			if payload == nil {
				t.Fatalf("decodeEventPayload(%s) returned nil payload", tc.kind)
			}
			if !tc.check(payload) {
				t.Fatalf("decodeEventPayload(%s) returned unexpected payload: %+v", tc.kind, payload)
			}
		})
	}
}

func TestDecodeEventPayloadReportsProblems(t *testing.T) {
	t.Parallel()
	t.Run("unknown_payload_field", func(t *testing.T) {
		t.Parallel()
		_, problems := decodeEventPayload(event.KindNotePublished, rawJSON(t, `{"note_id":"note-1","category_slug":"tech","intruder":"x"}`), 3)
		if !hasProblem(t, problems, "payload.intruder", string(openapi.Unknown)) {
			t.Fatalf("expected unknown-field problem, got %+v", problems)
		}
	})
	t.Run("missing_required_payload_string", func(t *testing.T) {
		t.Parallel()
		_, problems := decodeEventPayload(event.KindNotePublished, rawJSON(t, `{"category_slug":"tech"}`), 1)
		if !hasProblem(t, problems, "payload.note_id", string(openapi.Required)) {
			t.Fatalf("expected required problem for note_id, got %+v", problems)
		}
	})
	t.Run("malformed_payload_json", func(t *testing.T) {
		t.Parallel()
		_, problems := decodeEventPayload(event.KindNotePublished, json.RawMessage(`"not-an-object"`), 2)
		if !hasProblem(t, problems, "payload", string(openapi.Invalid)) {
			t.Fatalf("expected invalid payload problem, got %+v", problems)
		}
	})
	t.Run("missing_comment_parent_comment_id", func(t *testing.T) {
		t.Parallel()
		_, problems := decodeEventPayload(event.KindCommentCreated, rawJSON(t, `{"note_id":"note-1","comment_id":"comment-1"}`), 0)
		if !hasProblem(t, problems, "payload.parent_comment_id", string(openapi.Required)) {
			t.Fatalf("expected payload.parent_comment_id/required, got %+v", problems)
		}
	})
	t.Run("malformed_comment_parent_comment_id", func(t *testing.T) {
		t.Parallel()
		_, problems := decodeEventPayload(event.KindCommentCreated, rawJSON(t, `{"note_id":"note-1","comment_id":"comment-1","parent_comment_id":123}`), 0)
		if !hasProblem(t, problems, "payload.parent_comment_id", string(openapi.Invalid)) {
			t.Fatalf("expected payload.parent_comment_id/invalid, got %+v", problems)
		}
	})
}

func TestEventJSONHelpers(t *testing.T) {
	t.Parallel()
	problems := make([]openapi.InvalidEventProblem, 0)
	fields := map[string]json.RawMessage{
		"string":   json.RawMessage(`"value"`),
		"int":      json.RawMessage(`7`),
		"int64":    json.RawMessage(`42`),
		"object":   json.RawMessage(`{"nested":{"size":3}}`),
		"raw":      json.RawMessage(`true`),
		"kind":     json.RawMessage(`"note_published"`),
		"overflow": json.RawMessage(`{"big":"xxxxxxxxxx"}`),
	}

	if value, ok := requiredString(fields, "string", 0, &problems); !ok || value != "value" {
		t.Fatalf("requiredString = %q,%v", value, ok)
	}
	if value, ok := requiredInt(fields, "int", 0, &problems); !ok || value != 7 {
		t.Fatalf("requiredInt = %d,%v", value, ok)
	}
	if value, ok := requiredInt64(fields, "int64", 0, &problems); !ok || value != 42 {
		t.Fatalf("requiredInt64 = %d,%v", value, ok)
	}
	if _, ok := requiredRaw(fields, "raw", 0, &problems); !ok {
		t.Fatal("requiredRaw reported missing for present field")
	}
	if value, ok := nullableString(fields, "absent", 0, &problems); ok || value != nil {
		t.Fatalf("nullableString(absent) = %v,%v, want nil,false", value, ok)
	}
	if !knownEventKind(event.KindNotePublished) {
		t.Fatal("knownEventKind rejected a known kind")
	}
	if knownEventKind(event.Kind("future_kind")) {
		t.Fatal("knownEventKind accepted an unknown kind")
	}

	checkEventKeys(map[string]json.RawMessage{"id": nil, "forbidden": nil}, 4, &problems)
	if !hasProblem(t, problems, "forbidden", string(openapi.Unknown)) {
		t.Fatalf("checkEventKeys did not report unknown top-level key, got %+v", problems)
	}

	if err := requireJSONEOF(json.NewDecoder(strings.NewReader(""))); err != nil {
		t.Fatalf("requireJSONEOF on exhausted stream = %v, want nil", err)
	}
	if err := requireJSONEOF(json.NewDecoder(strings.NewReader(`{}`))); err == nil {
		t.Fatal("requireJSONEOF on trailing object = nil, want error")
	}

	if size, ok := rawObjectFieldSize(fields["overflow"], "big"); !ok || size <= 0 {
		t.Fatalf("rawObjectFieldSize = %d,%v, want positive size", size, ok)
	}
	if !isJSONWhitespace(' ') || isJSONWhitespace('x') {
		t.Fatal("isJSONWhitespace misclassified whitespace")
	}
	if !bytes.Equal(bytes.TrimSpace(json.RawMessage(`  null `)), []byte("null")) {
		t.Fatal("bytes.TrimSpace precondition check failed")
	}
}

func hasProblem(t *testing.T, problems []openapi.InvalidEventProblem, field, code string) bool {
	t.Helper()
	for _, problem := range problems {
		if problem.Field == field && (code == "" || string(problem.Code) == code) {
			return true
		}
	}
	return false
}
