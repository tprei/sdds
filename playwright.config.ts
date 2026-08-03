import { requiredEnv } from './tests/synthetics/env';
import { defineConfig, devices } from '@playwright/test';


const apiBaseURL = requiredEnv(
  'SDDS_SYNTHETICS_API_BASE_URL',
  process.env.SDDS_SYNTHETICS_API_BASE_URL,
);
const webPort = requiredEnv('SDDS_SYNTHETICS_WEB_PORT', process.env.SDDS_SYNTHETICS_WEB_PORT);
const webBaseURL =
  process.env.PLAYWRIGHT_BASE_URL ?? `http://localhost:${webPort}`;

export default defineConfig({
  outputDir: 'test-results',
  projects: [
    {
      name: 'chromium',
      testIgnore: 'layout.spec.ts',
      use: { ...devices['Desktop Chrome'], ...(process.env.CI ? { channel: 'chrome' } : {}) },
    },
    {
      name: 'layout-390x844',
      testMatch: 'layout.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 390, height: 844 },
        deviceScaleFactor: 1,
        ...(process.env.CI ? { channel: 'chrome' } : {}),
      },
    },
    {
      name: 'layout-430x932',
      testMatch: 'layout.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 430, height: 932 },
        deviceScaleFactor: 1,
        ...(process.env.CI ? { channel: 'chrome' } : {}),
      },
    },
    {
      name: 'layout-820x1180',
      testMatch: 'layout.spec.ts',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 820, height: 1180 },
        deviceScaleFactor: 1,
        ...(process.env.CI ? { channel: 'chrome' } : {}),
      },
    },
  ],
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: 'playwright-report' }],
  ],
  testDir: 'tests/synthetics',
  use: {
    baseURL: webBaseURL,
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'pnpm --filter @sdds/mobile exec expo start --web --localhost',
    env: {
      EXPO_NO_TELEMETRY: '1',
      EXPO_PUBLIC_SDDS_API_BASE_URL: apiBaseURL,
      RCT_METRO_PORT: webPort,
    },
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    url: webBaseURL,
  },
  workers: process.env.CI ? 1 : undefined,
});
