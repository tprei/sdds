const deletedNoteLimit = 100;

const deletedNoteIDs = new Set<string>();
const insertionOrder: string[] = [];

export function markNoteDeleted(noteID: string): void {
  if (typeof noteID !== 'string' || noteID.length === 0) {
    return;
  }
  if (deletedNoteIDs.has(noteID)) {
    return;
  }
  while (insertionOrder.length >= deletedNoteLimit) {
    const oldest = insertionOrder.shift();
    if (oldest !== undefined) {
      deletedNoteIDs.delete(oldest);
    }
  }
  deletedNoteIDs.add(noteID);
  insertionOrder.push(noteID);
}

export function isNoteDeleted(noteID: string): boolean {
  return deletedNoteIDs.has(noteID);
}
