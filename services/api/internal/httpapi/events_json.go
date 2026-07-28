package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/note"
	"github.com/tprei/sdds/services/api/internal/openapi"
)

func objectFields(raw json.RawMessage) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, false
	}
	return fields, true
}

func rawObjectFieldSize(raw json.RawMessage, name string) (int, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return 0, false
	}
	var size int
	found := false
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return 0, false
		}
		key, ok := keyToken.(string)
		if !ok {
			return 0, false
		}
		valueStart := decoder.InputOffset()
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return 0, false
		}
		valueEnd := decoder.InputOffset()
		if key != name {
			continue
		}
		relativeColon := bytes.IndexByte(raw[valueStart:valueEnd], ':')
		if relativeColon < 0 {
			size = len(value)
			found = true
			continue
		}
		start := valueStart + int64(relativeColon) + 1
		end := valueEnd
		for end < int64(len(raw)) && isJSONWhitespace(raw[end]) {
			end++
		}
		size = int(end - start)
		found = true
	}
	return size, found
}

func isJSONWhitespace(value byte) bool {
	return value == ' ' || value == '\t' || value == '\r' || value == '\n'
}

func checkEventKeys(fields map[string]json.RawMessage, index int, problems *[]openapi.InvalidEventProblem) {
	checkAllowedKeys(fields, []string{"id", "kind", "occurred_at", "installation_id", "platform", "app_version", "schema_version", "payload"}, index, "", problems)
}

func checkAllowedContextKeys(fields map[string]json.RawMessage, allowed []string, index int, prefix string, problems *[]openapi.InvalidEventProblem) {
	checkAllowedKeys(fields, allowed, index, prefix+".", problems)
}

func checkAllowedKeys(fields map[string]json.RawMessage, allowed []string, index int, prefix string, problems *[]openapi.InvalidEventProblem) {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = struct{}{}
	}
	for field := range fields {
		if _, ok := allowedSet[field]; !ok {
			*problems = append(*problems, invalidEventProblem(index, prefix+field, "unknown"))
		}
	}
}

func payloadFields(kind event.Kind) map[string]struct{} {
	var fields []string
	switch kind {
	case event.KindExploreNotesImpression:
		fields = []string{"category_slug", "result_count", "results"}
	case event.KindExploreNoteOpened:
		fields = []string{"note_id", "rank", "category_slug"}
	case event.KindSearchSubmitted:
		fields = []string{"search_id", "search_version", "query", "category_slug"}
	case event.KindSearchResultsImpression:
		fields = []string{"search_id", "search_version", "query", "category_slug", "result_count", "results"}
	case event.KindSearchResultOpened:
		fields = []string{"search_id", "search_version", "note_id", "rank", "retrieval_source"}
	case event.KindSearchReformulated:
		fields = []string{"previous_search_id", "previous_search_version", "search_id", "search_version", "previous_query", "query", "previous_category_slug", "category_slug"}
	case event.KindSearchNoResults:
		fields = []string{"search_id", "search_version", "query", "category_slug", "result_count"}
	case event.KindNoteMarkedUseful, event.KindNoteUnmarkedUseful:
		fields = []string{"note_id", "context"}
	case event.KindCommentCreated:
		fields = []string{"note_id", "comment_id"}
	case event.KindReportCreated:
		fields = []string{"report_id", "target_type", "target_id"}
	case event.KindNotePublished:
		fields = []string{"note_id", "category_slug"}
	default:
		return nil
	}
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		result[field] = struct{}{}
	}
	return result
}

func requiredRaw(fields map[string]json.RawMessage, name string, index int, problems *[]openapi.InvalidEventProblem) (json.RawMessage, bool) {
	value, ok := fields[name]
	if !ok {
		*problems = append(*problems, invalidEventProblem(index, name, "required"))
		return nil, false
	}
	return value, true
}

func requiredString(fields map[string]json.RawMessage, name string, index int, problems *[]openapi.InvalidEventProblem) (string, bool) {
	return requiredStringAt(fields, name, index, "", problems)
}

func requiredStringAt(fields map[string]json.RawMessage, name string, index int, prefix string, problems *[]openapi.InvalidEventProblem) (string, bool) {
	value, ok := fields[name]
	if !ok {
		*problems = append(*problems, invalidEventProblem(index, prefix+name, "required"))
		return "", false
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		*problems = append(*problems, invalidEventProblem(index, prefix+name, "invalid"))
		return "", false
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		*problems = append(*problems, invalidEventProblem(index, prefix+name, "invalid"))
		return "", false
	}
	return result, true
}

