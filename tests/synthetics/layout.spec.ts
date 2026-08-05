import { randomUUID } from 'node:crypto';

import { expect, test } from '@playwright/test';
import type { APIRequestContext, Page } from '@playwright/test';

import {
  expectEqualInsets,
  expectEvenlySpacedCenters,
  expectNoHorizontalClipping,
  expectSharedRow,
  expectWithinViewport,
  widthOf,
} from './geometry';
import {
  createAuthUser,
  createNote,
  loginUser,
  syntheticPassword,
} from './support';

// The design is drawn on a 390px frame and the app caps its content at
// maxAppWidth, so these numbers are the contract, not a snapshot of whatever
// the layout currently produces.
const gutter = 16;
const masonryGap = 12;
const maxAppWidth = 430;
const tabSlotCount = 4;

type LayoutFixture = { displayName: string; titles: string[] };

async function signInWithNotes(
  page: Page,
  request: APIRequestContext,
): Promise<LayoutFixture> {
  const suffix = randomUUID().replaceAll('-', '').slice(0, 8);
  const username = `layout-${suffix}`;
  const displayName = `Layout ${suffix}`;
  const session = await createAuthUser(request, {
    display_name: displayName,
    password: syntheticPassword,
    username,
  });
  const titles = [1, 2, 3, 4].map(
    (index) => `Nota de layout ${suffix} ${index}`,
  );
  for (const [index, title] of titles.entries()) {
    await createNote(request, session.token, {
      body: `Corpo da nota de layout ${suffix} numero ${index + 1}.`,
      category_slug: 'food',
      client_request_id: randomUUID(),
      title,
    });
  }
  await loginUser(page, username, '/');
  await expect(page.getByTestId('masonry-grid')).toBeVisible({
    timeout: 30_000,
  });
  return { displayName, titles };
}

