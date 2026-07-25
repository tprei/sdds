import type { TypedTransport } from './client';
import { APIRequestError as SharedAPIRequestError } from './request-error';
import { APIRequestError, APIResponseError } from './notes';
import { reportReceiptSchema } from './schema';
import type { components } from './generated/schema';

export type ReportTargetType = 'note' | 'comment';

export type ReportReason =
  | 'spam'
  | 'harassment'
  | 'harmful_or_misleading'
  | 'other';

export type ReportReceipt = {
  id: string;
  targetType: ReportTargetType;
  targetID: string;
  reason: ReportReason;
  details: string | null;
  createdAt: number;
};

export type CreateReportInput = {
  targetType: ReportTargetType;
  targetID: string;
  reason: ReportReason;
  details?: string;
};

type GeneratedSchemas = components['schemas'];
type CreateReportRequest = GeneratedSchemas['CreateReportRequest'];
type ReportReceiptResponse = GeneratedSchemas['ReportReceipt'];

export type ReportsAPI = {
  createReport(input: CreateReportInput): Promise<ReportReceipt>;
};

export function bindReportsAPI(transport: TypedTransport): ReportsAPI {
  return {
    async createReport(input) {
      const request: CreateReportRequest = {
        target_type: input.targetType,
        target_id: input.targetID,
        reason: input.reason,
        ...(input.details !== undefined && input.details.trim() !== ''
          ? { details: input.details.trim() }
          : {}),
      };
      try {
        const { data } = await transport.POST('/v1/reports', {
          body: request,
        });
        return parseReportReceipt(data);
      } catch (error) {
        rewrapTransportError(error);
      }
    },
  };
}

function rewrapTransportError(error: unknown): never {
  if (error instanceof SharedAPIRequestError) {
    throw new APIRequestError(error.status, error.body, error.retryAfter);
  }
  throw error;
}

function parseReportReceipt(value: unknown): ReportReceipt {
  const response = reportReceiptSchema.safeParse(value);
  if (!response.success) {
    throw new APIResponseError();
  }
  return mapReportReceipt(response.data);
}

function mapReportReceipt(value: ReportReceiptResponse): ReportReceipt {
  return {
    id: value.id,
    targetType: value.target_type,
    targetID: value.target_id,
    reason: value.reason,
    details: value.details,
    createdAt: value.created_at,
  };
}
