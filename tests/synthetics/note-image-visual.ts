import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { expect, test } from "@playwright/test";
import type { Locator, Page } from "@playwright/test";
const fixture = {
  name: "pao-de-queijo-640x427.jpg",
  dimensions: { naturalWidth: 640, naturalHeight: 427 },
  thresholds: {
    maxChromaDistance: 0.01,
    maxNormalizedDistance: 0.015,
    maxWindowNormalizedDistance: 0.01,
    minEdgeSimilarity: 0.9,
  },
  surface: { radiusPixels: 14 },
} as const;
export const fixtureName = fixture.name;
export const fixturePath = resolve(__dirname, "../fixtures/media", fixtureName);
const fixtureBytes = readFileSync(fixturePath);
const corruptionCases = [
  { kind: "affine" },
  { kind: "color-bars" },
  { kind: "grayscale" },
  { kind: "subtle-color-affine" },
  { kind: "box-blur", radius: 4 },
] as const;
const localizedRegion = {
  regionHeight: 96,
  regionWidth: 72,
  x: 284,
  y: 165,
} as const;
const localizedCorruptionCases = [
  { kind: "localized-color", offset: 20, ...localizedRegion },
  { kind: "localized-blur", radius: 16, ...localizedRegion },
  { kind: "localized-shift", offset: 8, ...localizedRegion },
] as const;
export type FixturePixelMatch = {
  chromaDistance: number;
  edgeSimilarity: number;
  normalizedDistance: number;
  window: WindowMetrics;
};
type WindowMetrics = { maxNormalizedDistance: number };
const windowGrid = 4;
type FixturePixelComparisonInput = {
  fixtureBase64: string;
  screenshotBase64: string;
};
type RenderedFixturePixels = {
  actualPixels: Uint8ClampedArray;
  expectedPixels: Uint8ClampedArray;
  height: number;
  width: number;
};
type CorruptionInput = {
  fixtureBase64: string;
  height: number;
  kind: string;
  offset?: number;
  radius?: number;
  regionHeight?: number;
  regionWidth?: number;
  width: number;
  x?: number;
  y?: number;
};
type LocalizedCorruptionInput = CorruptionInput &
  Required<Pick<CorruptionInput, "regionHeight" | "regionWidth" | "x" | "y">>;
