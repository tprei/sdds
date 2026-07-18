import { expect, it } from 'vitest';
import { imageUploadReceiptSchema } from './schema';

const validReceipt = {
  byte_size: 481234,
  content_type: 'image/jpeg' as const,
  expires_at: 1782993600000,
  height: 800,
  image_upload_id: '018ff5b8-0000-7000-8000-000000000001',
  width: 1200,
};
it('accepts generated receipts and strips extra keys', () => {
  expect(
    imageUploadReceiptSchema.parse({ ...validReceipt, request_id: 'extra' }),
  ).toEqual(validReceipt);
});
it('accepts uppercase UUIDs without transforming them', () => {
  const imageUploadID = validReceipt.image_upload_id.toUpperCase();
  expect(
    imageUploadReceiptSchema.parse({
      ...validReceipt,
      image_upload_id: imageUploadID,
    }).image_upload_id,
  ).toBe(imageUploadID);
});
it.each([
  ['missing required field', { ...validReceipt, width: undefined }],
  ['invalid UUID', { ...validReceipt, image_upload_id: 'not-a-uuid' }],
  ['invalid content type', { ...validReceipt, content_type: 'image/gif' }],
  ['fractional byte size', { ...validReceipt, byte_size: 1.5 }],
  ['nonpositive width', { ...validReceipt, width: 0 }],
  ['unsafe height', { ...validReceipt, height: Number.MAX_SAFE_INTEGER + 1 }],
])('rejects %s', (_name, value) => {
  expect(imageUploadReceiptSchema.safeParse(value).success).toBe(false);
});
