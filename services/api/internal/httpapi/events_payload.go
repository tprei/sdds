package httpapi

import (
	"encoding/json"

	"github.com/tprei/sdds/services/api/internal/comment"
	"github.com/tprei/sdds/services/api/internal/event"
	"github.com/tprei/sdds/services/api/internal/openapi"
	"github.com/tprei/sdds/services/api/internal/report"
)

func decodeEventPayload(kind event.Kind, rawPayload json.RawMessage, index int) (event.Payload, []openapi.InvalidEventProblem) {
	fields, ok := objectFields(rawPayload)
	if !ok {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload", Code: openapi.Invalid}}
	}
	problems := make([]openapi.InvalidEventProblem, 0)
	allowed := payloadFields(kind)
	if allowed == nil {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload", Code: openapi.Invalid}}
	}
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			problems = append(problems, invalidEventProblem(index, "payload."+field, "unknown"))
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}

	switch kind {
	case event.KindExploreNotesImpression:
		category, categoryProblems := nullableCategory(fields, "category_slug", index)
		resultCount, _ := requiredPayloadInt(fields, "result_count", index, &problems)
		results, resultsProblems := decodeExploreResults(fields, "results", index)
		problems = append(problems, categoryProblems...)
		problems = append(problems, resultsProblems...)
		return event.ExploreNotesImpressionPayload{CategorySlug: category, ResultCount: resultCount, Results: results}, problems
	case event.KindExploreNoteOpened:
		noteID, _ := requiredPayloadString(fields, "note_id", index, &problems)
		rank, _ := requiredPayloadInt(fields, "rank", index, &problems)
		category, categoryProblems := nullableCategory(fields, "category_slug", index)
		problems = append(problems, categoryProblems...)
		return event.ExploreNoteOpenedPayload{NoteID: noteID, Rank: rank, CategorySlug: category}, problems
	case event.KindSearchSubmitted:
		searchID, _ := requiredPayloadString(fields, "search_id", index, &problems)
		searchVersion, _ := requiredPayloadString(fields, "search_version", index, &problems)
		query, _ := requiredPayloadString(fields, "query", index, &problems)
		category, categoryProblems := nullableCategory(fields, "category_slug", index)
		problems = append(problems, categoryProblems...)
		return event.SearchSubmittedPayload{SearchID: searchID, SearchVersion: event.SearchVersion(searchVersion), Query: query, CategorySlug: category}, problems
	case event.KindSearchResultsImpression:
		searchID, _ := requiredPayloadString(fields, "search_id", index, &problems)
		searchVersion, _ := requiredPayloadString(fields, "search_version", index, &problems)
		query, _ := requiredPayloadString(fields, "query", index, &problems)
		category, categoryProblems := nullableCategory(fields, "category_slug", index)
		resultCount, _ := requiredPayloadInt(fields, "result_count", index, &problems)
		results, resultsProblems := decodeSearchResults(fields, "results", index)
		problems = append(problems, categoryProblems...)
		problems = append(problems, resultsProblems...)
		return event.SearchResultsImpressionPayload{SearchID: searchID, SearchVersion: event.SearchVersion(searchVersion), Query: query, CategorySlug: category, ResultCount: resultCount, Results: results}, problems
	case event.KindSearchResultOpened:
		searchID, _ := requiredPayloadString(fields, "search_id", index, &problems)
		searchVersion, _ := requiredPayloadString(fields, "search_version", index, &problems)
		noteID, _ := requiredPayloadString(fields, "note_id", index, &problems)
		rank, _ := requiredPayloadInt(fields, "rank", index, &problems)
		source, _ := requiredPayloadString(fields, "retrieval_source", index, &problems)
		return event.SearchResultOpenedPayload{SearchID: searchID, SearchVersion: event.SearchVersion(searchVersion), NoteID: noteID, Rank: rank, RetrievalSource: event.RetrievalSource(source)}, problems
	case event.KindSearchReformulated:
		previousSearchID, _ := requiredPayloadString(fields, "previous_search_id", index, &problems)
		previousVersion, _ := requiredPayloadString(fields, "previous_search_version", index, &problems)
		searchID, _ := requiredPayloadString(fields, "search_id", index, &problems)
		searchVersion, _ := requiredPayloadString(fields, "search_version", index, &problems)
		previousQuery, _ := requiredPayloadString(fields, "previous_query", index, &problems)
		query, _ := requiredPayloadString(fields, "query", index, &problems)
		previousCategory, previousCategoryProblems := nullableCategory(fields, "previous_category_slug", index)
		category, categoryProblems := nullableCategory(fields, "category_slug", index)
		problems = append(problems, previousCategoryProblems...)
		problems = append(problems, categoryProblems...)
		return event.SearchReformulatedPayload{PreviousSearchID: previousSearchID, PreviousSearchVersion: event.SearchVersion(previousVersion), SearchID: searchID, SearchVersion: event.SearchVersion(searchVersion), PreviousQuery: previousQuery, Query: query, PreviousCategorySlug: previousCategory, CategorySlug: category}, problems
	case event.KindSearchNoResults:
		searchID, _ := requiredPayloadString(fields, "search_id", index, &problems)
		searchVersion, _ := requiredPayloadString(fields, "search_version", index, &problems)
		query, _ := requiredPayloadString(fields, "query", index, &problems)
		category, categoryProblems := nullableCategory(fields, "category_slug", index)
		resultCount, _ := requiredPayloadInt(fields, "result_count", index, &problems)
		problems = append(problems, categoryProblems...)
		return event.SearchNoResultsPayload{SearchID: searchID, SearchVersion: event.SearchVersion(searchVersion), Query: query, CategorySlug: category, ResultCount: resultCount}, problems
	case event.KindNoteMarkedUseful, event.KindNoteUnmarkedUseful:
		noteID, _ := requiredPayloadString(fields, "note_id", index, &problems)
		contextValue, contextProblems := decodeUsefulContext(fields["context"], index)
		problems = append(problems, contextProblems...)
		if kind == event.KindNoteMarkedUseful {
			return event.NoteMarkedUsefulPayload{NoteID: noteID, Context: contextValue}, problems
		}
		return event.NoteUnmarkedUsefulPayload{NoteID: noteID, Context: contextValue}, problems
	case event.KindCommentCreated:
		noteID, _ := requiredPayloadString(fields, "note_id", index, &problems)
		commentID, _ := requiredPayloadString(fields, "comment_id", index, &problems)
		parentCommentID, _ := nullablePayloadString(fields, "parent_comment_id", index, &problems)
		payload := event.CommentCreatedPayload{NoteID: noteID, CommentID: comment.CommentID(commentID)}
		if parentCommentID != nil {
			payload.ParentCommentID = comment.CommentID(*parentCommentID)
		}
		return payload, problems
	case event.KindReportCreated:
		reportID, _ := requiredPayloadString(fields, "report_id", index, &problems)
		targetType, _ := requiredPayloadString(fields, "target_type", index, &problems)
		targetID, _ := requiredPayloadString(fields, "target_id", index, &problems)
		return event.ReportCreatedPayload{ReportID: report.ID(reportID), TargetType: report.TargetType(targetType), TargetID: targetID}, problems
	case event.KindNotePublished:
		noteID, _ := requiredPayloadString(fields, "note_id", index, &problems)
		category, categoryProblems := requiredCategory(fields, "category_slug", index)
		problems = append(problems, categoryProblems...)
		return event.NotePublishedPayload{NoteID: noteID, CategorySlug: category}, problems
	default:
		return nil, problems
	}
}

