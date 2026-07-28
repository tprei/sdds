package event

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tprei/sdds/services/api/internal/note"
)

const (
	minOccurredAt = int64(1)
	maxOccurredAt = int64(253402300799999)
)

func NormalizeAndValidate(input Input) (Record, []ValidationProblem) {
	problems := make([]ValidationProblem, 0)
	if input.ID == "" {
		problems = append(problems, ValidationProblem{Field: "id", Code: "required"})
	} else if !isCanonicalUUID(input.ID) {
		problems = append(problems, ValidationProblem{Field: "id", Code: "invalid"})
	}

	if input.Kind == "" {
		problems = append(problems, ValidationProblem{Field: "kind", Code: "required"})
	} else if !isKnownKind(input.Kind) {
		problems = append(problems, ValidationProblem{Field: "kind", Code: "unknown"})
	}

	if input.OccurredAt < minOccurredAt || input.OccurredAt > maxOccurredAt {
		problems = append(problems, ValidationProblem{Field: "occurred_at", Code: "invalid"})
	}
	if input.UserID == "" {
		problems = append(problems, ValidationProblem{Field: "user_id", Code: "required"})
	} else if !isCanonicalUUID(string(input.UserID)) {
		problems = append(problems, ValidationProblem{Field: "user_id", Code: "invalid"})
	}
	installationID := cloneString(input.InstallationID)
	if installationID != nil {
		if *installationID == "" {
			problems = append(problems, ValidationProblem{Field: "installation_id", Code: "required"})
		} else if !isCanonicalUUID(*installationID) {
			problems = append(problems, ValidationProblem{Field: "installation_id", Code: "invalid"})
		}
	}
	if input.Platform == "" {
		problems = append(problems, ValidationProblem{Field: "platform", Code: "required"})
	} else if !isPlatform(input.Platform) {
		problems = append(problems, ValidationProblem{Field: "platform", Code: "invalid"})
	}
	appVersion := cloneString(input.AppVersion)
	if appVersion != nil {
		length := utf8.RuneCountInString(*appVersion)
		if length == 0 {
			problems = append(problems, ValidationProblem{Field: "app_version", Code: "required"})
		} else if length > 64 {
			problems = append(problems, ValidationProblem{Field: "app_version", Code: "too_long"})
		}
	}
	if input.SchemaVersion != SchemaVersion1 {
		problems = append(problems, ValidationProblem{Field: "schema_version", Code: "unsupported"})
	}

	payload, payloadProblems := normalizePayload(input.Kind, input.Payload)
	problems = append(problems, payloadProblems...)
	if len(problems) == 0 {
		payloadJSON, err := marshalPayload(payload)
		if err != nil {
			problems = append(problems, ValidationProblem{Field: "payload", Code: "invalid"})
		} else if len(payloadJSON) > payloadMaxBytes {
			problems = append(problems, ValidationProblem{Field: "payload", Code: "too_large"})
		}
	}
	if len(problems) > 0 {
		return Record{}, problems
	}

	return Record{
		ID:             input.ID,
		Kind:           input.Kind,
		OccurredAt:     time.UnixMilli(input.OccurredAt).UTC(),
		UserID:         input.UserID,
		InstallationID: installationID,
		Platform:       input.Platform,
		AppVersion:     appVersion,
		SchemaVersion:  input.SchemaVersion,
		Payload:        payload,
	}, nil
}

