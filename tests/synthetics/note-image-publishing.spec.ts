import { randomUUID } from 'node:crypto';

import { expect, test } from '@playwright/test';
import type { Locator, Page } from '@playwright/test';

import {
  expectFixtureComparatorRejectsCorruptions,
  expectFixtureImage,
  fixtureName,
  fixturePath,
} from './note-image-visual';

const syntheticPassword = 'secret-password';
test.use({ viewport: { height: 720, width: 1280 } });

type SyntheticUser = { displayName: string; username: string };
type SyntheticScenario = {
  author: SyntheticUser;
  note: { body: string; title: string };
  reader: SyntheticUser;
};

test('publishes an image through the UI for another signed-in user', async ({
  page,
}) => {
  const scenario = createScenario();
  await page.goto('/');
  await expectExplore(page);
  await authorSignupAndPublish(page, scenario);
  await logoutAndSignupReader(page, scenario);
  const readerImage = await discoverPublishedImage(page, scenario);
  const readerProof = await expectFixtureImage(page, readerImage);
  await expectFixtureComparatorRejectsCorruptions(page);
  await viewPublishedNoteDetail(page, scenario, readerProof.mediaURL);
  await logoutReader(page);
});

function createScenario(): SyntheticScenario {
  const runSuffix = randomUUID().replaceAll('-', '').slice(0, 8);
  return {
    author: {
      displayName: `Ana da Padaria ${runSuffix}`,
      username: `img-author-${runSuffix}`,
    },
    note: {
      body: `Casquinha crocante, miolo macio e café passado na hora (${runSuffix}).`,
      title: `Pão de queijo dourado ${runSuffix}`,
    },
    reader: {
      displayName: `Bia Leitora ${runSuffix}`,
      username: `img-reader-${runSuffix}`,
    },
  };
}
async function expectExplore(page: Page): Promise<void> {
  await expectVisible(
    page.getByRole('heading', { exact: true, name: 'Explorar' }),
    30_000,
  );
}
async function authorSignupAndPublish(
  page: Page,
  scenario: SyntheticScenario,
): Promise<void> {
  await clickTab(page, 'Escrever');
  await expectVisible(
    page.getByText('Entre para escrever', { exact: true }),
    30_000,
  );
  await expectVisible(
    page.getByRole('button', { exact: true, name: 'Criar conta' }),
  );
  await page.getByRole('button', { exact: true, name: 'Criar conta' }).click();
  await expect(page).toHaveURL(/\/signup(?:[?#]|$)/, { timeout: 30_000 });
  await expectVisible(page.getByLabel('Seu nome'));
  await signUp(page, scenario.author);
  await expect(page).toHaveURL(/\/compose(?:[?#]|$)/, { timeout: 30_000 });
  await expectVisible(
    page.getByText('Conta uma dica', { exact: true }),
    30_000,
  );
  await page.getByLabel('Título da nota').fill(scenario.note.title);
  await page.getByLabel('Texto da nota').fill(scenario.note.body);
  await expectVisible(
    page.getByRole('button', { exact: true, name: 'Comida' }),
    30_000,
  );
  await uploadFixture(page);
  await page.getByRole('button', { exact: true, name: 'Comida' }).click();
  await page.getByRole('button', { exact: true, name: 'São Paulo' }).click();
  await expect(
    page.getByRole('button', { exact: true, name: 'Publicar' }),
  ).toBeEnabled();
  await page.getByRole('button', { exact: true, name: 'Publicar' }).click();
  await expectExplore(page);
  const authorCard = noteCard(page, scenario);
  await expectVisible(authorCard, 30_000);
  await expect(authorCard).toContainText(scenario.note.body);
}
async function uploadFixture(page: Page): Promise<void> {
  const fileChooserPromise = page.waitForEvent('filechooser');
  await page
    .getByRole('button', { exact: true, name: 'Adicionar imagem' })
    .click();
  const fileChooser = await fileChooserPromise;
  await fileChooser.setFiles(fixturePath);
  await expectVisible(page.getByText(fixtureName, { exact: true }), 30_000);
  await expectVisible(
    page.getByRole('button', { exact: true, name: 'Trocar imagem' }),
  );
  await expectVisible(
    page.getByRole('button', { exact: true, name: 'Remover imagem' }),
  );
}
async function logoutAndSignupReader(
  page: Page,
  scenario: SyntheticScenario,
): Promise<void> {
  await clickTab(page, 'Perfil');
  await expectVisible(
    page.getByRole('heading', {
      exact: true,
      name: scenario.author.displayName,
    }),
    30_000,
  );
  await expectVisible(page.getByRole('button', { exact: true, name: 'Sair' }));
  await page.getByRole('button', { exact: true, name: 'Sair' }).click();
  await expectVisible(
    page.getByText('Entre para publicar', { exact: true }),
    30_000,
  );
  await page.getByRole('button', { exact: true, name: 'Criar conta' }).click();
  await expect(page).toHaveURL(/\/signup(?:[?#]|$)/, { timeout: 30_000 });
  await expectVisible(page.getByLabel('Seu nome'));
  await signUp(page, scenario.reader);
  await expect(page).toHaveURL(/\/profile(?:[?#]|$)/, { timeout: 30_000 });
  await expectVisible(
    page.getByRole('heading', {
      exact: true,
      name: scenario.reader.displayName,
    }),
    30_000,
  );
  await expectVisible(page.getByRole('button', { exact: true, name: 'Sair' }));
}
async function discoverPublishedImage(
  page: Page,
  scenario: SyntheticScenario,
): Promise<Locator> {
  await clickTab(page, 'Início');
  await expectExplore(page);
  const card = noteCard(page, scenario);
  await expectVisible(card, 30_000);
  await expect(card).toContainText(scenario.note.body);
  await expectVisible(
    page.getByRole('button', {
      exact: true,
      name: `Abrir perfil do autor: ${scenario.author.displayName}`,
    }),
  );
  const image = noteImage(card, page, scenario);
  await expect(image).toHaveCount(1);
  return image;
}
function noteCard(page: Page, scenario: SyntheticScenario): Locator {
  return page.getByRole('button', {
    exact: true,
    name: `Abrir nota com imagem: ${scenario.note.title}`,
  });
}
function noteImage(
  container: Page | Locator,
  scope: Page | Locator,
  scenario: SyntheticScenario,
): Locator {
  return container
    .getByRole('img', {
      exact: true,
      name: `Imagem da nota: ${scenario.note.title}`,
    })
    .filter({ has: scope.locator('img') });
}
async function viewPublishedNoteDetail(
  page: Page,
  scenario: SyntheticScenario,
  mediaURL: string,
): Promise<void> {
  await noteCard(page, scenario).click();
  await expect(page).toHaveURL(/\/notes\/[^/?#]+$/, { timeout: 30_000 });
  await expectVisible(
    page.getByRole('heading', { exact: true, name: scenario.note.title }),
    30_000,
  );
  await expectVisible(
    page.getByLabel(`Texto da nota: ${scenario.note.body}`, { exact: true }),
  );
  await expectVisible(
    page.getByRole('button', {
      exact: true,
      name: `Abrir perfil do autor: ${scenario.author.displayName}`,
    }),
  );
  await expectVisible(
    page.getByLabel('Categoria da nota: Comida', { exact: true }),
  );
  await expectVisible(
    page.getByLabel('Lugar da nota: São Paulo', { exact: true }),
  );
  const image = noteImage(page, page, scenario);
  await expect(image).toHaveCount(1);
  const detailProof = await expectFixtureImage(page, image, mediaURL);
  expect(detailProof.mediaURL).toBe(mediaURL);
}
async function logoutReader(page: Page): Promise<void> {
  await clickTab(page, 'Perfil');
  await expectVisible(
    page.getByRole('button', { exact: true, name: 'Sair' }),
    30_000,
  );
  await page.getByRole('button', { exact: true, name: 'Sair' }).click();
  await expectVisible(
    page.getByRole('button', { exact: true, name: 'Criar conta' }),
    30_000,
  );
}
async function signUp(page: Page, user: SyntheticUser): Promise<void> {
  await page.getByLabel('Seu nome').fill(user.displayName);
  await page.getByLabel('Nome de usuário').fill(user.username);
  await page.getByLabel('Senha').fill(syntheticPassword);
  await page.getByRole('button', { exact: true, name: 'Criar conta' }).click();
}
async function clickTab(page: Page, name: string): Promise<void> {
  const tab = page.getByRole('tab', { name: new RegExp(`${name}$`) });
  await expectVisible(tab);
  await tab.click();
}

async function expectVisible(
  locator: Locator,
  timeout?: number,
): Promise<void> {
  if (timeout === undefined) {
    await expect(locator).toBeVisible();
  } else {
    await expect(locator).toBeVisible({ timeout });
  }
}