func decodeUsefulContext(raw json.RawMessage, index int) (event.UsefulContext, []openapi.InvalidEventProblem) {
	if len(raw) == 0 {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload.context", Code: openapi.Required}}
	}
	fields, ok := objectFields(raw)
	if !ok {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload.context", Code: openapi.Invalid}}
	}
	problems := make([]openapi.InvalidEventProblem, 0)
	source, sourceOK := requiredStringAt(fields, "source", index, "payload.context.", &problems)
	if !sourceOK {
		return nil, problems
	}
	switch source {
	case "search":
		checkAllowedContextKeys(fields, []string{"source", "search_id", "search_version", "rank", "retrieval_source"}, index, "payload.context", &problems)
		searchID, _ := requiredStringAt(fields, "search_id", index, "payload.context.", &problems)
		version, _ := requiredStringAt(fields, "search_version", index, "payload.context.", &problems)
		rank, _ := requiredIntAt(fields, "rank", index, "payload.context", &problems)
		retrievalSource, _ := requiredStringAt(fields, "retrieval_source", index, "payload.context.", &problems)
		return event.SearchUsefulContext{Source: source, SearchID: searchID, SearchVersion: event.SearchVersion(version), Rank: rank, RetrievalSource: event.RetrievalSource(retrievalSource)}, problems
	case "explore":
		checkAllowedContextKeys(fields, []string{"source", "rank", "category_slug"}, index, "payload.context", &problems)
		rank, _ := requiredIntAt(fields, "rank", index, "payload.context", &problems)
		category, categoryProblems := nullableCategoryAt(fields, "category_slug", index, "payload.context.")
		problems = append(problems, categoryProblems...)
		return event.ExploreUsefulContext{Source: source, Rank: rank, CategorySlug: category}, problems
	case "note_detail":
		checkAllowedContextKeys(fields, []string{"source"}, index, "payload.context", &problems)
		return event.NoteDetailUsefulContext{Source: source}, problems
	case "author_profile":
		checkAllowedContextKeys(fields, []string{"source"}, index, "payload.context", &problems)
		return event.AuthorProfileUsefulContext{Source: source}, problems
	default:
		problems = append(problems, invalidEventProblem(index, "payload.context.source", "invalid"))
		return nil, problems
	}
}

