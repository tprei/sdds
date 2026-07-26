import type { components } from './generated/schema';
import {
  isProductEventOfKind,
  type ProductEvent,
  type UsefulContext,
} from '../events/event-types';

type GeneratedSchemas = components['schemas'];
type EventInput = GeneratedSchemas['EventInput'];

export function mapProductEvent(event: ProductEvent): EventInput {
  const envelope = {
    id: event.id,
    occurred_at: event.occurredAt,
    installation_id: event.installationID,
    platform: event.platform,
    app_version: event.appVersion,
    schema_version: event.schemaVersion,
  };
  if (isProductEventOfKind(event, 'explore_notes_impression')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        category_slug: event.payload.categorySlug,
        result_count: event.payload.resultCount,
        results: event.payload.results.map((result) => ({
          note_id: result.noteID,
          rank: result.rank,
        })),
      },
    };
  }
  if (isProductEventOfKind(event, 'explore_note_opened')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        note_id: event.payload.noteID,
        rank: event.payload.rank,
        category_slug: event.payload.categorySlug,
      },
    };
  }
  if (isProductEventOfKind(event, 'search_submitted')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        search_id: event.payload.searchID,
        search_version: event.payload.searchVersion,
        query: event.payload.query,
        category_slug: event.payload.categorySlug,
      },
    };
  }
  if (isProductEventOfKind(event, 'search_results_impression')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        search_id: event.payload.searchID,
        search_version: event.payload.searchVersion,
        query: event.payload.query,
        category_slug: event.payload.categorySlug,
        result_count: event.payload.resultCount,
        results: event.payload.results.map((result) => ({
          note_id: result.noteID,
          rank: result.rank,
          retrieval_source: result.retrievalSource,
        })),
      },
    };
  }
  if (isProductEventOfKind(event, 'search_result_opened')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        search_id: event.payload.searchID,
        search_version: event.payload.searchVersion,
        note_id: event.payload.noteID,
        rank: event.payload.rank,
        retrieval_source: event.payload.retrievalSource,
      },
    };
  }
  if (isProductEventOfKind(event, 'search_reformulated')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        previous_search_id: event.payload.previousSearchID,
        previous_search_version: event.payload.previousSearchVersion,
        search_id: event.payload.searchID,
        search_version: event.payload.searchVersion,
        previous_query: event.payload.previousQuery,
        query: event.payload.query,
        previous_category_slug: event.payload.previousCategorySlug,
        category_slug: event.payload.categorySlug,
      },
    };
  }
  if (isProductEventOfKind(event, 'search_no_results')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        search_id: event.payload.searchID,
        search_version: event.payload.searchVersion,
        query: event.payload.query,
        category_slug: event.payload.categorySlug,
        result_count: 0,
      },
    };
  }
  if (isProductEventOfKind(event, 'note_marked_useful')) {
    return usefulEvent(envelope, event, 'note_marked_useful');
  }
  if (isProductEventOfKind(event, 'note_unmarked_useful')) {
    return usefulEvent(envelope, event, 'note_unmarked_useful');
  }
  if (isProductEventOfKind(event, 'comment_created')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        note_id: event.payload.noteID,
        comment_id: event.payload.commentID,
      },
    };
  }
  if (isProductEventOfKind(event, 'report_created')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        report_id: event.payload.reportID,
        target_type: event.payload.targetType,
        target_id: event.payload.targetID,
      },
    };
  }
  if (isProductEventOfKind(event, 'note_published')) {
    return {
      ...envelope,
      kind: event.kind,
      payload: {
        note_id: event.payload.noteID,
        category_slug: event.payload.categorySlug,
      },
    };
  }
  throw new Error('event_kind_unknown');
}

function usefulEvent(
  envelope: {
    id: string;
    occurred_at: number;
    installation_id: string;
    platform: ProductEvent['platform'];
    app_version: string;
    schema_version: 1;
  },
  event: ProductEvent,
  kind: 'note_marked_useful' | 'note_unmarked_useful',
): EventInput {
  if (kind === 'note_marked_useful' && isProductEventOfKind(event, kind)) {
    return {
      ...envelope,
      kind,
      payload: {
        note_id: event.payload.noteID,
        context: mapUsefulContext(event.payload.context),
      },
    };
  }
  if (kind === 'note_unmarked_useful' && isProductEventOfKind(event, kind)) {
    return {
      ...envelope,
      kind,
      payload: {
        note_id: event.payload.noteID,
        context: mapUsefulContext(event.payload.context),
      },
    };
  }
  throw new Error('event_kind_mismatch');
}

function mapUsefulContext(
  context: UsefulContext,
): GeneratedSchemas['EventUsefulContext'] {
  switch (context.source) {
    case 'search':
      return {
        source: context.source,
        search_id: context.searchID,
        search_version: context.searchVersion,
        rank: context.rank,
        retrieval_source: context.retrievalSource,
      };
    case 'explore':
      return {
        source: context.source,
        rank: context.rank,
        category_slug: context.categorySlug,
      };
    case 'note_detail':
      return { source: context.source };
    case 'author_profile':
      return { source: context.source };
  }
}