func requiredPayloadString(fields map[string]json.RawMessage, name string, index int, problems *[]openapi.InvalidEventProblem) (string, bool) {
	return requiredStringAt(fields, name, index, "payload.", problems)
}

func requiredInt(fields map[string]json.RawMessage, name string, index int, problems *[]openapi.InvalidEventProblem) (int, bool) {
	return requiredIntAt(fields, name, index, "", problems)
}

func requiredIntAt(fields map[string]json.RawMessage, name string, index int, prefix string, problems *[]openapi.InvalidEventProblem) (int, bool) {
	value, ok := fields[name]
	field := name
	if prefix != "" {
		field = prefix + "." + name
	}
	if !ok {
		*problems = append(*problems, invalidEventProblem(index, field, "required"))
		return 0, false
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		*problems = append(*problems, invalidEventProblem(index, field, "invalid"))
		return 0, false
	}
	var result int
	if err := json.Unmarshal(value, &result); err != nil {
		*problems = append(*problems, invalidEventProblem(index, field, "invalid"))
		return 0, false
	}
	return result, true
}

func requiredPayloadInt(fields map[string]json.RawMessage, name string, index int, problems *[]openapi.InvalidEventProblem) (int, bool) {
	return requiredIntAt(fields, name, index, "payload", problems)
}

func requiredInt64(fields map[string]json.RawMessage, name string, index int, problems *[]openapi.InvalidEventProblem) (int64, bool) {
	value, ok := fields[name]
	if !ok {
		*problems = append(*problems, invalidEventProblem(index, name, "required"))
		return 0, false
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		*problems = append(*problems, invalidEventProblem(index, name, "invalid"))
		return 0, false
	}
	var result int64
	if err := json.Unmarshal(value, &result); err != nil {
		*problems = append(*problems, invalidEventProblem(index, name, "invalid"))
		return 0, false
	}
	return result, true
}

func nullableString(fields map[string]json.RawMessage, name string, index int, problems *[]openapi.InvalidEventProblem) (*string, bool) {
	value, ok := fields[name]
	if !ok {
		*problems = append(*problems, invalidEventProblem(index, name, "required"))
		return nil, false
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, true
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		*problems = append(*problems, invalidEventProblem(index, name, "invalid"))
		return nil, false
	}
	return &result, true
}

func nullableCategory(fields map[string]json.RawMessage, name string, index int) (*note.CategorySlug, []openapi.InvalidEventProblem) {
	return nullableCategoryAt(fields, name, index, "payload.")
}

func nullableCategoryAt(fields map[string]json.RawMessage, name string, index int, fieldPrefix string) (*note.CategorySlug, []openapi.InvalidEventProblem) {
	value, ok := fields[name]
	if !ok {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: fieldPrefix + name, Code: openapi.Required}}
	}
	if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, nil
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: fieldPrefix + name, Code: openapi.Invalid}}
	}
	category := note.CategorySlug(result)
	return &category, nil
}

func requiredCategory(fields map[string]json.RawMessage, name string, index int) (note.CategorySlug, []openapi.InvalidEventProblem) {
	value, ok := fields[name]
	if !ok {
		return "", []openapi.InvalidEventProblem{{Index: index, Field: "payload." + name, Code: openapi.Required}}
	}
	var result string
	if err := json.Unmarshal(value, &result); err != nil {
		return "", []openapi.InvalidEventProblem{{Index: index, Field: "payload." + name, Code: openapi.Invalid}}
	}
	return note.CategorySlug(result), nil
}

func invalidEventProblem(index int, field, code string) openapi.InvalidEventProblem {
	return openapi.InvalidEventProblem{Index: index, Field: field, Code: openapi.InvalidEventProblemCode(code)}
}

func knownEventKind(kind event.Kind) bool {
	switch kind {
	case event.KindExploreNotesImpression, event.KindExploreNoteOpened, event.KindSearchSubmitted, event.KindSearchResultsImpression,
		event.KindSearchResultOpened, event.KindSearchReformulated, event.KindSearchNoResults, event.KindNoteMarkedUseful,
		event.KindNoteUnmarkedUseful, event.KindCommentCreated, event.KindReportCreated, event.KindNotePublished:
		return true
	default:
		return false
	}
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON")
		}
		return err
	}
	return nil
}

func decimalString(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	position := len(digits)
	for value > 0 {
		position--
		digits[position] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[position:])
}
