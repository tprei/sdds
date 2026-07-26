import { createEventsReceiptSchema } from './schema';
import { mapProductEvent } from './event-input';
import type { TypedTransport } from './client';
import type { components } from './generated/schema';
import type { ProductEvent } from '../events/event-types';

type GeneratedSchemas = components['schemas'];
type CreateEventsReceipt = GeneratedSchemas['CreateEventsReceipt'];
export type EventsAPI = {
  createEvents(events: readonly ProductEvent[]): Promise<CreateEventsReceipt>;
};

export class EventsAPIResponseError extends Error {
  constructor() {
    super('events_api_response_invalid');
  }
}

export function bindEventsAPI(transport: TypedTransport): EventsAPI {
  return {
    async createEvents(events) {
      const { data } = await transport.POST('/v1/events', {
        body: { events: events.map(mapProductEvent) },
      });
      const receipt = createEventsReceiptSchema.safeParse(data);
      if (!receipt.success) {
        throw new EventsAPIResponseError();
      }
      return receipt.data;
    },
  };
}
