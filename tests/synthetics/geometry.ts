import { expect } from '@playwright/test';
import type { Locator, Page } from '@playwright/test';

// Numeric layout invariants, asserted against real boxes in a real browser.
//
// The unit suite runs in a node environment with no layout engine: it can see
// that an element received `{ flexDirection: 'row' }`, never that it rendered
// as a row. These assertions cover that gap without committing screenshots.
// They encode intent — "these centers are evenly spaced", "these insets
// match" — rather than appearance, so an intentional redesign that keeps the
// intent keeps them green, and each failure reads as numbers in a CI log.

// Sub-pixel: rn-web resolves flex to fractional widths, so exact equality
// would fail on layouts that are correct.
const tolerance = 0.5;

type Box = { height: number; width: number; x: number; y: number };

async function boxOf(locator: Locator, label: string): Promise<Box> {
  await expect(locator, `${label} should be visible`).toBeVisible();
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error(`${label} has no layout box`);
  }
  return box;
}

async function boxesOf(locators: Locator[], label: string): Promise<Box[]> {
  const boxes: Box[] = [];
  for (const [index, locator] of locators.entries()) {
    boxes.push(await boxOf(locator, `${label}[${index}]`));
  }
  return boxes;
}

export async function expectWithinViewport(
  page: Page,
  locator: Locator,
  label: string,
): Promise<void> {
  const viewport = page.viewportSize();
  if (viewport === null) {
    throw new Error('viewport size unavailable');
  }
  const box = await boxOf(locator, label);
  expect(box.x, `${label} starts left of the viewport`).toBeGreaterThanOrEqual(
    -tolerance,
  );
  expect(
    box.x + box.width,
    `${label} extends past the right edge of the viewport`,
  ).toBeLessThanOrEqual(viewport.width + tolerance);
}

// A container may overflow only if it can be scrolled to reach the overflow.
// Anything else is content the user cannot get to.
export async function expectNoHorizontalClipping(
  locator: Locator,
  label: string,
): Promise<void> {
  await expect(locator, `${label} should be visible`).toBeVisible();
  const overflow = await locator.evaluate((element) => ({
    clientWidth: element.clientWidth,
    overflowX: getComputedStyle(element).overflowX,
    scrollWidth: element.scrollWidth,
  }));
  if (overflow.overflowX === 'auto' || overflow.overflowX === 'scroll') {
    return;
  }
  expect(
    overflow.scrollWidth,
    `${label} hides ${overflow.scrollWidth - overflow.clientWidth}px of content it cannot scroll to`,
  ).toBeLessThanOrEqual(overflow.clientWidth + tolerance);
}

export async function expectEvenlySpacedCenters(
  locators: Locator[],
  label: string,
): Promise<void> {
  expect(
    locators.length,
    `${label} needs at least three elements to have comparable gaps`,
  ).toBeGreaterThanOrEqual(3);
  const centers = (await boxesOf(locators, label))
    .map((box) => box.x + box.width / 2)
    .sort((left, right) => left - right);
  const gaps = centers
    .slice(1)
    .map((center, index) => center - (centers[index] ?? 0));
  const spread = Math.max(...gaps) - Math.min(...gaps);
  expect(
    spread,
    `${label} centers are unevenly spaced: gaps ${gaps.map((gap) => gap.toFixed(2)).join(', ')}`,
  ).toBeLessThanOrEqual(tolerance);
}

// Catches the class of bug where a row of siblings renders as a column: the
// style object still says `row`, but the boxes no longer share a centerline.
export async function expectSharedRow(
  locators: Locator[],
  label: string,
): Promise<void> {
  const boxes = await boxesOf(locators, label);
  const centers = boxes.map((box) => box.y + box.height / 2);
  const spread = Math.max(...centers) - Math.min(...centers);
  expect(
    spread,
    `${label} does not share a row: vertical centers ${centers.map((center) => center.toFixed(2)).join(', ')}`,
  ).toBeLessThanOrEqual(Math.min(...boxes.map((box) => box.height)) / 2);
}

export async function expectEqualInsets(
  page: Page,
  locators: Locator[],
  label: string,
): Promise<void> {
  const viewport = page.viewportSize();
  if (viewport === null) {
    throw new Error('viewport size unavailable');
  }
  const boxes = await boxesOf(locators, label);
  const leftInsets = boxes.map((box) => box.x);
  const rightInsets = boxes.map((box) => viewport.width - (box.x + box.width));
  expect(
    Math.max(...leftInsets) - Math.min(...leftInsets),
    `${label} left insets disagree: ${leftInsets.map((inset) => inset.toFixed(2)).join(', ')}`,
  ).toBeLessThanOrEqual(tolerance);
  expect(
    Math.max(...rightInsets) - Math.min(...rightInsets),
    `${label} right insets disagree: ${rightInsets.map((inset) => inset.toFixed(2)).join(', ')}`,
  ).toBeLessThanOrEqual(tolerance);
}

export async function expectCentered(
  outer: Locator,
  inner: Locator,
  label: string,
): Promise<void> {
  const outerBox = await boxOf(outer, `${label} container`);
  const innerBox = await boxOf(inner, `${label} content`);
  const outerCenter = outerBox.x + outerBox.width / 2;
  const innerCenter = innerBox.x + innerBox.width / 2;
  expect(
    Math.abs(outerCenter - innerCenter),
    `${label} is not centered in its container`,
  ).toBeLessThanOrEqual(tolerance);
}

export async function widthOf(locator: Locator, label: string): Promise<number> {
  return (await boxOf(locator, label)).width;
}