func decodeExploreResults(fields map[string]json.RawMessage, name string, index int) ([]event.ExploreResult, []openapi.InvalidEventProblem) {
	var rawResults []json.RawMessage
	problems := make([]openapi.InvalidEventProblem, 0)
	raw, ok := fields[name]
	if !ok {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload." + name, Code: openapi.Required}}
	}
	if err := json.Unmarshal(raw, &rawResults); err != nil {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload." + name, Code: openapi.Invalid}}
	}
	if rawResults == nil {
		return nil, problems
	}
	results := make([]event.ExploreResult, 0, len(rawResults))
	for resultIndex, rawResult := range rawResults {
		resultFields, ok := objectFields(rawResult)
		field := "payload." + name + "[" + decimalString(resultIndex) + "]"
		if !ok {
			problems = append(problems, invalidEventProblem(index, field, "invalid"))
			continue
		}
		checkAllowedContextKeys(resultFields, []string{"note_id", "rank"}, index, field, &problems)
		noteID, _ := requiredStringAt(resultFields, "note_id", index, field+".", &problems)
		rank, _ := requiredIntAt(resultFields, "rank", index, field, &problems)
		results = append(results, event.ExploreResult{NoteID: noteID, Rank: rank})
	}
	return results, problems
}

func decodeSearchResults(fields map[string]json.RawMessage, name string, index int) ([]event.SearchResult, []openapi.InvalidEventProblem) {
	var rawResults []json.RawMessage
	problems := make([]openapi.InvalidEventProblem, 0)
	raw, ok := fields[name]
	if !ok {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload." + name, Code: openapi.Required}}
	}
	if err := json.Unmarshal(raw, &rawResults); err != nil {
		return nil, []openapi.InvalidEventProblem{{Index: index, Field: "payload." + name, Code: openapi.Invalid}}
	}
	if rawResults == nil {
		return nil, problems
	}
	results := make([]event.SearchResult, 0, len(rawResults))
	for resultIndex, rawResult := range rawResults {
		resultFields, ok := objectFields(rawResult)
		field := "payload." + name + "[" + decimalString(resultIndex) + "]"
		if !ok {
			problems = append(problems, invalidEventProblem(index, field, "invalid"))
			continue
		}
		checkAllowedContextKeys(resultFields, []string{"note_id", "rank", "retrieval_source"}, index, field, &problems)
		noteID, _ := requiredStringAt(resultFields, "note_id", index, field+".", &problems)
		rank, _ := requiredIntAt(resultFields, "rank", index, field, &problems)
		source, _ := requiredStringAt(resultFields, "retrieval_source", index, field+".", &problems)
		results = append(results, event.SearchResult{NoteID: noteID, Rank: rank, RetrievalSource: event.RetrievalSource(source)})
	}
	return results, problems
}
