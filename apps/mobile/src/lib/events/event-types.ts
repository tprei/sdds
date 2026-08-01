export const productEventKinds = {
  exploreNotesImpression: 'explore_notes_impression',
  exploreNoteOpened: 'explore_note_opened',
  searchSubmitted: 'search_submitted',
  searchResultsImpression: 'search_results_impression',
  searchResultOpened: 'search_result_opened',
  searchReformulated: 'search_reformulated',
  searchNoResults: 'search_no_results',
  noteMarkedUseful: 'note_marked_useful',
  noteUnmarkedUseful: 'note_unmarked_useful',
  commentCreated: 'comment_created',
  reportCreated: 'report_created',
  notePublished: 'note_published',
} as const;
export type ProductEventKind =
  (typeof productEventKinds)[keyof typeof productEventKinds];
export type EventPlatform = 'ios' | 'android' | 'web';
export type SearchVersion = 'fts5-v1' | 'hybrid-serafim100m-fts5-v1';
export type RetrievalSource = 'lexical' | 'semantic' | 'hybrid';
export type ReportTargetType = 'note' | 'comment';
export type ExploreResult = { noteID: string; rank: number };
export type SearchResult = { noteID: string; rank: number; retrievalSource: RetrievalSource };
export type UsefulContext =
  | { source: 'search'; searchID: string; searchVersion: SearchVersion; rank: number; retrievalSource: RetrievalSource }
  | { source: 'explore'; rank: number; categorySlug: string | null }
  | { source: 'note_detail' }
  | { source: 'author_profile' };

export type PayloadByKind = {
  explore_notes_impression: { categorySlug: string | null; resultCount: number; results: readonly ExploreResult[] };
  explore_note_opened: { noteID: string; rank: number; categorySlug: string | null };
  search_submitted: { searchID: string; searchVersion: SearchVersion; query: string; categorySlug: string | null };
  search_results_impression: { searchID: string; searchVersion: SearchVersion; query: string; categorySlug: string | null; resultCount: number; results: readonly SearchResult[] };
  search_result_opened: { searchID: string; searchVersion: SearchVersion; noteID: string; rank: number; retrievalSource: RetrievalSource };
  search_reformulated: { previousSearchID: string; previousSearchVersion: SearchVersion; searchID: string; searchVersion: SearchVersion; previousQuery: string; query: string; previousCategorySlug: string | null; categorySlug: string | null };
  search_no_results: { searchID: string; searchVersion: SearchVersion; query: string; categorySlug: string | null; resultCount: 0 };
  note_marked_useful: { noteID: string; context: UsefulContext };
  note_unmarked_useful: { noteID: string; context: UsefulContext };
  comment_created: { noteID: string; commentID: string };
  report_created: { reportID: string; targetType: ReportTargetType; targetID: string };
  note_published: { noteID: string; categorySlug: string };
};
export type ProductEventPayload<K extends ProductEventKind> = PayloadByKind[K];
export type ProductEvent<K extends ProductEventKind = ProductEventKind> = {
  id: string; kind: K; occurredAt: number; installationID: string; platform: EventPlatform;
  appVersion: string; schemaVersion: 1; payload: PayloadByKind[K];
};
export function isProductEventOfKind<K extends ProductEventKind>(event: ProductEvent, kind: K): event is ProductEvent<K> {
  return event.kind === kind;
}