export async function expectFixtureImage(
  page: Page,
  container: Locator,
  expectedURL?: string,
): Promise<{ mediaURL: string }> {
  const surface = await expectImageSurfaceSource(container, expectedURL);
  const screenshot = (await container.screenshot()).toString("base64");
  const pixelMatch = await compareScreenshotToFixture(page, screenshot);
  annotateScore("fixture-pixel-score", pixelMatch);
  expect(isFixtureMatch(pixelMatch)).toBe(true);
  const mediaURL = surface.backgroundURL;
  if (mediaURL === null) {
    throw new Error("note media surface URL missing");
  }
  await expectDecodedFixture(container, mediaURL);
  return { mediaURL };
}
export async function expectFixtureComparatorRejectsCorruptions(
  page: Page,
): Promise<void> {
  for (const corruption of corruptionCases) {
    const render =
      corruption.kind === "box-blur"
        ? renderBoxBlurImage
        : renderCorruptionImage;
    const screenshot = await page.evaluate(render, {
      fixtureBase64: fixtureBytes.toString("base64"),
      height: fixture.dimensions.naturalHeight,
      width: fixture.dimensions.naturalWidth,
      ...corruption,
    });
    const pixelMatch = await compareScreenshotToFixture(page, screenshot);
    annotateScore("fixture-corruption-score", pixelMatch, corruption.kind);
    expect(isFixtureMatch(pixelMatch)).toBe(false);
  }
  for (const corruption of localizedCorruptionCases) {
    const screenshot = await page.evaluate(renderLocalizedCorruptionImage, {
      fixtureBase64: fixtureBytes.toString("base64"),
      height: fixture.dimensions.naturalHeight,
      width: fixture.dimensions.naturalWidth,
      ...corruption,
    });
    const pixelMatch = await compareScreenshotToFixture(page, screenshot);
    annotateScore("fixture-corruption-score", pixelMatch, corruption.kind);
    expect(isFixtureMatch(pixelMatch)).toBe(false);
  }
}
function isFixtureMatch(pixelMatch: FixturePixelMatch): boolean {
  return (
    pixelMatch.normalizedDistance <= fixture.thresholds.maxNormalizedDistance &&
    pixelMatch.chromaDistance <= fixture.thresholds.maxChromaDistance &&
    pixelMatch.edgeSimilarity >= fixture.thresholds.minEdgeSimilarity &&
    pixelMatch.window.maxNormalizedDistance <=
      fixture.thresholds.maxWindowNormalizedDistance
  );
}
function annotateScore(
  type: string,
  pixelMatch: FixturePixelMatch,
  kind?: string,
): void {
  const description = JSON.stringify(pixelMatch);
  test.info().annotations.push({
    description: kind === undefined ? description : `${kind}:${description}`,
    type,
  });
}
async function readImageSurface(container: Locator) {
  return container.evaluate((element) => {
    const candidates = [
      element,
      ...Array.from(element.querySelectorAll<HTMLElement>("*")),
    ];
    const surface = candidates.find(
      (candidate) => getComputedStyle(candidate).backgroundImage !== "none",
    );
    const backgroundImage = surface
      ? getComputedStyle(surface).backgroundImage
      : "none";
    const backgroundMatch = backgroundImage.match(
      /^url\((?:"|')?(.*?)(?:"|')?\)$/,
    );
    const bounds = (surface ?? element).getBoundingClientRect();
    const preload = element.querySelector("img");
    return {
      backgroundURL: backgroundMatch?.[1] ?? null,
      height: bounds.height,
      preloadURL: preload?.currentSrc || preload?.src || null,
      width: bounds.width,
    };
  });
}
async function expectImageSurfaceSource(
  container: Locator,
  expectedURL?: string,
) {
  await expect(container).toBeVisible();
  await expect
    .poll(
      async () => {
        const surface = await readImageSurface(container);
        return (
          surface.width > 0 &&
          surface.height > 0 &&
          surface.backgroundURL !== null &&
          surface.preloadURL !== null
        );
      },
      { timeout: 15_000 },
    )
    .toBe(true);
  const surface = await readImageSurface(container);
  const { backgroundURL, preloadURL } = surface;
  if (backgroundURL === null || preloadURL === null) {
    throw new Error("note media surface source missing");
  }
  expect(surface.width).toBeGreaterThan(0);
  expect(surface.height).toBeGreaterThan(0);
  expect(backgroundURL).toBe(preloadURL);
  expect(backgroundURL).toContain("/v1/media/images/");
  if (expectedURL !== undefined) {
    expect(backgroundURL).toBe(expectedURL);
  }
  return surface;
}
async function compareScreenshotToFixture(
  page: Page,
  screenshotBase64: string,
): Promise<FixturePixelMatch> {
  const renderedPixels = await page.evaluate(renderFixturePixels, {
    fixtureBase64: fixtureBytes.toString("base64"),
    screenshotBase64,
  });
  return scoreFixturePixels(renderedPixels);
}
async function renderFixturePixels({
  fixtureBase64,
  screenshotBase64,
}: FixturePixelComparisonInput): Promise<RenderedFixturePixels> {
  const decode = async (base64: string, type: string): Promise<ImageBitmap> => {
    const bytes = Uint8Array.from(atob(base64), (character) =>
      character.charCodeAt(0),
    );
    return createImageBitmap(new Blob([bytes], { type }));
  };
  const actualBitmap = await decode(screenshotBase64, "image/png");
  const fixtureBitmap = await decode(fixtureBase64, "image/jpeg");
  try {
    const width = actualBitmap.width;
    const height = actualBitmap.height;
    const canvas = new OffscreenCanvas(width, height);
    const context = canvas.getContext("2d", { willReadFrequently: true });
    if (context === null) {
      throw new Error("note media pixel context missing");
    }
    context.drawImage(actualBitmap, 0, 0);
    const actualPixels = context.getImageData(0, 0, width, height).data;
    context.clearRect(0, 0, width, height);
    const scale = Math.max(
      width / fixtureBitmap.width,
      height / fixtureBitmap.height,
    );
    const drawWidth = fixtureBitmap.width * scale;
    const drawHeight = fixtureBitmap.height * scale;
    context.drawImage(
      fixtureBitmap,
      (width - drawWidth) / 2,
      (height - drawHeight) / 2,
      drawWidth,
      drawHeight,
    );
    return {
      actualPixels,
      expectedPixels: context.getImageData(0, 0, width, height).data,
      height,
      width,
    };
  } finally {
    actualBitmap.close();
    fixtureBitmap.close();
  }
}
function scoreFixturePixels(
  rendered: RenderedFixturePixels,
): FixturePixelMatch {
  const { actualPixels, expectedPixels, height, width } = rendered;
  const windowWidth = Math.ceil(width / windowGrid);
  const windowHeight = Math.ceil(height / windowGrid);
  const xStride = Math.max(1, Math.floor(windowWidth / 2));
  const yStride = Math.max(1, Math.floor(windowHeight / 2));
  const columnCount = Math.ceil(width / xStride);
  const windows = Array.from(
    { length: Math.ceil(height / yStride) * columnCount },
    () => [0, 0] as [number, number],
  );
  let count = 0;
  let distanceSum = 0;
  let chromaDistanceSum = 0;
  for (let y = 0; y < height; y += 1) {
    for (let x = 0; x < width; x += 1) {
      if (!isInsideRoundedContent(x, y, width, height)) continue;
      const [rgbDistance, chromaDistance] = pixelDistances(
        actualPixels,
        expectedPixels,
        (y * width + x) * 4,
      );
      count += 1;
      distanceSum += rgbDistance;
      chromaDistanceSum += chromaDistance;
      const firstColumn = Math.max(
        0,
        Math.ceil((x - windowWidth + 1) / xStride),
      );
      const lastColumn = Math.floor(x / xStride);
      const firstRow = Math.max(0, Math.ceil((y - windowHeight + 1) / yStride));
      const lastRow = Math.floor(y / yStride);
      for (let row = firstRow; row <= lastRow; row += 1) {
        for (let column = firstColumn; column <= lastColumn; column += 1) {
          const totals = windows[row * columnCount + column]!;
          totals[0] += 1;
          totals[1] += rgbDistance;
        }
      }
    }
  }
  return {
    chromaDistance: count === 0 ? 1 : chromaDistanceSum / count,
    edgeSimilarity: scoreEdgePreservation(
      actualPixels,
      expectedPixels,
      width,
      height,
    ),
    normalizedDistance: count === 0 ? 1 : distanceSum / count,
    window: scoreWindowMetrics(windows),
  };
}
type WindowTotals = [number, number];
function scoreWindowMetrics(windows: WindowTotals[]): WindowMetrics {
  let maxDistance = 0;
  for (const [count, distanceSum] of windows) {
    if (count === 0) continue;
    maxDistance = Math.max(maxDistance, distanceSum / count);
  }
  return { maxNormalizedDistance: maxDistance };
}
function pixelDistances(
  actualPixels: Uint8ClampedArray,
  expectedPixels: Uint8ClampedArray,
  index: number,
): [number, number] {
  const actualRed = actualPixels[index] ?? 0;
  const actualGreen = actualPixels[index + 1] ?? 0;
  const actualBlue = actualPixels[index + 2] ?? 0;
  const expectedRed = expectedPixels[index] ?? 0;
  const expectedGreen = expectedPixels[index + 1] ?? 0;
  const expectedBlue = expectedPixels[index + 2] ?? 0;
  return [
    (Math.abs(actualRed - expectedRed) +
      Math.abs(actualGreen - expectedGreen) +
      Math.abs(actualBlue - expectedBlue)) /
      (3 * 255),
    (Math.abs(actualRed - actualGreen - expectedRed + expectedGreen) +
      Math.abs(actualGreen - actualBlue - expectedGreen + expectedBlue)) /
      (2 * 255),
  ];
}
function scoreEdgePreservation(
  actualPixels: Uint8ClampedArray,
  expectedPixels: Uint8ClampedArray,
  width: number,
  height: number,
): number {
  const radius = 1;
  const stride = width * 4;
  const luminance = (p: Uint8ClampedArray, i: number): number =>
    (299 * p[i]! + 587 * p[i + 1]! + 114 * p[i + 2]!) / 1000;
  const edge = (p: Uint8ClampedArray, x: number, y: number): number =>
    Math.hypot(
      luminance(p, (y * width + x) * 4 + 4) -
        luminance(p, (y * width + x) * 4 - 4),
      luminance(p, (y * width + x) * 4 + stride) -
        luminance(p, (y * width + x) * 4 - stride),
    );
  const left = radius;
  const top = radius;
  const right = width - radius - 1;
  const bottom = height - radius - 1;
  let expectedEdgeSum = 0;
  let differenceSum = 0;
  for (
    let y = Math.max(radius, top);
    y <= Math.min(height - radius - 1, bottom);
    y += 1
  ) {
    for (
      let x = Math.max(radius, left);
      x <= Math.min(width - radius - 1, right);
      x += 1
    ) {
      if (
        !isInsideRoundedContent(x - radius, y - radius, width, height) ||
        !isInsideRoundedContent(x + radius, y + radius, width, height)
      )
        continue;
      const expectedMagnitude = edge(expectedPixels, x, y);
      expectedEdgeSum += expectedMagnitude;
      differenceSum += Math.abs(edge(actualPixels, x, y) - expectedMagnitude);
    }
  }
  return expectedEdgeSum === 0 ? 1 : 1 - differenceSum / expectedEdgeSum;
}
function isInsideRoundedContent(
  x: number,
  y: number,
  width: number,
  height: number,
): boolean {
  if (x < 0 || y < 0 || x >= width || y >= height) return false;
  const radius = Math.min(
    fixture.surface.radiusPixels,
    Math.floor(Math.min(width, height) / 2),
  );
  const right = width - 1;
  const bottom = height - 1;
  if (
    (x >= radius && x <= right - radius) ||
    (y >= radius && y <= bottom - radius)
  )
    return true;
  const cornerX = x < radius ? radius : right - radius;
  const cornerY = y < radius ? radius : bottom - radius;
  return (x - cornerX) ** 2 + (y - cornerY) ** 2 <= radius ** 2;
}
async function renderBoxBlurImage(input: CorruptionInput): Promise<string> {
  const { fixtureBase64, height, radius, width } = input;
  if (radius === undefined) throw new Error("box blur radius missing");
  const canvas = document.createElement("canvas");
  Object.assign(canvas, { height, width });
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (context === null) throw new Error("counterimage canvas context missing");
  const bytes = Uint8Array.from(atob(fixtureBase64), (character) =>
    character.charCodeAt(0),
  );
  const bitmap = await createImageBitmap(
    new Blob([bytes], { type: "image/jpeg" }),
  );
  try {
    context.filter = `blur(${radius}px)`;
    context.drawImage(bitmap, 0, 0, width, height);
  } finally {
    bitmap.close();
  }
  return canvas.toDataURL("image/png").replace(/^data:image\/png;base64,/, "");
}
async function renderCorruptionImage(input: CorruptionInput): Promise<string> {
  const { fixtureBase64, height, kind, width } = input;
  const canvas = document.createElement("canvas");
  Object.assign(canvas, { height, width });
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (context === null) throw new Error("counterimage canvas context missing");
  if (kind === "color-bars") {
    const colors = [
      "#000000",
      "#ffffff",
      "#ff0000",
      "#00ff00",
      "#0000ff",
      "#ffff00",
      "#ff00ff",
      "#00ffff",
    ];
    const barWidth = Math.ceil(width / colors.length);
    colors.forEach((color, index) => {
      context.fillStyle = color;
      context.fillRect(index * barWidth, 0, barWidth, height);
    });
  } else {
    const bytes = Uint8Array.from(atob(fixtureBase64), (character) =>
      character.charCodeAt(0),
    );
    const bitmap = await createImageBitmap(
      new Blob([bytes], { type: "image/jpeg" }),
    );
    try {
      if (kind === "grayscale") context.filter = "grayscale(1)";
      else if (kind === "affine") context.filter = "brightness(0.8)";
      else {
        context.filter = "saturate(0.78) hue-rotate(7deg)";
        context.setTransform(1.008, 0.002, -0.001, 0.992, 1.4, 0.9);
      }
      context.drawImage(bitmap, 0, 0, width, height);
    } finally {
      bitmap.close();
    }
  }
  return canvas.toDataURL("image/png").replace(/^data:image\/png;base64,/, "");
}
async function renderLocalizedCorruptionImage({
  fixtureBase64,
  height,
  kind,
  offset,
  radius,
  regionHeight,
  regionWidth,
  width,
  x,
  y,
}: LocalizedCorruptionInput): Promise<string> {
  const canvas = document.createElement("canvas");
  Object.assign(canvas, { height, width });
  const context = canvas.getContext("2d", { willReadFrequently: true });
  if (context === null) throw new Error("localized canvas context missing");
  const bytes = Uint8Array.from(atob(fixtureBase64), (character) =>
    character.charCodeAt(0),
  );
  const bitmap = await createImageBitmap(
    new Blob([bytes], { type: "image/jpeg" }),
  );
  try {
    context.drawImage(bitmap, 0, 0, width, height);
    if (kind === "localized-blur") {
      context.save();
      context.beginPath();
      context.rect(x, y, regionWidth, regionHeight);
      context.clip();
      context.filter = `blur(${radius!}px)`;
      context.drawImage(bitmap, 0, 0, width, height);
      context.restore();
    } else {
      const pixels = context.getImageData(0, 0, width, height);
      const source = new Uint8ClampedArray(pixels.data);
      const amount = offset ?? 0;
      for (let row = y; row < y + regionHeight; row += 1) {
        for (let column = x; column < x + regionWidth; column += 1) {
          const index = (row * width + column) * 4;
          const sourceIndex =
            kind === "localized-shift"
              ? (row * width + Math.max(0, column - amount)) * 4
              : index;
          for (let channel = 0; channel < 3; channel += 1) {
            pixels.data[index + channel] =
              source[sourceIndex + channel]! +
              (kind === "localized-color" ? amount : 0);
          }
        }
      }
      context.putImageData(pixels, 0, 0);
    }
  } finally {
    bitmap.close();
  }
  return canvas.toDataURL("image/png").replace(/^data:image\/png;base64,/, "");
}
async function expectDecodedFixture(
  container: Locator,
  expectedURL: string,
): Promise<void> {
  const nativeImage = container.locator("img");
  await expect(nativeImage).toHaveCount(1);
  await expect
    .poll(
      () =>
        nativeImage.evaluate((element) => {
          if (!(element instanceof HTMLImageElement)) {
            throw new Error("note media did not render an image element");
          }
          return {
            complete: element.complete,
            naturalHeight: element.naturalHeight,
            naturalWidth: element.naturalWidth,
            sourceURL: element.currentSrc || element.src,
          };
        }),
      { timeout: 15_000 },
    )
    .toEqual({
      complete: true,
      ...fixture.dimensions,
      sourceURL: expectedURL,
    });
}
