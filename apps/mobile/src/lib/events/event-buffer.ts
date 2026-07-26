import { APIRequestError } from '@/lib/api/request-error';
import type { EventsAPI } from '@/lib/api/events';
import type { ProductEvent } from './event-types';
const eventBatchSize = 25;
const eventQueueLimit = 100;
const coalesceDelayMilliseconds = 250;
const maxRetryAfterSeconds = 5;
const maxAttempts = 3;
type EventBufferTransport = Pick<EventsAPI, 'createEvents'>;
type Timer = ReturnType<typeof setTimeout>;
export type EventBuffer = {
  dispose(): void;
  enqueue(event: ProductEvent): boolean;
  flush(): void;
};
export function createEventBuffer(transport: EventBufferTransport): EventBuffer {
  return new BufferedEvents(transport);
}
class BufferedEvents implements EventBuffer {
  private readonly queue: ProductEvent[] = [];
  private flushTimer: Timer | null = null;
  private isDisposed = false;
  private isSending = false;
  constructor(private readonly transport: EventBufferTransport) {}

  enqueue(event: ProductEvent): boolean {
    if (this.isDisposed || this.queue.length >= eventQueueLimit) return false;
    this.queue.push(event);
    if (!this.isSending) this.scheduleFlush(coalesceDelayMilliseconds);
    return true;
  }
  flush(): void {
    if (this.flushTimer !== null) {
      clearTimeout(this.flushTimer);
      this.flushTimer = null;
    }
    if (this.isDisposed || this.isSending || this.queue.length === 0) return;
    this.isSending = true;
    void this.sendQueuedEvents().finally(() => {
      this.isSending = false;
      if (!this.isDisposed && this.queue.length > 0) this.scheduleFlush(0);
    });
  }
  dispose(): void {
    this.isDisposed = true;
    if (this.flushTimer !== null) clearTimeout(this.flushTimer);
    this.flushTimer = null;
    this.queue.length = 0;
  }
  private async sendQueuedEvents(): Promise<void> {
    while (!this.isDisposed && this.queue.length > 0) {
      await this.sendBatch(this.queue.slice(0, eventBatchSize), 1);
    }
  }
  private async sendBatch(batch: ProductEvent[], attempt: number): Promise<void> {
    if (this.isDisposed || batch.length === 0) return;
    try {
      const receipt = await this.transport.createEvents(batch);
      if (receipt.accepted_count + receipt.duplicate_count !== batch.length) {
        throw new Error('events_receipt_invalid');
      }
      this.removeQueuePrefix(batch.length);
    } catch (error: unknown) {
      if (this.isDisposed) return;
      const status = error instanceof APIRequestError ? error.status : undefined;
      if (status === 400 && error instanceof APIRequestError) {
        await this.dropInvalidEvents(batch, error, attempt);
      } else if (status === 413) {
        await this.splitOversizedBatch(batch, attempt);
      } else if (isRetryable(status) && attempt < maxAttempts) {
        await waitFor(retryDelayMilliseconds(error, attempt));
        await this.sendBatch(batch, attempt + 1);
      } else {
        this.removeQueuePrefix(batch.length);
      }
    }
  }
  private async dropInvalidEvents(batch: ProductEvent[], error: APIRequestError, attempt: number): Promise<void> {
    const invalidIndexes = new Set(
      (error.eventProblems ?? []).map((problem) => problem.index).filter(
        (index) => Number.isInteger(index) && index >= 0 && index < batch.length,
      ),
    );
    if (invalidIndexes.size === 0) {
      this.removeQueuePrefix(batch.length);
      return;
    }
    const validEvents = batch.filter((_, index) => !invalidIndexes.has(index));
    this.removeQueuePrefix(batch.length);
    this.queue.unshift(...validEvents);
    await this.sendBatch(validEvents, attempt);
  }
  private async splitOversizedBatch(batch: ProductEvent[], attempt: number): Promise<void> {
    if (batch.length === 1) {
      this.removeQueuePrefix(1);
      return;
    }
    const midpoint = Math.ceil(batch.length / 2);
    await this.sendBatch(batch.slice(0, midpoint), attempt);
    if (!this.isDisposed) await this.sendBatch(batch.slice(midpoint), attempt);
  }
  private removeQueuePrefix(count: number): void { this.queue.splice(0, count); }
  private scheduleFlush(delayMilliseconds: number): void {
    if (this.isDisposed || this.flushTimer !== null) return;
    this.flushTimer = setTimeout(() => {
      this.flushTimer = null;
      this.flush();
    }, delayMilliseconds);
  }
}
function isRetryable(status: number | undefined): boolean {
  return status === undefined || status === 408 || status === 429 || (status !== undefined && status >= 500);
}
function retryDelayMilliseconds(error: unknown, attempt: number): number {
  if (error instanceof APIRequestError && error.retryAfter !== undefined) {
    return Math.min(error.retryAfter, maxRetryAfterSeconds) * 1000;
  }
  return attempt === 1 ? 250 : 500;
}
function waitFor(delayMilliseconds: number): Promise<void> {
  const { promise, resolve } = Promise.withResolvers<void>();
  setTimeout(resolve, delayMilliseconds);
  return promise;
}