test.describe('layout geometry', () => {
  test('the bottom nav spaces its four slots evenly', async ({
    page,
    request,
  }) => {
    await signInWithNotes(page, request);

    const bar = page.getByTestId('tab-bar');
    await expectWithinViewport(page, bar, 'tab bar');
    await expectNoHorizontalClipping(bar, 'tab bar');

    // All four slots carry equal weight, so their centers form one evenly
    // spaced series. Asserting it over the three tab items alone would be
    // wrong: the FAB sits between the second and third tab, so that middle
    // gap is legitimately double.
    const tabNames = ['index', 'search', 'profile'];
    const slots = [
      ...tabNames.map((name) => page.getByTestId(`tab-item-${name}`)),
      page.getByTestId('tab-fab-slot'),
    ];
    await expectEvenlySpacedCenters(slots, 'tab bar slots');

    // Equal centers alone would also hold if the FAB slot were resized and
    // the tabs shifted to match, so pin its weight to a tab item's too.
    const slotWidth = await widthOf(
      page.getByTestId('tab-item-index'),
      'a tab item',
    );
    const fabSlotWidth = await widthOf(
      page.getByTestId('tab-fab-slot'),
      'the FAB slot',
    );
    expect(
      Math.abs(fabSlotWidth - slotWidth),
      'the FAB slot should carry a tab item’s weight, not a fixed width',
    ).toBeLessThanOrEqual(0.5);
    expect(slots.length).toBe(tabSlotCount);
  });

  test('the grid, header, and rail share one gutter and stay on screen', async ({
    page,
    request,
  }) => {
    await signInWithNotes(page, request);

    const grid = page.getByTestId('masonry-grid');
    const headerRow = page.getByTestId('app-header-row');
    await expectWithinViewport(page, grid, 'masonry grid');
    await expectWithinViewport(page, headerRow, 'header row');
    await expectEqualInsets(
      page,
      [grid, headerRow],
      'the grid and the header row',
    );

    // The rail scrolls horizontally on purpose, so overflow is legitimate
    // there; its leading inset still has to match the surfaces above it.
    const rail = page.getByTestId('category-rail');
    await expectNoHorizontalClipping(rail, 'category rail');
    const firstChip = rail.getByRole('button').first();
    const gridBox = await grid.boundingBox();
    const chipBox = await firstChip.boundingBox();
    if (gridBox === null || chipBox === null) {
      throw new Error('expected the grid and the first chip to have boxes');
    }
    expect(
      Math.abs(chipBox.x - (gridBox.x + gutter)),
      'the first category chip should start at the shared gutter',
    ).toBeLessThanOrEqual(0.5);
  });

  test('the grid balances two columns and never exceeds maxAppWidth', async ({
    page,
    request,
  }) => {
    await signInWithNotes(page, request);

    const grid = page.getByTestId('masonry-grid');
    const gridWidth = await widthOf(grid, 'masonry grid');
    expect(
      gridWidth,
      'the grid should stop growing at maxAppWidth',
    ).toBeLessThanOrEqual(maxAppWidth + 0.5);

    const columns = page.getByTestId(/^masonry-column-\d+$/);
    await expect(columns).toHaveCount(2);
    const first = await widthOf(columns.nth(0), 'the first column');
    const second = await widthOf(columns.nth(1), 'the second column');
    expect(
      Math.abs(first - second),
      'both columns should take the same width',
    ).toBeLessThanOrEqual(0.5);
    expect(
      Math.abs(first + second + masonryGap + 2 * gutter - gridWidth),
      'the columns, their gap, and both gutters should fill the grid exactly',
    ).toBeLessThanOrEqual(1);

    const card = page.getByTestId('note-card').first();
    await expectWithinViewport(page, card, 'a note card');
    await expectNoHorizontalClipping(card, 'a note card');
  });

  test('a note card keeps its useful count beside its icon', async ({
    page,
    request,
  }) => {
    const fixture = await signInWithNotes(page, request);

    // The primitive under this row applies the caller's style to the element
    // holding the children; when it did not, every one of these rows stacked
    // while the style object still read as a row.
    //
    // The feed carries notes from every other synthetic, so pin this to one
    // note this test created rather than whichever card happens to sort first.
    const card = page
      .getByTestId('note-card')
      .filter({ hasText: fixture.titles[0] ?? '' });
    const author = card.getByRole('button', {
      exact: true,
      name: `Abrir perfil do autor: ${fixture.displayName}`,
    });
    const useful = card.getByRole('button', {
      exact: true,
      name: 'Marcar como útil',
    });
    await expectSharedRow([author, useful], 'the card footer row');
  });

  test('the compose photo cell fits the screen', async ({ page, request }) => {
    await signInWithNotes(page, request);
    await page.getByLabel('Escrever um achado', { exact: true }).click();
    await expect(page.getByTestId('compose-add-image')).toBeVisible({
      timeout: 30_000,
    });

    const addImage = page.getByTestId('compose-add-image');
    await expectWithinViewport(page, addImage, 'the add-image cell');
    await expectNoHorizontalClipping(addImage, 'the add-image cell');
  });

  test('the account recovery screens fit the viewport', async ({ page }) => {
    for (const path of ['/recover-password', '/new-password?token=dummy', '/verify-email?token=dummy']) {
      await page.goto(path);
      // _layout.tsx anchors every deep link on the (tabs) group so the back
      // affordance always has somewhere to go. That anchor screen stays
      // mounted (aria-hidden, display:none) beneath the pushed screen, so it
      // still carries its own app-header-row; narrow to the one actually
      // on screen rather than the raw (2-element) testID match.
      const header = page.getByTestId('app-header-row').and(page.locator(':visible'));
      await expect(header).toBeVisible({ timeout: 10_000 });
      await expectWithinViewport(page, header, `the ${path} screen header`);
      await expectNoHorizontalClipping(header, `the ${path} screen header`);
    }
  });
});