func normalizePayload(kind Kind, payload Payload) (Payload, []ValidationProblem) {
	if payload == nil {
		return nil, []ValidationProblem{{Field: "payload", Code: "required"}}
	}

	switch kind {
	case KindExploreNotesImpression:
		value, ok := payload.(ExploreNotesImpressionPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := validateExploreImpression(&value)
		return value, problems
	case KindExploreNoteOpened:
		value, ok := payload.(ExploreNoteOpenedPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.note_id", value.NoteID)
		problems = appendRankProblem(problems, "payload.rank", value.Rank)
		problems = appendCategoryProblem(problems, "payload.category_slug", value.CategorySlug)
		return value, problems
	case KindSearchSubmitted:
		value, ok := payload.(SearchSubmittedPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.search_id", value.SearchID)
		problems = appendSearchVersionProblem(problems, "payload.search_version", value.SearchVersion)
		value.Query, problems = normalizeQuery(problems, "payload.query", value.Query)
		problems = appendCategoryProblem(problems, "payload.category_slug", value.CategorySlug)
		return value, problems
	case KindSearchResultsImpression:
		value, ok := payload.(SearchResultsImpressionPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.search_id", value.SearchID)
		problems = appendSearchVersionProblem(problems, "payload.search_version", value.SearchVersion)
		value.Query, problems = normalizeQuery(problems, "payload.query", value.Query)
		problems = appendCategoryProblem(problems, "payload.category_slug", value.CategorySlug)
		problems = append(problems, validateSearchResults(value.ResultCount, value.Results)...)
		return value, problems
	case KindSearchResultOpened:
		value, ok := payload.(SearchResultOpenedPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.search_id", value.SearchID)
		problems = appendSearchVersionProblem(problems, "payload.search_version", value.SearchVersion)
		problems = appendUUIDProblem(problems, "payload.note_id", value.NoteID)
		problems = appendRankProblem(problems, "payload.rank", value.Rank)
		problems = appendRetrievalSourceProblem(problems, "payload.retrieval_source", value.RetrievalSource)
		return value, problems
	case KindSearchReformulated:
		value, ok := payload.(SearchReformulatedPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.previous_search_id", value.PreviousSearchID)
		problems = appendSearchVersionProblem(problems, "payload.previous_search_version", value.PreviousSearchVersion)
		problems = appendUUIDProblem(problems, "payload.search_id", value.SearchID)
		problems = appendSearchVersionProblem(problems, "payload.search_version", value.SearchVersion)
		value.PreviousQuery, problems = normalizeQuery(problems, "payload.previous_query", value.PreviousQuery)
		value.Query, problems = normalizeQuery(problems, "payload.query", value.Query)
		problems = appendCategoryProblem(problems, "payload.previous_category_slug", value.PreviousCategorySlug)
		problems = appendCategoryProblem(problems, "payload.category_slug", value.CategorySlug)
		return value, problems
	case KindSearchNoResults:
		value, ok := payload.(SearchNoResultsPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.search_id", value.SearchID)
		problems = appendSearchVersionProblem(problems, "payload.search_version", value.SearchVersion)
		value.Query, problems = normalizeQuery(problems, "payload.query", value.Query)
		problems = appendCategoryProblem(problems, "payload.category_slug", value.CategorySlug)
		if value.ResultCount != 0 {
			problems = append(problems, ValidationProblem{Field: "payload.result_count", Code: "invalid"})
		}
		return value, problems
	case KindNoteMarkedUseful:
		value, ok := payload.(NoteMarkedUsefulPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		return value, validateUsefulPayload(value.NoteID, value.Context)
	case KindNoteUnmarkedUseful:
		value, ok := payload.(NoteUnmarkedUsefulPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		return value, validateUsefulPayload(value.NoteID, value.Context)
	case KindCommentCreated:
		value, ok := payload.(CommentCreatedPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.note_id", value.NoteID)
		problems = appendUUIDProblem(problems, "payload.comment_id", string(value.CommentID))
		return value, problems
	case KindReportCreated:
		value, ok := payload.(ReportCreatedPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.report_id", string(value.ReportID))
		if value.TargetType != "note" && value.TargetType != "comment" {
			problems = append(problems, ValidationProblem{Field: "payload.target_type", Code: "invalid"})
		}
		problems = appendUUIDProblem(problems, "payload.target_id", value.TargetID)
		return value, problems
	case KindNotePublished:
		value, ok := payload.(NotePublishedPayload)
		if !ok {
			return payload, payloadMismatch()
		}
		problems := make([]ValidationProblem, 0)
		problems = appendUUIDProblem(problems, "payload.note_id", value.NoteID)
		problems = appendCategoryValueProblem(problems, "payload.category_slug", value.CategorySlug)
		return value, problems
	default:
		return payload, nil
	}
}

func validateExploreImpression(value *ExploreNotesImpressionPayload) []ValidationProblem {
	problems := make([]ValidationProblem, 0)
	problems = appendCategoryProblem(problems, "payload.category_slug", value.CategorySlug)
	problems = append(problems, validateExploreResults(value.ResultCount, value.Results)...)
	return problems
}

func validateExploreResults(resultCount int, results []ExploreResult) []ValidationProblem {
	problems := make([]ValidationProblem, 0)
	if resultCount < 0 {
		problems = append(problems, ValidationProblem{Field: "payload.result_count", Code: "invalid"})
	} else if resultCount > resultMaxCount {
		problems = append(problems, ValidationProblem{Field: "payload.result_count", Code: "too_large"})
	}
	if results == nil {
		problems = append(problems, ValidationProblem{Field: "payload.results", Code: "required"})
	} else if resultCount != len(results) {
		problems = append(problems, ValidationProblem{Field: "payload.result_count", Code: "invalid"})
	}
	seenIDs := make(map[string]struct{}, len(results))
	seenRanks := make(map[int]struct{}, len(results))
	for index, result := range results {
		field := "payload.results[" + itoa(index) + "]"
		problems = appendUUIDProblem(problems, field+".note_id", result.NoteID)
		problems = appendRankProblem(problems, field+".rank", result.Rank)
		if _, ok := seenIDs[result.NoteID]; ok {
			problems = append(problems, ValidationProblem{Field: field + ".note_id", Code: "invalid"})
		}
		seenIDs[result.NoteID] = struct{}{}
		if _, ok := seenRanks[result.Rank]; ok {
			problems = append(problems, ValidationProblem{Field: field + ".rank", Code: "invalid"})
		}
		seenRanks[result.Rank] = struct{}{}
		if result.Rank != index+1 {
			problems = append(problems, ValidationProblem{Field: field + ".rank", Code: "invalid"})
		}
	}
	return problems
}

func validateSearchResults(resultCount int, results []SearchResult) []ValidationProblem {
	problems := make([]ValidationProblem, 0)
	if resultCount < 0 {
		problems = append(problems, ValidationProblem{Field: "payload.result_count", Code: "invalid"})
	} else if resultCount > resultMaxCount {
		problems = append(problems, ValidationProblem{Field: "payload.result_count", Code: "too_large"})
	}
	if results == nil {
		problems = append(problems, ValidationProblem{Field: "payload.results", Code: "required"})
	} else if resultCount != len(results) {
		problems = append(problems, ValidationProblem{Field: "payload.result_count", Code: "invalid"})
	}
	seenIDs := make(map[string]struct{}, len(results))
	seenRanks := make(map[int]struct{}, len(results))
	for index, result := range results {
		field := "payload.results[" + itoa(index) + "]"
		problems = appendUUIDProblem(problems, field+".note_id", result.NoteID)
		problems = appendRankProblem(problems, field+".rank", result.Rank)
		problems = appendRetrievalSourceProblem(problems, field+".retrieval_source", result.RetrievalSource)
		if _, ok := seenIDs[result.NoteID]; ok {
			problems = append(problems, ValidationProblem{Field: field + ".note_id", Code: "invalid"})
		}
		seenIDs[result.NoteID] = struct{}{}
		if _, ok := seenRanks[result.Rank]; ok {
			problems = append(problems, ValidationProblem{Field: field + ".rank", Code: "invalid"})
		}
		seenRanks[result.Rank] = struct{}{}
		if result.Rank != index+1 {
			problems = append(problems, ValidationProblem{Field: field + ".rank", Code: "invalid"})
		}
	}
	return problems
}

func validateUsefulPayload(noteID string, context UsefulContext) []ValidationProblem {
	problems := make([]ValidationProblem, 0)
	problems = appendUUIDProblem(problems, "payload.note_id", noteID)
	if context == nil {
		return append(problems, ValidationProblem{Field: "payload.context", Code: "required"})
	}
	switch value := context.(type) {
	case SearchUsefulContext:
		if value.Source != "search" {
			problems = append(problems, ValidationProblem{Field: "payload.context.source", Code: "invalid"})
		}
		problems = appendUUIDProblem(problems, "payload.context.search_id", value.SearchID)
		problems = appendSearchVersionProblem(problems, "payload.context.search_version", value.SearchVersion)
		problems = appendRankProblem(problems, "payload.context.rank", value.Rank)
		problems = appendRetrievalSourceProblem(problems, "payload.context.retrieval_source", value.RetrievalSource)
	case ExploreUsefulContext:
		if value.Source != "explore" {
			problems = append(problems, ValidationProblem{Field: "payload.context.source", Code: "invalid"})
		}
		problems = appendRankProblem(problems, "payload.context.rank", value.Rank)
		problems = appendCategoryProblem(problems, "payload.context.category_slug", value.CategorySlug)
	case NoteDetailUsefulContext:
		if value.Source != "note_detail" {
			problems = append(problems, ValidationProblem{Field: "payload.context.source", Code: "invalid"})
		}
	case AuthorProfileUsefulContext:
		if value.Source != "author_profile" {
			problems = append(problems, ValidationProblem{Field: "payload.context.source", Code: "invalid"})
		}
	default:
		problems = append(problems, ValidationProblem{Field: "payload.context", Code: "invalid"})
	}
	return problems
}

func normalizeQuery(problems []ValidationProblem, field, query string) (string, []ValidationProblem) {
	query = strings.TrimSpace(query)
	length := utf8.RuneCountInString(query)
	if length == 0 {
		problems = append(problems, ValidationProblem{Field: field, Code: "required"})
	} else if length > note.SearchQueryMaxLength {
		problems = append(problems, ValidationProblem{Field: field, Code: "too_long"})
	}
	return query, problems
}

func appendUUIDProblem(problems []ValidationProblem, field, value string) []ValidationProblem {
	if value == "" {
		return append(problems, ValidationProblem{Field: field, Code: "required"})
	}
	if !isCanonicalUUID(value) {
		return append(problems, ValidationProblem{Field: field, Code: "invalid"})
	}
	return problems
}

func appendCategoryProblem(problems []ValidationProblem, field string, value *note.CategorySlug) []ValidationProblem {
	if value == nil {
		return problems
	}
	return appendCategoryValueProblem(problems, field, *value)
}

func appendCategoryValueProblem(problems []ValidationProblem, field string, value note.CategorySlug) []ValidationProblem {
	if value == "" {
		return append(problems, ValidationProblem{Field: field, Code: "required"})
	}
	for _, category := range note.Categories {
		if category.Active && category.Slug == value {
			return problems
		}
	}
	return append(problems, ValidationProblem{Field: field, Code: "invalid"})
}

func appendSearchVersionProblem(problems []ValidationProblem, field string, value SearchVersion) []ValidationProblem {
	if value == "" {
		return append(problems, ValidationProblem{Field: field, Code: "required"})
	}
	if value != SearchVersionFTS5V1 {
		return append(problems, ValidationProblem{Field: field, Code: "unsupported"})
	}
	return problems
}

func appendRetrievalSourceProblem(problems []ValidationProblem, field string, value RetrievalSource) []ValidationProblem {
	if value == "" {
		return append(problems, ValidationProblem{Field: field, Code: "required"})
	}
	if value != RetrievalSourceLexical && value != RetrievalSourceSemantic && value != RetrievalSourceHybrid {
		return append(problems, ValidationProblem{Field: field, Code: "invalid"})
	}
	return problems
}

func appendRankProblem(problems []ValidationProblem, field string, rank int) []ValidationProblem {
	if rank < 1 || rank > resultMaxCount {
		return append(problems, ValidationProblem{Field: field, Code: "invalid"})
	}
	return problems
}

func payloadMismatch() []ValidationProblem {
	return []ValidationProblem{{Field: "payload", Code: "invalid"}}
}

func isKnownKind(kind Kind) bool {
	switch kind {
	case KindExploreNotesImpression, KindExploreNoteOpened, KindSearchSubmitted, KindSearchResultsImpression,
		KindSearchResultOpened, KindSearchReformulated, KindSearchNoResults, KindNoteMarkedUseful,
		KindNoteUnmarkedUseful, KindCommentCreated, KindReportCreated, KindNotePublished:
		return true
	default:
		return false
	}
}

func isPlatform(platform Platform) bool {
	return platform == PlatformIOS || platform == PlatformAndroid || platform == PlatformWeb
}

func isCanonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == value
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
